#pragma once

#include "Babel.hpp"

#include "MessageSerde.hpp"
#include "PlayerInfo.hpp"

#include <algorithm>
#include <deque>
#include <format>
#include <memory>
#include <mutex>
#include <random>
#include <vector>

constexpr usize ROOM_MAX_PLAYERS = 4;
constexpr int TURN_TIMEOUT_SECONDS = 30; // Reduced for testing

enum class RoomState {
  WaitingForStart,
  Dealing,
  BettingRound,
  RevealCommunity,
  Showdown,
  Finished
};

enum class RoundPhase { PreFlop, Flop, Turn, River };

namespace {

struct Card {
  u8 value; // 0-51
  str to_string() const { return std::format("{:02}", value); }
};

class Deck {
private:
  vec<u8> cards;
  std::mt19937 rng{std::random_device{}()};

public:
  Deck() { reset(); }

  void shuffle() { std::shuffle(cards.begin(), cards.end(), rng); }

  uint8_t draw() {
    if (cards.empty())
      return 255;
    uint8_t c = cards.back();
    cards.pop_back();
    return c;
  }

  void reset() {
    cards.resize(52);
    std::iota(cards.begin(), cards.end(), 0);
    shuffle();
  }
};

struct PlayerGameContext {
  PlayerInfo* info_ptr; // Non-owning pointer
  bool is_folded = false;
  bool is_ready = false;
  int current_round_bet = 0;
  vec<u8> hand;

  void reset_round() { current_round_bet = 0; }

  void reset_game() {
    is_folded = false;
    is_ready = false;
    current_round_bet = 0;
    hand.clear();
  }
};

} // namespace

class Room final {
private:
  vec<uq_ptr<PlayerInfo>> players;
  vec<uq_ptr<PlayerInfo>>& return_arr;
  std::mutex& return_mtx;
  std::thread room_thread;
  std::atomic<bool> running = false;

  vec<PlayerGameContext> player_contexts;
  Deck deck;
  RoomState state = RoomState::WaitingForStart;
  RoundPhase round_phase = RoundPhase::PreFlop;

  vec<u8> community_cards;
  int pot = 0;
  int current_highest_bet = 0;
  bool bet_placed_this_round = false;

  std::deque<usize> action_queue; // Indices of players
  std::chrono::steady_clock::time_point turn_deadline;
  bool waiting_for_action = false;

  void room_logic() {
    while (running) {
      if (state == RoomState::WaitingForStart) {
        cleanup_disconnected_lobby();
      }

      process_all_players_messages();

      switch (state) {
      case RoomState::WaitingForStart:
        check_start_conditions();
        break;

      case RoomState::Dealing:
        perform_deal();
        break;

      case RoomState::BettingRound:
        process_timeout_logic();
        break;

      case RoomState::RevealCommunity:
        reveal_next_cards();
        break;

      case RoomState::Showdown:
        determine_winner_and_reset();
        break;

      case RoomState::Finished:
        break;
      }

      std::this_thread::sleep_for(std::chrono::milliseconds(50));
    }
  }

  void process_all_players_messages() {
    if (players.size() != player_contexts.size()) {
      sync_contexts();
    }

    for (size_t i = 0; i < players.size(); ++i) {
      auto& player = *players[i];
      player.accept_messages();

      while (auto msg_opt = player.msg_in_reader.read()) {
        dispatch_message(i, msg_opt.value());
        std::cout << std::format("R #{}: Processed player message", id)
                  << std::endl;
      }

      player.send_messages();
    }
  }

  void dispatch_message(size_t player_idx, const Net::MsgStruct& msg) {
    switch (state) {
    case RoomState::WaitingForStart:
      if (msg.code == "RDY1") {
        handle_lobby_ready(player_idx);
      }
      break;

    case RoomState::BettingRound:
      if (!action_queue.empty() && action_queue.front() == player_idx) {
        handle_betting_action(player_idx, msg);
      } else {
        // Optional: Send "Not your turn" or "Wait" warning
        // For now, we silently ignore out-of-turn messages to prevent state
        // corruption
      }
      break;

    default:
      break;
    }
  }

  void handle_lobby_ready(size_t player_idx) {
    if (player_idx < player_contexts.size()) {
      std::cout << "Player: " << player_idx << " Sent Ready" << std::endl;
      player_contexts[player_idx].is_ready = true;
      broadcast("RDOK", Net::Serde::write_sm_int(player_idx));
    }
  }

  void handle_betting_action(size_t player_idx, const Net::MsgStruct& msg) {
    auto& ctx = player_contexts[player_idx];
    bool action_accepted = false;

    if (msg.code == "FOLD") {
      ctx.is_folded = true;
      broadcast("FOLD", std::format("{:02}", player_idx));
      action_accepted = true;
    } else if (msg.code == "CHCK") {
      // Double check needed here
      if (current_highest_bet == ctx.current_round_bet) {
        broadcast("CHCK", std::format("{:02}", player_idx));
        action_accepted = true;
      } else {
        send_error(player_idx, "Cannot check, must Call or Fold");
      }
    } else if (msg.code == "CALL") {
      // Valid only if there is a bet to call
      if (current_highest_bet > ctx.current_round_bet) {
        int amount_to_call = current_highest_bet - ctx.current_round_bet;
        ctx.current_round_bet += amount_to_call;
        pot += amount_to_call;
        broadcast("CALL",
                  std::format("{:02}{:04}", player_idx, current_highest_bet));
        action_accepted = true;
      } else {
        // If bets are equal, a Call is effectively a Check
        broadcast("CHCK", std::format("{:02}", player_idx));
        action_accepted = true;
      }
    } else if (msg.code == "BETT") {
      if (bet_placed_this_round) {
        send_error(player_idx, "Cannot raise. Only Call or Fold allowed.");
      } else {
        int amount = 0;
        try {
          amount = std::stoi(msg.payload.value_or("0"));
        } catch (...) {
        }

        if (amount > 0) {
          current_highest_bet = amount;
          ctx.current_round_bet = amount;
          pot += amount;
          bet_placed_this_round = true;

          broadcast("BETT", std::format("{:02}{:04}", player_idx, amount));

          requeue_active_players(player_idx);
          action_accepted = true;
        }
      }
    }

    if (action_accepted) {
      action_queue.pop_front();
      waiting_for_action = false;
      start_next_turn_or_phase();
    }
  }

  void start_next_turn_or_phase() {
    if (action_queue.empty()) {
      advance_phase();
      return;
    }

    size_t next_idx = action_queue.front();

    if (!players[next_idx]->is_connected()) {
      player_contexts[next_idx].is_folded = true;
      broadcast("FOLD", std::format("{:02}", next_idx));
      action_queue.pop_front();
      start_next_turn_or_phase();
      return;
    }

    waiting_for_action = true;
    turn_deadline = std::chrono::steady_clock::now() +
                    std::chrono::seconds(TURN_TIMEOUT_SECONDS);

    broadcast("TURN", std::format("{:02}", next_idx));
  }

  void requeue_active_players(size_t aggressor_idx) {
    for (size_t i = 0; i < player_contexts.size(); ++i) {
      if (i == aggressor_idx)
        continue;
      if (player_contexts[i].is_folded)
        continue;

      bool already_queued = false;
      for (auto q : action_queue) {
        if (q == i)
          already_queued = true;
      }

      if (!already_queued) {
        action_queue.push_back(i);
      }
    }
  }

  void advance_phase() {
    if (round_phase == RoundPhase::River) {
      state = RoomState::Showdown;
    } else {
      if (round_phase == RoundPhase::PreFlop)
        round_phase = RoundPhase::Flop;
      else if (round_phase == RoundPhase::Flop)
        round_phase = RoundPhase::Turn;
      else if (round_phase == RoundPhase::Turn)
        round_phase = RoundPhase::River;

      state = RoomState::RevealCommunity;
    }
  }

  void check_start_conditions() {
    if (players.size() < 2)
      return;

    if (players.size() != player_contexts.size()) {
      sync_contexts();
    }

    std::cout << std::format("R #{}: Checking start conditions", id)
              << std::endl;

    bool all_ready = true;
    int index = 0;
    for (const auto& ctx : player_contexts) {
      if (!ctx.is_ready) {
        std::cout << "P #" << index << " Is not ready | "
                  << (ctx.is_ready ? "True" : "False") << std::endl;
        all_ready = false;
        break;
      }
    }

    if (all_ready) {
      state = RoomState::Dealing;
      deck.reset();
      pot = 0;
      round_phase = RoundPhase::PreFlop;
      community_cards.clear();
      broadcast("GMST", null); // Game Start
    }
  }

  void perform_deal() {
    for (auto& ctx : player_contexts) {
      ctx.reset_game();
      u8 c1 = deck.draw();
      u8 c2 = deck.draw();
      ctx.hand.push_back(c1);
      ctx.hand.push_back(c2);

      ctx.info_ptr->msg_out_writer.wait_and_insert(
          {"CDTP", std::format("{:02}", c1)});
      ctx.info_ptr->msg_out_writer.wait_and_insert(
          {"CDTP", std::format("{:02}", c2)});

      std::cout << std::format("Sending hand: [{}|{}]", c1, c2) << std::endl;
    }

    prepare_new_betting_round();
  }

  void reveal_next_cards() {
    int count = (round_phase == RoundPhase::Flop) ? 3 : 1;

    for (int i = 0; i < count; ++i) {
      u8 c = deck.draw();
      community_cards.push_back(c);
      broadcast("CRVR", std::format("{:02}", c));
      std::cout << "River card: " << c << std::endl;
    }

    prepare_new_betting_round();
  }

  void prepare_new_betting_round() {
    action_queue.clear();
    current_highest_bet = 0;
    bet_placed_this_round = false;

    for (size_t i = 0; i < player_contexts.size(); ++i) {
      if (!player_contexts[i].is_folded && players[i]->is_connected()) {
        player_contexts[i].reset_round();
        action_queue.push_back(i);
      }
    }

    state = RoomState::BettingRound;
    start_next_turn_or_phase();
  }

  void process_timeout_logic() {
    if (waiting_for_action &&
        std::chrono::steady_clock::now() > turn_deadline) {
      if (!action_queue.empty()) {
        size_t idx = action_queue.front();
        broadcast("TOUT", std::format("{:02}", idx));
        player_contexts[idx].is_folded = true; // Auto-fold on timeout
        action_queue.pop_front();
        waiting_for_action = false;
        start_next_turn_or_phase();
      }
    }
  }

  void determine_winner_and_reset() {
    // Placeholder for win logic (hand evaluation)
    // For now, just reset to lobby
    broadcast("GMDN", null); // Game Done
    for (const auto& p : players) {
      p->flush_messages();
    }
    state = RoomState::WaitingForStart;

    // Move players back to lobby thread logic
    std::lock_guard g{return_mtx};
    for (auto& p : players) {
      return_arr.emplace_back(std::move(p));
    }
    players.clear();
    player_contexts.clear();
    current_players = 0;
  }

  void sync_contexts() {
    player_contexts.clear();
    for (auto& p : players) {
      player_contexts.push_back({p.get()});
    }
  }

  void cleanup_disconnected_lobby() {
    const auto count_removed = std::erase_if(
        players, [](const auto& p) { return !p->is_connected(); });

    if (count_removed > 0) {
      current_players = players.size();
      sync_contexts();
    }
  }

  void broadcast(const str& code, const opt<str>& payload) {
    for (auto& p : players) {
      if (p->is_connected()) {
        p->msg_out_writer.wait_and_insert({code, payload});
      }
    }
  }

  void send_error(size_t player_idx, const str& msg) {
    if (player_idx < players.size()) {
      players[player_idx]->msg_out_writer.wait_and_insert({"ERR_", msg});
    }
  }

public:
  usize id;
  str name;
  int current_players;
  int max_players;

  Room(std::size_t id, str name, vec<uq_ptr<PlayerInfo>>& return_vec,
       std::mutex& return_mutex)
      : id(id), name(name), current_players(0), max_players(ROOM_MAX_PLAYERS),
        return_mtx(return_mutex), return_arr(return_vec) {
    running = true;
    room_thread = std::thread(&Room::room_logic, this);
  }

  ~Room() {
    running = false;
    if (room_thread.joinable())
      room_thread.join();
  }

  Room(const Room&) = delete;
  Room& operator=(const Room&) = delete;

  str to_payload_string() const {
    using namespace Net::Serde;
    return write_bg_int(id) + write_net_str(name) +
           write_sm_int(current_players) + write_sm_int(max_players);
  }

  auto can_player_join() const noexcept -> bool {
    std::cout << std::format("{} | {} ? {}", static_cast<int>(state),
                             current_players, max_players)
              << std::endl;
    return state == RoomState::WaitingForStart && current_players < max_players;
  }

  auto accept_player(uq_ptr<PlayerInfo>&& p) -> void {
    players.emplace_back(std::move(p));
    current_players++;
  }
};
