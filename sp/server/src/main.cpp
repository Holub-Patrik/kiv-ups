#include <array>
#include <chrono>
#include <cstring>
#include <exception>
#include <format>
#include <iostream>
#include <optional>
#include <string>
#include <thread>
#include <vector>

extern "C" {
#include <asm-generic/socket.h>
#include <netinet/in.h>
#include <sys/socket.h>
#include <sys/types.h>
#include <sys/un.h>
#include <unistd.h>
}

constexpr long max_port = 65535;
constexpr int player_count = 4;

enum class SocketExceptionType { SOCK, SETSOCKOPT, BIND, LISTEN, ACCEPT };

class SocketException : public std::exception {
private:
  SocketExceptionType t;

public:
  SocketException(SocketExceptionType type) : t(type) {}

  const char* what() const noexcept override {
    switch (t) {
    case SocketExceptionType::SOCK:
      return "Couldn't aquire socket fd";
      break;
    case SocketExceptionType::SETSOCKOPT:
      return "Couldn't set socket options";
      break;
    case SocketExceptionType::BIND:
      return "Failed to bind socket";
      break;
    case SocketExceptionType::LISTEN:
      return "Can't listen on the given port";
      break;
    case SocketExceptionType::ACCEPT:
      return "Failed to accept a remote connection to a socket";
      break;
    }
  }

  const SocketExceptionType get_type() const noexcept { return t; }
};

enum class PlayerAction {
  RAISE,
  SKIP,
  PASS,
  FOLD,
  INIT, // init state
};

enum class RoundState {
  PASS,
  BET,
};

class PlayerState final {
private:
  PlayerAction _taken_action;

public:
};

class Lobby final {
private:
  std::array<PlayerState, 4> _players;

public:
};

class ServerSocket final {
private:
  int sock_fd;
  struct sockaddr_in addr;

public:
  ServerSocket() = delete;
  ~ServerSocket() { close(sock_fd); }

  ServerSocket(const ServerSocket& other) = delete;
  ServerSocket& operator=(const ServerSocket& other) = delete;

  ServerSocket(ServerSocket&& other) noexcept
      : sock_fd(other.sock_fd), addr(other.addr) {
    other.sock_fd = -1;
    memset(&other.sock_fd, 0, sizeof(typeof(other.sock_fd)));
  }

  ServerSocket& operator=(ServerSocket&& other) noexcept {
    sock_fd = other.sock_fd;
    addr = other.addr;

    other.sock_fd = -1;
    memset(&other.sock_fd, 0, sizeof(typeof(other.sock_fd)));

    return *this;
  }

  explicit ServerSocket(std::uint16_t port) {
    sock_fd = socket(AF_INET, SOCK_STREAM, 0);
    if (sock_fd <= 0) {
      throw SocketException(SocketExceptionType::SOCK);
    }

    memset(&addr, 0, sizeof(struct sockaddr_in));
    addr.sin_family = AF_INET;
    addr.sin_port = htons(port);
    addr.sin_addr.s_addr = INADDR_ANY;

    int param = 1;
    const auto setsockopt_ret = setsockopt(sock_fd, SOL_SOCKET, SO_REUSEADDR,
                                           (const char*)&param, sizeof(int));
    if (setsockopt_ret == -1) {
      throw SocketException(SocketExceptionType::SETSOCKOPT);
    }

    const auto bind_ret =
        bind(sock_fd, (struct sockaddr*)&addr, sizeof(struct sockaddr_in));
    if (bind_ret != 0) {
      throw SocketException(SocketExceptionType::BIND);
    }

    const auto listen_ret = listen(sock_fd, 4);
    if (listen_ret != 0) {
      throw SocketException(SocketExceptionType::LISTEN);
    }
  }

  int get_fd() const noexcept { return sock_fd; }
};

class RemoteSock final {
private:
  int sock_fd;
  struct sockaddr_in addr;
  socklen_t addr_len;

public:
  ~RemoteSock() {
    close(sock_fd);
    std::cout << "Closed socket: " << sock_fd << std::endl;
  }

  RemoteSock(const ServerSocket& server) {
    const auto accepted_sock =
        accept(server.get_fd(), (struct sockaddr*)&addr, &addr_len);
    if (accepted_sock <= 0) {
      throw SocketException{SocketExceptionType::ACCEPT};
    }

    sock_fd = accepted_sock;
  }

  RemoteSock(const RemoteSock& sock) = delete;
  RemoteSock& operator=(const RemoteSock& sock) = delete;

  RemoteSock(RemoteSock&& sock)
      : sock_fd(sock.sock_fd), addr(sock.addr), addr_len(sock.addr_len) {
    sock.sock_fd = -1;
    sock.addr_len = -1;
    memset(&sock.addr, 0, sizeof(typeof(sock.addr)));
  }

  RemoteSock& operator=(RemoteSock&& sock) {
    sock_fd = sock.sock_fd;
    addr_len = sock.addr_len;
    addr = sock.addr;

    sock.sock_fd = -1;
    sock.addr_len = -1;
    memset(&sock.addr, 0, sizeof(typeof(sock.addr)));

    return *this;
  }

  int get_fd() const noexcept { return sock_fd; }
};

auto player_thread(const ServerSocket& server_sock,
                   std::vector<std::string>& mem, std::mutex& mem_mutex) -> int;

class Server final {
private:
  ServerSocket sock;
  std::array<std::thread, player_count> player_threads;
  std::array<std::vector<std::string>, player_count> player_mem;
  std::array<std::mutex, player_count> player_mem_mutex;

public:
  virtual ~Server() = default;

  Server(const Server& other) = delete;
  Server(Server&& other) = delete;

  Server& operator=(const Server& other) = delete;
  Server& operator=(Server&& other) = delete;

  Server(ServerSocket&& sock) : sock(std::move(sock)) {
    player_threads = {std::thread{[]() -> void { return; }},
                      std::thread{[]() -> void { return; }},
                      std::thread{[]() -> void { return; }},
                      std::thread{[]() -> void { return; }}};
    for (auto& t : player_threads) {
      t.join();
    }
  }

  void run() {
    // manually unrolled the loop
    // with a for loop, it was for some reason causing issues
    player_threads.at(0) = std::thread([&]() -> int {
      return player_thread(sock, player_mem.at(0), player_mem_mutex.at(0));
    });
    player_threads.at(1) = std::thread([&]() -> int {
      return player_thread(sock, player_mem.at(1), player_mem_mutex.at(1));
    });
    player_threads.at(2) = std::thread([&]() -> int {
      return player_thread(sock, player_mem.at(2), player_mem_mutex.at(2));
    });
    player_threads.at(3) = std::thread([&]() -> int {
      return player_thread(sock, player_mem.at(3), player_mem_mutex.at(3));
    });

    bool joined = false;
    while (true) {
      for (auto& t : player_threads) {
        if (t.joinable()) {
          t.join();
          joined = true;
        }

        if (!joined) {
          std::this_thread::sleep_for(std::chrono::milliseconds(20));
        }

        joined = false;
      }
    }

    return;
  }
};

struct usr_args {
  std::uint16_t port;
  explicit usr_args(std::uint16_t p) : port(p) {}
};

auto player_thread(const ServerSocket& server_sock,
                   std::vector<std::string>& mem, std::mutex& mem_mutex)
    -> int {
  RemoteSock player{server_sock};
  std::cout << "Accepted connection from player" << std::endl;
  std::array<char, 256> buf;
  memset(buf.data(), 0, buf.size());

  const auto bytes_read = recv(player.get_fd(), buf.data(), buf.size(), 0);
  std::cout << std::format("Thread: Read {} bytes: {}", bytes_read, buf.data())
            << std::endl;
  std::string msg_to_client{"S0015HelloFromServer"};
  const auto bytes_sent =
      send(player.get_fd(), msg_to_client.data(), msg_to_client.size(), 0);

  return 0;
}

auto parse_args(std::vector<std::string> args) -> std::optional<usr_args> {
  try {
    unsigned long usr_part = std::stoul(args[0]);
    if (usr_part > max_port) {
      return std::nullopt;
    } else {
      std::uint16_t port = static_cast<std::uint16_t>(usr_part);
      struct usr_args ret{port};
      return ret;
    }
  } catch (std::exception e) {
    return std::nullopt;
  }
}

auto main(int argc, char* argv[]) -> int {
  if (argc <= 1) {
    std::cout << "Not enough arguments." << std::endl;
  }

  std::vector<std::string> args{};
  for (int i = 1; i < argc; i++) {
    args.emplace_back(std::string{argv[i]});
  }

  const auto maybe_parsed_args = parse_args(args);
  if (!maybe_parsed_args) {
    std::cout << "Error during parsing arguments." << std::endl;
    return -1;
  }

  const auto parsed_args = maybe_parsed_args.value();

  ServerSocket sock{parsed_args.port};
  Server server{std::move(sock)};
  server.run();

  return 0;
}
