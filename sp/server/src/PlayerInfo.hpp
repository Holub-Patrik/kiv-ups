#pragma once

#include "CircularBufferQueue.hpp"
#include "MessageSerde.hpp"
#include "SockWrapper.hpp"

#include <ostream>
#include <sys/socket.h>
#include <unistd.h>

constexpr std::size_t MSG_BATCH_SIZE = 10;
constexpr int MAX_CONSECUTIVE_ERRORS = 3;
constexpr int MAX_FAST_FORWARD_BYTES = 100;

enum class PlayerState {
  Connected, // Just connected, waiting for CONN
  AwaitingReconnect,
  AwaitingRooms, // Sent PNOK, waiting for PINF
  SendingRooms,  // Received PINF, sending room list
  AwaitingJoin,  // Finished sending rooms, waiting for JOIN
  InRoom         // Player is in a room
};

class PlayerInfo final {
private:
  CB::TwinBuffer<Net::MsgStruct, 128> msg_buf;
  CB::Server<Net::MsgStruct, 128> msg_server;

  std::thread send_t;

  bool ping_received = true;
  bool disconnected = false;

public:
  int room_send_index = 0;
  int invalid_msg_count = 0;
  int reconnect_index = 0;

  str nickname;
  u64 chips;

  PlayerState state = PlayerState::Connected;
  CB::Client<Net::MsgStruct, 128> msg_client;
  Net::Serde::MainParser parser{};
  RemoteSocket sock;

  virtual ~PlayerInfo() {
    sock.close_fd();
    send_t.join();
  }

  PlayerInfo(const ServerSocket& server_sock)
      : sock(server_sock), msg_server(msg_buf), msg_client(msg_buf) {
    send_t = std::thread{&PlayerInfo::accept_messages, this};
  }

  void reset() {
    invalid_msg_count = 0;
    room_send_index = 0;
    parser.reset_parser();
  }

  void clear_ping() { ping_received = false; }
  bool get_ping() const noexcept { return ping_received; }
  void send_ping() {
    const auto msg = Net::MsgStruct{"PING", null};
    const auto& msg_str = msg.to_string();

    const auto sent_bytes =
        send(sock.get_fd(), msg_str.data(), msg_str.size(), 0);
    if (sent_bytes < 0) {
      std::cerr << "Send error, disconnecting client FD: " << sock.get_fd()
                << "\n";
      disconnected = true;
      return;
    }
  }

  void accept_messages() {
    std::array<char, 256> byte_buf{0};

    while (!disconnected) {
      const auto bytes_read =
          recv(sock.get_fd(), byte_buf.data(), byte_buf.size(), 0);

      if (bytes_read == 0) {
        std::cout << "Client disconnected (FD: " << sock.get_fd() << ")\n";
        disconnected = true;
        break;
      } else if (bytes_read < 0) {
        std::cout << "Read error" << std::endl;
        break; // error
      }

      std::cout << "Received " << bytes_read << " bytes on FD " << sock.get_fd()
                << std::endl;

      usize total_parsed_bytes = 0;
      Net::Serde::ParseResults results{};

      while (total_parsed_bytes < static_cast<usize>(bytes_read)) {
        const auto& start = byte_buf.begin() + total_parsed_bytes;
        const auto& end = byte_buf.begin() + bytes_read;

        results = parser.parse_bytes(std::string_view{start, end});

        if (results.error_occured) {
          std::cerr << "Error occured on FD: " << sock.get_fd() << "\n";
          disconnected = true;
          return;
        }

        if (results.parser_done) {
          invalid_msg_count = 0;

          opt<str> payload = null;
          if (results.type == Net::Serde::MsgType::Payload && results.payload) {
            payload = results.payload;
          }

          std::cout << std::format("Msg parsed -> Code: {}{}", results.code,
                                   payload ? " | Payload: " + payload.value()
                                           : "")
                    << std::endl;

          if (results.code == "PING") {
            ping_received = true;
          } else {
            msg_server.writer.wait_and_insert(
                Net::MsgStruct{results.code, payload});
          }

          total_parsed_bytes += results.bytes_parsed;
          parser.reset_parser();
          continue;
        }

        total_parsed_bytes += results.bytes_parsed;
      }
    }
  }

  auto send_message(const Net::MsgStruct& msg) -> void {
    const auto& msg_str = msg.to_string();

    std::cout << std::format("Sending -> Code: {}{}", msg.code,
                             msg.payload ? "| Payload: " + msg.payload.value()
                                         : "")
              << std::endl;

    const auto sent_bytes =
        send(sock.get_fd(), msg_str.data(), msg_str.size(), 0);
    if (sent_bytes < 0) {
      std::cerr << "Send error, disconnecting client FD: " << sock.get_fd()
                << "\n";
      disconnected = true;
      return;
    }
  }

  bool is_connected() const { return !disconnected; }
  void disconnect() { disconnected = true; }
};
