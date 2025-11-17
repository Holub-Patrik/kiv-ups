#pragma once

#include "CircularBufferQueue.hpp"
#include "MessageSerde.hpp"
#include "SockWrapper.hpp"

#include <chrono>
#include <sstream>
#include <sys/socket.h>
#include <thread>

struct MsgStruct {
  std::string code;
  std::optional<std::string> payload = std::nullopt;

  std::string to_string() const {
    std::stringstream ss;
    ss << "PKR";

    if (payload) {
      ss << "P";
    } else {
      ss << "N";
    }

    ss << code;
    if (payload) {
      const auto& contents = payload.value();
      const auto& len_str = std::format("{:04d}", contents.size());
      ss << len_str << contents;
    }

    ss << "\n";
    return ss.str();
  }
};

struct Room {
  std::string id;
  std::string name;
  int current_players;
  int max_players;

  std::string to_payload_string() const {
    std::stringstream ss;
    ss << std::format("{:04}", std::stoi(id));
    ss << std::format("{:04}", name.length());
    ss << name;
    ss << std::format("{:02}", current_players);
    ss << std::format("{:02}", max_players);
    return ss.str();
  }
};

enum class PlayerState {
  Connected,     // Just connected, waiting for CONN
  AwaitingRooms, // Sent 00OK, waiting for RMRQ
  SendingRooms,  // Received RMRQ, busy sending room list
  AwaitingJoin   // Finished sending rooms, waiting for join request
};

class PlayerInfo final {
private:
  RemoteSocket sock;
  bool still_connected = true;

  std::thread acceptor_thread;
  std::thread sender_thread;

  std::atomic<bool> acceptor_stop = false;
  std::atomic<bool> sender_stop = false;

  CB::Buffer<MsgStruct, 128> msg_in;
  CB::Buffer<MsgStruct, 128> msg_out;

  auto acceptor() -> void {
    CB::Writer out{msg_in};
    std::array<char, 256> byte_buf{0};
    Net::Serde::MainParser parser{};

    std::cout << "Accepter thread started" << std::endl;

    while (!acceptor_stop) {

      const auto bytes_read =
          recv(sock.get_fd(), byte_buf.data(), byte_buf.size(), MSG_DONTWAIT);

      if (bytes_read == 0) {
        std::cout << "Client disconnected (FD: " << sock.get_fd() << ")\n";
        acceptor_stop = true;
        sender_stop = true;
        break;
      } else if (bytes_read < 0) { // No data or error
        std::this_thread::sleep_for(std::chrono::milliseconds(20));
        continue;
      }
      std::cout << "Recieved: " << bytes_read << std::endl;

      usize total_parsed_bytes = 0;
      Net::Serde::ParseResults results{};

      while (true) {
        const auto& start = byte_buf.begin() + total_parsed_bytes;
        const auto& end = byte_buf.begin() + bytes_read;

        results = parser.parse_bytes(std::string_view{start, end});

        if (results.error_occured) {
          // set stop to true
          acceptor_stop = true;
          return;
        }

        if (results.parser_done) {
          out.wait_and_insert(MsgStruct{results.code, results.payload});
          parser.reset_parser();
        }

        total_parsed_bytes += results.bytes_parsed;

        if (total_parsed_bytes >= bytes_read) {
          break;
        }
      }
    }

    std::cout << "Accepter thread ending" << std::endl;
  }

  auto sender() -> void {
    CB::Reader in{msg_out};
    while (!sender_stop) {
      const auto& maybe_msg = in.read();
      if (maybe_msg) {
        const auto& msg = maybe_msg.value();
        const auto& msg_str = msg.to_string();
        if (send(sock.get_fd(), msg_str.data(), msg_str.size(), 0) < 0) {
          std::cerr << "Send error, disconnecting client FD: " << sock.get_fd()
                    << "\n";
          sender_stop = true;
          acceptor_stop = true;
        }
      } else {
        std::this_thread::sleep_for(std::chrono::milliseconds{20});
      }
    }
  }

public:
  PlayerState state = PlayerState::Connected;
  int room_send_index = 0;

  CB::Reader<MsgStruct, 128> msg_in_reader;
  CB::Writer<MsgStruct, 128> msg_out_writer;

  virtual ~PlayerInfo() {
    sender_stop = true;
    acceptor_stop = true;
    sender_thread.join();
    acceptor_thread.join();
  }

  PlayerInfo(const ServerSocket& server_sock)
      : sock(server_sock), msg_in_reader(msg_in), msg_out_writer(msg_out) {}

  auto run() -> void {
    acceptor_thread = std::thread{[this] { acceptor(); }};
    sender_thread = std::thread([this]() { sender(); });
  }

  bool is_connected() const { return !acceptor_stop && !sender_stop; }
};

class Server final {
private:
  std::vector<std::unique_ptr<PlayerInfo>> players;
  std::vector<Room> rooms;
  std::mutex player_mutex;
  std::atomic<bool> running = false;
  std::thread logic_thread;

  void process_logic() {
    while (running) {
      { // Mutex scope
        std::lock_guard<std::mutex> lock(player_mutex);

        for (auto& player_ptr : players) {
          // this will process all messages
          // should be simple to reimplement as just one if one client
          // absolutely floods the server
          process_player_messages(*player_ptr);
        }

        players.erase(std::remove_if(players.begin(), players.end(),
                                     [](const std::unique_ptr<PlayerInfo>& p) {
                                       return !p->is_connected();
                                     }),
                      players.end());
      }

      std::this_thread::sleep_for(std::chrono::milliseconds(50));
    }
  }

  void process_player_messages(PlayerInfo& player) {
    while (true) {
      auto msg_opt = player.msg_in_reader.read();
      if (!msg_opt) {
        break; // No more messages
      }

      auto msg = msg_opt.value();
      std::cout << "Processing message (Code: " << msg.code << ") for state "
                << (int)player.state << "\n";

      switch (player.state) {

      case PlayerState::Connected:
        if (msg.code == "CONN") {
          std::cout << "Player sent CONN, sending 00OK\n";
          player.msg_out_writer.wait_and_insert({"00OK", std::nullopt});
          player.state = PlayerState::AwaitingRooms;
        }
        break;

      case PlayerState::AwaitingRooms:
        if (msg.code == "RMRQ") {
          std::cout << "Player sent RMRQ, starting room send\n";
          player.state = PlayerState::SendingRooms;
          player.room_send_index = 0;
          // Send the first room immediately
          send_room_info(player);
        }
        break;

      case PlayerState::SendingRooms:
        if (msg.code == "00OK") {
          send_room_info(player);
        }
        break;

      case PlayerState::AwaitingJoin:
        break;
      }
    }
  }

  void send_room_info(PlayerInfo& player) {
    if (player.room_send_index < rooms.size()) {
      const auto& room = rooms[player.room_send_index];
      std::string room_payload = room.to_payload_string();

      std::cout << "Sending room: " << room.name << "\n";
      player.msg_out_writer.wait_and_insert({"ROOM", room_payload});
      player.room_send_index++;
    } else {
      std::cout << "Done sending rooms, sending DONE\n";
      player.msg_out_writer.wait_and_insert({"DONE", std::nullopt});
      player.state = PlayerState::AwaitingJoin;
    }
  }

public:
  ~Server() {
    running = false;
    if (logic_thread.joinable()) {
      logic_thread.join();
    }
  }

  auto run(std::uint16_t port) -> void {
    std::cout << "Server starting" << std::endl;
    rooms.reserve(3);
    rooms.push_back({"1", "TestRoom1", 0, 4});
    rooms.push_back({"2", "BigStakes", 0, 8});
    rooms.push_back({"3", "Casuals", 0, 6});

    running = true;
    logic_thread = std::thread(&Server::process_logic, this);

    ServerSocket server_sock(port);
    std::cout << "Server listening on port " << port << std::endl;

    while (running) {
      try {
        std::cout << "Waiting for new connection...\n";

        auto new_player = std::make_unique<PlayerInfo>(server_sock);
        new_player->run();

        { // Mutex scope
          std::lock_guard<std::mutex> lock(player_mutex);
          players.push_back(std::move(new_player));
        }

        std::cout << "New player connection accepted and added to list.\n";

      } catch (const SocketException& e) {
        if (running) {
          std::cerr << "SocketException in accept loop: " << e.what()
                    << std::endl;
        }
      }
    }
  }
};
