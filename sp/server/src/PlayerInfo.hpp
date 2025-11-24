#pragma once

#include "CircularBufferQueue.hpp"
#include "MessageSerde.hpp"
#include "SockWrapper.hpp"
#include <ostream>

constexpr std::size_t MSG_BATCH_SIZE = 10;
constexpr int MAX_CONSECUTIVE_ERRORS = 3;
constexpr int MAX_FAST_FORWARD_BYTES = 100;

enum class PlayerState {
  Connected,     // Just connected, waiting for CONN
  AwaitingRooms, // Sent PNOK, waiting for PINF
  SendingRooms,  // Received PINF, sending room list
  AwaitingJoin,  // Finished sending rooms, waiting for JOIN
  InRoom         // Player is in a room
};

class PlayerInfo final {
private:
  CB::TwinBuffer<Net::MsgStruct, 128> msg_buf;
  CB::Server<Net::MsgStruct, 128> msg_server;

public:
  bool disconnected = false;
  int room_send_index = 0;
  int invalid_msg_count = 0;
  str nickname;
  PlayerState state = PlayerState::Connected;
  CB::Client<Net::MsgStruct, 128> msg_client;
  Net::Serde::MainParser parser{};
  RemoteSocket sock;

  virtual ~PlayerInfo() {}

  PlayerInfo(const ServerSocket& server_sock)
      : sock(server_sock), msg_server(msg_buf), msg_client(msg_buf) {}

  // Reset all connection-specific state
  void reset() {
    invalid_msg_count = 0;
    room_send_index = 0;
    parser.reset_parser();
  }

  auto accept_messages() -> void {
    std::array<char, 256> byte_buf{0};

    for (std::size_t i = 0; i < MSG_BATCH_SIZE; i++) {
      const auto bytes_read =
          recv(sock.get_fd(), byte_buf.data(), byte_buf.size(), MSG_DONTWAIT);

      if (bytes_read == 0) {
        std::cout << "Client disconnected (FD: " << sock.get_fd() << ")\n";
        disconnected = true;
        break;
      } else if (bytes_read < 0) {
        break; // No data or error
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
          invalid_msg_count++;
          std::cerr << "Protocol Error " << invalid_msg_count << "/"
                    << MAX_CONSECUTIVE_ERRORS << " on FD: " << sock.get_fd()
                    << "\n";

          if (invalid_msg_count >= MAX_CONSECUTIVE_ERRORS) {
            std::cerr << "Too many errors, disconnecting FD: " << sock.get_fd()
                      << "\n";
            disconnected = true;
            return;
          }

          // Try to resync
          parser.reset_parser();
          total_parsed_bytes++;

          usize scanned = 0;
          bool found_sync = false;

          while (total_parsed_bytes < static_cast<usize>(bytes_read) &&
                 scanned < MAX_FAST_FORWARD_BYTES) {
            auto state = parser.parse_byte(byte_buf[total_parsed_bytes]);
            parser.reset_parser();

            if (state != Net::Serde::ParserState::Invalid) {
              found_sync = true;
              break;
            }
            total_parsed_bytes++;
            scanned++;
          }

          if (found_sync) {
            std::cout << "Resync successful at offset " << total_parsed_bytes
                      << "\n";
          } else {
            std::cout << "Resync failed (limit or buffer end reached).\n";
          }

          continue;
        }

        if (results.parser_done) {
          // Valid message - reset error counter
          invalid_msg_count = 0;

          // Extract payload if present
          opt<str> payload = null;
          if (results.type == Net::Serde::MsgType::Payload && results.payload) {
            payload = results.payload;
          }

          std::cout << std::format("Msg parsed -> Code: {}{}", results.code,
                                   payload ? " | Payload: " + payload.value()
                                           : "")
                    << std::endl;

          // Insert into message queue for processing
          msg_server.writer.wait_and_insert(
              Net::MsgStruct{results.code, payload});
          parser.reset_parser();
        }

        total_parsed_bytes += results.bytes_parsed;
      }
    }
  }

  auto send_messages() -> void {
    for (std::size_t i = 0; i < MSG_BATCH_SIZE; i++) {
      const auto& maybe_msg = msg_server.reader.read();
      if (!maybe_msg)
        break;

      const auto& msg = maybe_msg.value();
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
  }

  // Flush all pending messages
  auto flush_messages() -> void {
    while (true) {
      const auto& maybe_msg = msg_server.reader.read();
      if (!maybe_msg)
        break;

      const auto& msg = maybe_msg.value();
      const auto& msg_str = msg.to_string();

      if (send(sock.get_fd(), msg_str.data(), msg_str.size(), 0) < 0) {
        std::cerr << "Send error, disconnecting client FD: " << sock.get_fd()
                  << "\n";
        disconnected = true;
        return;
      }
    }
  }

  bool is_connected() const { return !disconnected; }
};
