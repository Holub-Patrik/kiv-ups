#pragma once

#include "Babel.hpp"

#include "MessageSerde.hpp"
#include "PlayerInfo.hpp"
#include "RoomThread.hpp"
#include "SockWrapper.hpp"

#include <mutex>
#include <optional>
#include <sys/socket.h>
#include <thread>

using namespace std::chrono_literals;

class Server final {
private:
  vec<uq_ptr<PlayerInfo>> players;
  std::mutex player_mutex;
  // has to be unique_ptr since Room has logic which requires that copy and move
  // be deleted
  vec<uq_ptr<Room>> rooms;
  std::atomic<bool> running = false;
  std::thread logic_thread;

  auto process_logic() -> void {
    while (running) {
      {
        std::lock_guard g{player_mutex};

        vec<pair<usize, usize>> players_to_move{};
        for (usize i = 0; i < players.size(); i++) {
          const auto wants_to_join = process_player_messages(*players[i]);

          if (wants_to_join) {
            players_to_move.push_back({i, wants_to_join.value()});
          }
        }

        for (const auto& [p_idx, room_idx] : players_to_move) {
          // flush all pending messages before moving to a room
          players[p_idx]->flush_messages();
          rooms[room_idx]->accept_player(std::move(players[p_idx]));
          players.erase(players.begin() + p_idx);
        }

        players.erase(std::remove_if(players.begin(), players.end(),
                                     [](const std::unique_ptr<PlayerInfo>& p) {
                                       return !p->is_connected();
                                     }),
                      players.end());
      }

      std::this_thread::sleep_for(50ms);
    }
  }

  auto process_player_messages(PlayerInfo& player) -> opt<usize> {
    for (usize i = 0; i < MSG_BATCH_SIZE; i++) {
      player.accept_messages();

      auto msg_opt = player.msg_client.reader.read();

      if (!msg_opt) {
        break; // No more messages
      }

      bool room_exists = false;
      usize room_idx = 0;

      auto msg = msg_opt.value();
      std::cout << "Processing message (Code: " << msg.code << ") for state "
                << (int)player.state << "\n";

      switch (player.state) {

      case PlayerState::Connected:
        if (msg.code == "CONN") {
          std::cout << "Player sent CONN, sending 00OK\n";
          player.msg_client.writer.wait_and_insert({"00OK", null});
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
        if (msg.code == "JOIN") {
          const auto& maybe_id = Net::Serde::read_bg_int(msg.payload.value());

          if (!maybe_id) {
            player.msg_client.writer.wait_and_insert({"FAIL", null});
            // here I should dconn the player since he sent weird data
          }

          const auto& [req_id, _] = maybe_id.value();

          for (usize i = 0; i < rooms.size(); i++) {
            const auto& room = *rooms[i];
            if (room.id == req_id) {
              if (!room.can_player_join()) {
                player.msg_client.writer.wait_and_insert({"FAIL", null});
                break;
              } else {
                room_exists = true;
                room_idx = i;
                break;
              }
            }
          }
        }
        break;
      }

      player.send_messages();

      if (room_exists) {
        return room_idx;
      }
    }
    return null;
  }

  void send_room_info(PlayerInfo& player) {
    if (player.room_send_index < rooms.size()) {
      const auto& room = *rooms[player.room_send_index];
      std::string room_payload = room.to_payload_string();

      std::cout << "Sending room: " << room.name << "\n";
      player.msg_client.writer.wait_and_insert({"ROOM", room_payload});
      player.room_send_index++;
    } else {
      std::cout << "Done sending rooms, sending DONE\n";
      player.msg_client.writer.wait_and_insert({"DONE", null});
      player.state = PlayerState::AwaitingJoin;
    }
  }

public:
  Server(/* add config obj/struct here */) {
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
    std::cout << "Server starting" << std::endl;

    running = true;
    logic_thread = std::thread(&Server::process_logic, this);

    ServerSocket server_sock(port);
    std::cout << "Server listening on port " << port << std::endl;

    while (running) {
      try {
        std::cout << "Waiting for new connection...\n";

        auto new_player = std::make_unique<PlayerInfo>(server_sock);

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
