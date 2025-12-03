#pragma once

#include "Babel.hpp"
#include "MessageSerde.hpp"
#include "PlayerInfo.hpp"
#include "RoomThread.hpp"
#include "SockWrapper.hpp"

#include <algorithm>
#include <chrono>
#include <cstddef>
#include <mutex>
#include <optional>
#include <sys/socket.h>
#include <thread>

using namespace std::chrono_literals;

class Server final {
private:
  vec<uq_ptr<PlayerInfo>> players;
  std::mutex player_mutex;
  vec<uq_ptr<Room>> rooms;
  std::atomic<bool> running = false;
  std::thread logic_thread;
  time_point<hr_clock> last_ping = hr_clock::now();

  void process_player_messages(PlayerInfo& player, Result& res) {
    for (usize i = 0; i < MSG_BATCH_SIZE; i++) {
      auto msg_opt = player.msg_client.reader.read();
      if (!msg_opt)
        break; // No more messages

      const auto& msg = msg_opt.value();
      std::cout << std::format("Processing (Code: {}) for state {} on FD {}\n",
                               msg.code, (int)player.state,
                               player.sock.get_fd());

      // Disconnect on unknown message code
      if (!is_valid_code(msg.code)) {
        std::cerr << std::format(
            "Unknown message code '{}' from FD {}, disconnecting\n", msg.code,
            player.sock.get_fd());
        player.disconnected = true;
        break;
      }

      // Route message based on player state
      switch (player.state) {
      case PlayerState::Connected:
        if (msg.code == Msg::CONN) {
          if (!msg.payload) {
            std::cerr << "CONN message without payload, disconnecting\n";
            player.disconnected = true;
            break;
          }
          handle_conn(player, msg.payload.value());
        } else {
          std::cerr << std::format("Unexpected message {} in Connected state\n",
                                   msg.code);
          player.disconnected = true;
        }
        break;

      case PlayerState::AwaitingReconnect:
        if (msg.code == Msg::RCON) {
          std::cout << "Player accepted reconnect" << std::endl;
          res.reconnect = true;
          return;
        } else if (msg.code == Msg::PINF) {
          if (!msg.payload) {
            std::cerr << "PINF message without payload, disconnecting\n";
            player.disconnected = true;
            break;
          }
          handle_pinf(player, msg.payload.value());
        } else {
          std::cerr << std::format(
              "Unexpected message {} in SendingRooms state\n", msg.code);
          player.disconnected = true;
        }

      case PlayerState::AwaitingRooms:
        if (msg.code == Msg::PINF) {
          if (!msg.payload) {
            std::cerr << "PINF message without payload, disconnecting\n";
            player.disconnected = true;
            break;
          }
          handle_pinf(player, msg.payload.value());
        } else {
          std::cerr << std::format(
              "Unexpected message {} in AwaitingRooms state\n", msg.code);
          player.disconnected = true;
        }
        break;

      case PlayerState::SendingRooms:
        if (msg.code == Msg::RMOK) {
          send_next_room(player);
        } else if (msg.code == Msg::RMFL) {
          std::cerr << "Client reported room receive failure, disconnecting\n";
          player.disconnected = true;
        } else {
          std::cerr << std::format(
              "Unexpected message {} in SendingRooms state\n", msg.code);
          player.disconnected = true;
        }
        break;

      case PlayerState::AwaitingJoin:
        if (msg.code == Msg::JOIN) {
          if (!msg.payload) {
            std::cerr << "JOIN message without payload, disconnecting\n";
            player.disconnected = true;
            break;
          }
          const auto& mb_room = handle_join(player, msg.payload.value());
          if (mb_room) {
            res.connect = true;
            res.room_idx = mb_room.value();
          }
          return;
        } else if (msg.code == Msg::RMRQ) {
          // Client can request room list again
          player.state = PlayerState::SendingRooms;
          player.room_send_index = 0;
          send_room_info(player);
        } else {
          std::cerr << std::format(
              "Unexpected message {} in AwaitingJoin state\n", msg.code);
          player.disconnected = true;
        }
        break;

      case PlayerState::InRoom:
        std::cerr << std::format(
            "Player in InRoom state but still in main list, disconnecting\n");
        player.disconnected = true;
        break;
      }

      // Check if player was marked disconnected during processing
      if (player.disconnected) {
        break;
      }
    }
  }

  bool is_valid_code(const str_v& code) {
    static const arr<str_v, 40> valid_codes = {
        Msg::CONN, Msg::PNOK, Msg::RCON, Msg::FAIL, Msg::PINF, Msg::PIOK,
        Msg::RMRQ, Msg::ROOM, Msg::DONE, Msg::RMOK, Msg::RMFL, Msg::RMUP,
        Msg::UPOK, Msg::UPFL, Msg::JOIN, Msg::JNOK, Msg::JNFL, Msg::RMST,
        Msg::STOK, Msg::STFL, Msg::RDY1, Msg::PRDY, Msg::GMST, Msg::CDTP,
        Msg::PTRN, Msg::CHCK, Msg::FOLD, Msg::CALL, Msg::BETT, Msg::ACOK,
        Msg::ACFL, Msg::NYET, Msg::SDWN, Msg::SDOK, Msg::SDFL, Msg::GMDN,
        Msg::DNOK, Msg::DNFL, Msg::DCON};

    return std::find(valid_codes.begin(), valid_codes.end(), code) !=
           valid_codes.end();
  }

  void handle_conn(PlayerInfo& player, const str& payload) {
    // Parse nickname from payload
    const auto& nick_opt = Net::Serde::read_str(payload);
    if (!nick_opt) {
      std::cerr << "Failed to parse nickname from CONN payload\n";
      player.send_message({str{Msg::FAIL}, null});
      player.disconnected = true;
      return;
    }

    const auto& [nickname, _] = nick_opt.value();
    player.nickname = nickname;

    // Check if player is already in a room (reconnect logic)
    for (usize i = 0; i < rooms.size(); i++) {
      const auto& room = *rooms[i];
      for (const auto& seat : room.ctx.seats) {
        if (seat.is_occupied && seat.nickname == nickname &&
            seat.connection == nullptr) {
          std::cout << std::format("Reconnect candidate {} found in room {}\n",
                                   nickname, i);
          player.reconnect_index = i;
          player.send_message({str{Msg::RCON}, null});
          player.state = PlayerState::AwaitingReconnect;
          return;
        }
      }
    }

    // New player
    std::cout << std::format("New player {} connected\n", nickname);
    player.send_message({str{Msg::PNOK}, null});
    player.state = PlayerState::AwaitingRooms;
  }

  void handle_pinf(PlayerInfo& player, const str& payload) {
    const auto& mb_chips = Net::Serde::read_var_int(payload);
    if (!mb_chips) {
      std::cout << "Player sent malformed chips" << std::endl;
      player.disconnected = true;
      return;
    }
    const auto [chips, _] = mb_chips.value();

    player.chips = chips;

    std::cout << std::format(
        "Received player info from {} ({}), sending PIOK\n", player.nickname,
        player.chips);

    player.send_message({str{Msg::PIOK}, null});
    player.state = PlayerState::AwaitingJoin;
  }

  void send_room_info(PlayerInfo& player) {
    if (player.room_send_index < rooms.size()) {
      const auto& room = *rooms[player.room_send_index];
      std::string room_payload = room.serialize();

      std::cout << std::format("Sending room {} to {}\n", room.name,
                               player.nickname);
      player.send_message({str{Msg::ROOM}, room_payload});
      player.room_send_index++;
    } else {
      std::cout << std::format("Done sending rooms to {}, sending DONE\n",
                               player.nickname);
      player.send_message({str{Msg::DONE}, null});
      player.state = PlayerState::AwaitingJoin;
    }
  }

  void send_next_room(PlayerInfo& player) { send_room_info(player); }

  opt<usize> handle_join(PlayerInfo& player, const str& payload) {
    const auto& id_opt = Net::Serde::read_bg_int(payload);
    if (!id_opt) {
      std::cerr << "Failed to parse room ID from JOIN payload\n";
      player.send_message({str{Msg::JNFL}, null});
      return null;
    }

    const auto& [req_id, _] = id_opt.value();

    for (usize i = 0; i < rooms.size(); i++) {
      const auto& room = *rooms[i];
      if (room.id == req_id) {
        if (!room.can_player_join()) {
          std::cerr << std::format("Room {} full, rejecting {}\n", req_id,
                                   player.nickname);
          player.send_message({str{Msg::JNFL}, null});
          return null;
        }

        std::cout << std::format("Accepted {} into room {}\n", player.nickname,
                                 req_id);
        player.send_message({str{Msg::JNOK}, null});
        return i;
      }
    }

    std::cerr << std::format("Room {} not found for {}\n", req_id,
                             player.nickname);
    player.send_message({str{Msg::JNFL}, null});
    return null;
  }

  auto process_logic() -> void {
    Result res{};
    while (running) {
      const auto now = hr_clock::now();
      const auto diff = dur_cast<seconds>(now - last_ping);

      if (diff.count() > 10) {
        for (i64 p_idx = players.size() - 1; p_idx > 0; p_idx--) {
          if (!players[p_idx]->is_connected()) {
            continue;
          }

          if (!players[p_idx]->get_ping()) {
            players[p_idx]->disconnected = true;
            // disconnect player
          }

          players[p_idx]->clear_ping();
          players[p_idx]->send_ping();
        }
      }

      std::erase_if(players, [](const uq_ptr<PlayerInfo>& p) {
        return !p->is_connected();
      });

      { // normal message handling
        std::lock_guard g{player_mutex};

        // Process each player's messages
        // If connect/reconnect happens -> move the player
        for (i64 p_idx = players.size() - 1; p_idx > 0; p_idx--) {
          res.reset();
          process_player_messages(*players[p_idx], res);

          if (res.reconnect) {
            rooms[res.room_idx]->reconnect_player(std::move(players[p_idx]));
          }
          if (res.connect) {
            rooms[res.room_idx]->accept_player(std::move(players[p_idx]));
          }
          players.erase(players.begin() + p_idx);
        }
      }

      std::this_thread::sleep_for(10ms);
    }
  }

public:
  Server() {
    // Create some default rooms
    rooms.emplace_back(
        std::make_unique<Room>(1, "Room 1", players, player_mutex));
    rooms.emplace_back(
        std::make_unique<Room>(2, "Room 2", players, player_mutex));
    rooms.emplace_back(
        std::make_unique<Room>(3, "Room 3", players, player_mutex));
    rooms.emplace_back(
        std::make_unique<Room>(4, "Room 4", players, player_mutex));
  }

  ~Server() {
    running = false;
    if (logic_thread.joinable()) {
      logic_thread.join();
    }
  }

  auto run(std::uint16_t port) -> void {
    std::cout << "Server starting on port " << port << std::endl;

    running = true;
    logic_thread = std::thread{&Server::process_logic, this};

    ServerSocket server_sock(port);

    while (running) {
      try {
        std::cout << "Waiting for new connection...\n";
        auto new_player = std::make_unique<PlayerInfo>(server_sock);

        {
          std::lock_guard<std::mutex> lock(player_mutex);
          players.push_back(std::move(new_player));
        }

        std::cout << "New connection accepted and added to player list\n";
      } catch (const SocketException& e) {
        if (running) {
          std::cerr << "SocketException in accept loop: " << e.what()
                    << std::endl;
        }
      }
    }
  }
};
