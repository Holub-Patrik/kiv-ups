#pragma once

#include <cerrno>
#include <cstring>
#include <exception>
#include <iostream>
#include <string>

extern "C" {
#include <asm-generic/socket.h>
#include <fcntl.h>
#include <netinet/in.h>
#include <sys/socket.h>
#include <sys/types.h>
#include <sys/un.h>
#include <unistd.h>
}

#include "Babel.hpp"

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

  explicit ServerSocket(u16 port, u32 ip = INADDR_ANY) {
    sock_fd = socket(AF_INET, SOCK_STREAM, 0);
    if (sock_fd <= 0) {
      throw SocketException(SocketExceptionType::SOCK);
    }

    memset(&addr, 0, sizeof(struct sockaddr_in));
    addr.sin_family = AF_INET;
    addr.sin_port = htons(port);
    addr.sin_addr.s_addr = ip;

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

class RemoteSocket final {
private:
  bool closed = false;
  int sock_fd;
  struct sockaddr_in addr;
  socklen_t addr_len;

  void close_fd_internal() {
    if (!closed) {
      closed = true;
      shutdown(sock_fd, SHUT_RDWR);
      close(sock_fd);
    }
  }

public:
  ~RemoteSocket() {
    close_fd_internal();
    std::cout << "Closed socket: " << sock_fd << std::endl;
  }

  RemoteSocket(const ServerSocket& server) {
    const auto accepted_sock =
        accept(server.get_fd(), (struct sockaddr*)&addr, &addr_len);
    if (accepted_sock <= 0) {
      throw SocketException{SocketExceptionType::ACCEPT};
    }

    sock_fd = accepted_sock;
  }

  RemoteSocket(const RemoteSocket& sock) = delete;
  RemoteSocket& operator=(const RemoteSocket& sock) = delete;

  RemoteSocket(RemoteSocket&& sock)
      : sock_fd(sock.sock_fd), addr(sock.addr), addr_len(sock.addr_len) {
    sock.sock_fd = -1;
    sock.addr_len = -1;
    memset(&sock.addr, 0, sizeof(typeof(sock.addr)));
  }

  RemoteSocket& operator=(RemoteSocket&& sock) {
    sock_fd = sock.sock_fd;
    addr_len = sock.addr_len;
    addr = sock.addr;

    sock.sock_fd = -1;
    sock.addr_len = -1;
    memset(&sock.addr, 0, sizeof(typeof(sock.addr)));

    return *this;
  }

  int get_fd() const noexcept { return sock_fd; }
  void close_fd() { close_fd_internal(); }
};
