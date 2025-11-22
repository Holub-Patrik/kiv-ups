#pragma once

#include "CircularBufferQueue.hpp"
#include "MessageSerde.hpp"
#include "SockWrapper.hpp"

constexpr std::size_t MSG_BATCH_SIZE = 10;
constexpr int MAX_CONSECUTIVE_ERRORS = 3;
constexpr int MAX_FAST_FORWARD_BYTES = 100;

enum class PlayerState {
  Connected,     // Just connected, waiting for CONN
  AwaitingRooms, // Sent 00OK, waiting for RMRQ
  SendingRooms,  // Received RMRQ, busy sending room list
  AwaitingJoin   // Finished sending rooms, waiting for join request
};

class PlayerInfo final {
private:
  RemoteSocket sock;

  CB::Buffer<Net::MsgStruct, 128> msg_in;
  CB::Buffer<Net::MsgStruct, 128> msg_out;

  CB::Reader<Net::MsgStruct, 128> msg_in_writer;
  CB::Reader<Net::MsgStruct, 128> msg_out_reader;

public:
  // created when remote sock is accepted, thus it should be false by default
  bool disconnected = false;
  int room_send_index = 0;
  int invalid_msg_count = 0;
  PlayerState state = PlayerState::Connected;

  CB::Reader<Net::MsgStruct, 128> msg_in_reader;
  CB::Writer<Net::MsgStruct, 128> msg_out_writer;

  Net::Serde::MainParser parser{};

  virtual ~PlayerInfo() {}

  PlayerInfo(const ServerSocket& server_sock)
      : sock(server_sock), msg_in_reader(msg_in), msg_in_writer(msg_in),
        msg_out_reader(msg_out), msg_out_writer(msg_out) {}

  auto accept_messages() -> void {
    std::array<char, 256> byte_buf{0};

    for (std::size_t i = 0; i < MSG_BATCH_SIZE; i++) {

      const auto bytes_read =
          recv(sock.get_fd(), byte_buf.data(), byte_buf.size(), MSG_DONTWAIT);

      if (bytes_read == 0) {
        std::cout << "Client disconnected (FD: " << sock.get_fd() << ")\n";
        disconnected = true;
        break;
      } else if (bytes_read < 0) { // No data or error
        break;
      }
      std::cout << "Recieved: " << bytes_read << std::endl;

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
          // Valid message received, reset error counter
          invalid_msg_count = 0;
          std::cout << std::format("Msg parsed -> Code: {}", results.code,
                                   results.payload.has_value()
                                       ? " | Payload: {}" +
                                             results.payload.value()
                                       : "")
                    << std::endl;
          msg_out_writer.wait_and_insert(
              Net::MsgStruct{results.code, results.payload});
          parser.reset_parser();
        }

        total_parsed_bytes += results.bytes_parsed;
      }
    }
  }

  auto send_messages() -> void {
    for (std::size_t i = 0; i < MSG_BATCH_SIZE; i++) {
      const auto& maybe_msg = msg_out_reader.read();
      if (maybe_msg) {
        const auto& msg = maybe_msg.value();
        const auto& msg_str = msg.to_string();
        if (send(sock.get_fd(), msg_str.data(), msg_str.size(), 0) < 0) {
          std::cerr << "Send error, disconnecting client FD: " << sock.get_fd()
                    << "\n";
          disconnected = true;
        }
      } else {
        break;
      }
    }
  }

  // use sparingly, this reads until all messages are processed
  auto flush_messages() -> void {
    CB::Reader in{msg_out};
    while (true) {
      const auto& maybe_msg = in.read();
      if (maybe_msg) {
        const auto& msg = maybe_msg.value();
        const auto& msg_str = msg.to_string();
        if (send(sock.get_fd(), msg_str.data(), msg_str.size(), 0) < 0) {
          std::cerr << "Send error, disconnecting client FD: " << sock.get_fd()
                    << "\n";
          disconnected = true;
        }
      } else {
        break;
      }
    }
  }

  bool is_connected() const { return !disconnected; }
};
