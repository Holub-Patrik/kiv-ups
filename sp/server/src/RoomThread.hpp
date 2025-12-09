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
enum class PlayerAction {
  None = 0,
  Check,
  Call,
  Fold,
  Bet,
  Left,
};

} // namespace GameUtils

// Persistent data for a player slot
struct PlayerSeat {
  bool is_occupied = false;

  str nickname;
  int chips = 1000;
  int round_bet = 0;
  int total_bet = 0;

  bool is_folded = false;
  bool is_ready = false;
  bool showdowm_okay = false;

  vec<u8> hand;
  GameUtils::PlayerAction action_taken = GameUtils::PlayerAction::None;
  usize action_amount = 0;
  uq_ptr<PlayerInfo> connection = nullptr;

  void reset_round();
  void reset_game();
  bool is_active() const;
};

struct RoomContext {
  std::vector<PlayerSeat> seats;
  GameUtils::Deck deck;
  vec<u8> community_cards;
  int pot = 0;
  int current_high_bet = 0;
  int dealer_idx = 0;
  int current_actor = -1;
  bool room_locked = false;
  RoundPhase round_phase = RoundPhase::PreFlop;

  RoomContext() = delete;
  RoomContext(int p_count);

  int count_active_players() const;
  int count_occupied_seats() const;
  void broadcast(const str_v& code, const opt<str>& payload);
  void broadcast_ex(const int seat_idx, const str_v& code,
                    const opt<str>& payload);
  void send_to(const int seat_idx, const str_v& code, const opt<str>& payload);
  str serialize() const;
};

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

// Room Class
class Room final {
private:
  std::thread room_thread;
  std::atomic<bool> running = false;

  std::mutex incoming_mtx;
  vec<uq_ptr<PlayerInfo>> incoming_queue;
  std::mutex& return_mtx;
  vec<uq_ptr<PlayerInfo>>& return_arr;

  uq_ptr<RoomState> current_state;
  uq_ptr<RoomState> next_state_ptr;
  bool pending_transition = false;

  time_point<hr_clock> last_ping = hr_clock::now();

  void player_leave(usize seat);

public:
  usize id;
  str name;
  RoomContext ctx;

  Room(usize id, str name, vec<uq_ptr<PlayerInfo>>& return_vec,
       std::mutex& return_mutex, usize p_count);
  ~Room();

  template <typename T, typename... Args> void transition_to(Args&&... args) {
    next_state_ptr = std::make_unique<T>(std::forward<Args>(args)...);
    pending_transition = true;
  }

  void accept_player(uq_ptr<PlayerInfo>&& p);
  void reconnect_player(uq_ptr<PlayerInfo>&& p);
  str serialize() const;
  bool can_player_join(const str& p_name = "") const;
  void room_logic();

private:
  void process_incoming_players();
  void process_network_io();
};

// Concrete State Classes
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
  bool has_bet_occurred = false;
  time_point<hr_clock> last_action_time;

  void start_next_turn(RoomContext& ctx);
  void requeue_others(RoomContext&, int aggressor_idx);
  opt<str> check_bet_conditions(const PlayerSeat& seat,
                                const Net::MsgStruct& msg);

public:
  void on_enter(Room& room, RoomContext& ctx) override;
  void on_leave(Room& room, RoomContext& ctx) override;
  void on_tick(Room& room, RoomContext& ctx) override;
  void on_message(Room& room, RoomContext& ctx, int seat_idx,
                  const Net::MsgStruct& msg) override;
  str get_name() const override { return "Betting"; }
};

class ShowdownState : public RoomState {
private:
  time_point<hr_clock> sd_ok_timeout_start;

public:
  void on_enter(Room& room, RoomContext& ctx) override;
  void on_leave(Room& room, RoomContext& ctx) override;
  void on_tick(Room& room, RoomContext& ctx) override;
  void on_message(Room& room, RoomContext& ctx, int seat_idx,
                  const Net::MsgStruct& msg) override;
  str get_name() const override { return "Showdown"; }
};
