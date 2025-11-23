#pragma once

#include "Babel.hpp"
#include "MessageSerde.hpp"
#include "PlayerInfo.hpp"

#include <array>
#include <deque>
#include <memory>
#include <mutex>
#include <random>

// Forward declarations
class Room;
class RoomState;

constexpr usize ROOM_MAX_PLAYERS = 4;

enum class RoundPhase { PreFlop, Flop, Turn, River };

namespace GameUtils {
struct Card {
  u8 value; // 0-51
  str to_string() const;
};

class Deck {
private:
  vec<u8> cards;
  std::mt19937 rng{std::random_device{}()};

public:
  Deck();
  void shuffle();
  uint8_t draw();
  void reset();
};
} // namespace GameUtils

// Persistent data for a player slot (survives disconnects)
struct PlayerSeat {
  bool is_occupied = false;
  str nickname;
  int chips = 1000; // Default buy-in
  int current_bet = 0;
  bool is_folded = false;
  bool is_ready = false;
  vec<u8> hand;

  // The active connection (nullptr if disconnected)
  PlayerInfo* connection = nullptr;

  void reset_round();
  void reset_game();
  bool is_active() const;
};

// Shared Context Data (The "Board")
struct RoomContext {
  std::array<PlayerSeat, ROOM_MAX_PLAYERS> seats;
  GameUtils::Deck deck;
  vec<u8> community_cards;
  int pot = 0;
  int dealer_idx = 0;
  RoundPhase round_phase = RoundPhase::PreFlop;

  // Helpers
  int count_active_players() const;
  void broadcast(const str& code, const opt<str>& payload);
  void send_to(int seat_idx, const str& code, const opt<str>& payload);
};

// -----------------------------------------------------------
// FSM Interface
// -----------------------------------------------------------
class RoomState {
public:
  virtual ~RoomState() = default;
  virtual void on_enter(Room& room, RoomContext& ctx) = 0;
  virtual void on_tick(Room& room, RoomContext& ctx) = 0;
  virtual void on_leave(Room& room, RoomContext& ctx) = 0;
  virtual void on_message(Room& room, RoomContext& ctx, int seat_idx,
                          const Net::MsgStruct& msg) = 0;
  virtual str get_name() const = 0;
};

// -----------------------------------------------------------
// Room Class (The Context)
// -----------------------------------------------------------
class Room final {
private:
  std::thread room_thread;
  std::atomic<bool> running = false;
  std::mutex incoming_mtx;
  vec<uq_ptr<PlayerInfo>> incoming_queue;
  vec<uq_ptr<PlayerInfo>>& return_arr;
  std::mutex& return_mtx;

  uq_ptr<RoomState> current_state;
  uq_ptr<RoomState> next_state_ptr;
  bool pending_transition = false;

public:
  usize id;
  str name;
  RoomContext ctx;

  Room(std::size_t id, str name, vec<uq_ptr<PlayerInfo>>& return_vec,
       std::mutex& return_mutex);
  ~Room();

  template <typename T, typename... Args> void transition_to(Args&&... args) {
    next_state_ptr = std::make_unique<T>(std::forward<Args>(args)...);
    pending_transition = true;
  }

  void accept_player(uq_ptr<PlayerInfo>&& p);
  str to_payload_string() const;
  bool can_player_join() const;

  void room_logic();

private:
  void process_incoming_players();
  void process_network_io();
};

// -----------------------------------------------------------
// Concrete States Classes
// -----------------------------------------------------------

class LobbyState : public RoomState {
public:
  void on_enter(Room& room, RoomContext& ctx) override;
  void on_leave(Room& room, RoomContext& ctx) override;
  void on_tick(Room& room, RoomContext& ctx) override;
  void on_message(Room& room, RoomContext& ctx, int seat_idx,
                  const Net::MsgStruct& msg) override;
  str get_name() const override { return "Lobby"; }
};

class DealingState : public RoomState {
public:
  void on_enter(Room& room, RoomContext& ctx) override;
  void on_leave(Room& room, RoomContext& ctx) override;
  void on_tick(Room& room, RoomContext& ctx) override;
  void on_message(Room& room, RoomContext& ctx, int seat_idx,
                  const Net::MsgStruct& msg) override;
  str get_name() const override { return "Dealing"; }
};

// Handles Flop, Turn, and River card reveals
class CommunityCardState : public RoomState {
public:
  void on_enter(Room& room, RoomContext& ctx) override;
  void on_leave(Room& room, RoomContext& ctx) override;
  void on_tick(Room& room, RoomContext& ctx) override;
  void on_message(Room& room, RoomContext& ctx, int seat_idx,
                  const Net::MsgStruct& msg) override;
  str get_name() const override { return "CommunityCard"; }
};

class BettingState : public RoomState {
private:
  std::deque<int> action_queue;
  int current_actor = -1;
  int current_high_bet = 0;
  bool has_bet_occurred = false;

  void start_next_turn(RoomContext& ctx);
  void requeue_others(RoomContext&, int aggressor_idx);

public:
  void on_enter(Room& room, RoomContext& ctx) override;
  void on_leave(Room& room, RoomContext& ctx) override;
  void on_tick(Room& room, RoomContext& ctx) override;
  void on_message(Room& room, RoomContext& ctx, int seat_idx,
                  const Net::MsgStruct& msg) override;
  str get_name() const override { return "Betting"; }
};

class ShowdownState : public RoomState {
public:
  void on_enter(Room& room, RoomContext& ctx) override;
  void on_leave(Room& room, RoomContext& ctx) override;
  void on_tick(Room& room, RoomContext& ctx) override;
  void on_message(Room& room, RoomContext& ctx, int seat_idx,
                  const Net::MsgStruct& msg) override;
  str get_name() const override { return "Showdown"; }
};
