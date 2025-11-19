#pragma once

#include "Babel.hpp"

#include "MessageSerde.hpp"
#include "PlayerInfo.hpp"

#include <format>
#include <memory>
#include <mutex>
#include <random>
#include <vector>

constexpr usize ROOM_MAX_PLAYERS = 4;
constexpr int TURN_TIMEOUT_SECONDS = 3000;

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
  Deck() {
    cards.resize(52);
    std::iota(cards.begin(), cards.end(), 0);
  }

  void shuffle() { std::shuffle(cards.begin(), cards.end(), rng); }

  // Returns 255 if empty (shouldn't happen in simple poker)
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
  PlayerInfo* info_ptr; // Non-owning pointer to the actual connection
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
  // temporary vector that holds the pointers
  // It is used so that MainThread stops handling these players
  vec<uq_ptr<PlayerInfo>> players;
  // return information
  vec<uq_ptr<PlayerInfo>>& return_arr;
  std::mutex& return_mtx;

  vec<PlayerGameContext> player_contexts;

  std::thread room_thread;
  std::atomic<bool> running = false;

  Deck deck;
  RoomState state = RoomState::WaitingForStart;
  RoundPhase round_phase = RoundPhase::PreFlop;

  vec<u8> community_cards;
  int pot = 0;
  int current_highest_bet = 0;

  std::deque<usize> action_queue; // Stores indices of players who need to act
  std::chrono::steady_clock::time_point turn_deadline;
  bool waiting_for_action = false;

  void broadcast(const str& code, const opt<str>& payload) {
    for (auto& p : players) {
      if (p->is_connected()) {
        p->msg_out_writer.wait_and_insert({code, payload});
      }
    }
  }

  void room_logic() {
    while (running) {

      if (state == RoomState::WaitingForStart) {
        cleanup_disconnected_lobby();
      }

      process_incoming_messages();

      switch (state) {
      case RoomState::WaitingForStart:
        check_start_conditions();
        break;

      case RoomState::Dealing:
        perform_deal();
        break;

      case RoomState::BettingRound:
        process_betting_logic();
        break;

      case RoomState::RevealCommunity:
        reveal_next_cards();
        break;

      case RoomState::Showdown:
        // TODO: Implement Scoring logic
        reset_to_lobby();
        break;

      case RoomState::Finished:
        break;
      }

      std::this_thread::sleep_for(std::chrono::milliseconds(50));
    }
  }

  void cleanup_disconnected_lobby() {
    std::erase_if(players, [](const auto& p) { return !p->is_connected(); });
    current_players = players.size();
  }

  void process_incoming_messages() {
    for (size_t i = 0; i < players.size(); ++i) {
      auto& player = *players[i];

      player.accept_messages();

      while (auto msg_opt = player.msg_in_reader.read()) {
        auto msg = msg_opt.value();
        handle_message(i, msg);
      }

      player.send_messages();
    }
  }

  void handle_message(size_t player_idx, const Net::MsgStruct& msg) {
    if (state == RoomState::WaitingForStart) {
      if (msg.code == "RDY1") {
        // Initialize context if not exists (lazy init)
        ensure_contexts_synced();
        if (player_idx < player_contexts.size()) {
          player_contexts[player_idx].is_ready = true;
          std::cout << "Player " << player_idx << " is READY.\n";
        }
      }
    } else if (state == RoomState::BettingRound) {
      if (waiting_for_action && !action_queue.empty() &&
          action_queue.front() == player_idx) {
        handle_game_move(player_idx, msg);
      }
    }
  }

  void ensure_contexts_synced() {
    if (player_contexts.size() != players.size()) {
      player_contexts.clear();
      for (auto& p : players) {
        player_contexts.push_back({p.get()});
      }
    }
  }

  void check_start_conditions() {
    if (players.size() < 2)
      return; // Need at least 2 to play

    ensure_contexts_synced();

    bool all_ready = true;
    for (const auto& ctx : player_contexts) {
      if (!ctx.is_ready) {
        all_ready = false;
        break;
      }
    }

    if (all_ready) {
      std::cout << "All players ready. Locking room and starting game.\n";
      state = RoomState::Dealing;
      deck.reset();
      pot = 0;
      round_phase = RoundPhase::PreFlop;
      community_cards.clear();
    }
  }

  void perform_deal() {
    std::cout << "Dealing cards...\n";

    // Deal 2 cards to each player
    for (auto& ctx : player_contexts) {
      ctx.reset_game();
      // Card 1
      uint8_t c1 = deck.draw();
      ctx.hand.push_back(c1);
      ctx.info_ptr->msg_out_writer.wait_and_insert(
          {"CDTP", std::format("{:02}", c1)});

      // Card 2
      uint8_t c2 = deck.draw();
      ctx.hand.push_back(c2);
      ctx.info_ptr->msg_out_writer.wait_and_insert(
          {"CDTP", std::format("{:02}", c2)});
    }

    // Setup Action Queue for Pre-Flop (Round 1)
    prepare_betting_round();
  }

  void prepare_betting_round() {
    action_queue.clear();
    current_highest_bet = 0;

    // Add all active (non-folded) players to the queue
    for (size_t i = 0; i < player_contexts.size(); ++i) {
      if (!player_contexts[i].is_folded && players[i]->is_connected()) {
        player_contexts[i].reset_round();
        action_queue.push_back(i);
      }
    }

    state = RoomState::BettingRound;
    start_next_turn();
  }

  void start_next_turn() {
    if (action_queue.empty()) {
      // Round Complete
      advance_phase();
      return;
    }

    size_t p_idx = action_queue.front();

    // Check if player disconnected before their turn
    if (!players[p_idx]->is_connected()) {
      player_contexts[p_idx].is_folded = true;
      broadcast("FOLD", std::to_string(p_idx)); // Notify others
      action_queue.pop_front();
      start_next_turn();
      return;
    }

    waiting_for_action = true;

    // Set Timeout
    turn_deadline = std::chrono::steady_clock::now() +
                    std::chrono::seconds(TURN_TIMEOUT_SECONDS);

    // Notify players whose turn it is (optional: or just the player)
    // Protocol wasn't specific, but usually good to tell everyone "P1 turn"
    broadcast("TURN", std::format("{:02}", p_idx));
  }

  void handle_game_move(size_t player_idx, const Net::MsgStruct& msg) {
    auto& ctx = player_contexts[player_idx];
    bool valid_move = false;

    // Parse action: "FOLD", "CHCK", "BETT"
    if (msg.code == "FOLD") {
      ctx.is_folded = true;
      broadcast("FOLD", std::format("{:02}", player_idx));
      valid_move = true;
    } else if (msg.code == "CHCK") {
      // Can only check if current bet matches highest
      if (ctx.current_round_bet == current_highest_bet) {
        broadcast("CHCK", std::format("{:02}", player_idx));
        valid_move = true;
      }
    } else if (msg.code == "CALL") {

    } else if (msg.code == "BETT") {
      // Logic: Bets cannot be raised OVER.
      // Meaning if High is 10, and I have 0, I bet 10 to CALL.
      // If High is 0, I can bet X.

      int amount = 0;
      try {
        if (msg.payload)
          amount = std::stoi(msg.payload.value());
      } catch (...) {
        // somehow handle incorrect message here
      }

      if (amount > 0) {
        // Logic: Update Pot
        int diff = amount - ctx.current_round_bet;
        pot += diff;
        ctx.current_round_bet = amount;

        broadcast("BETT", std::format("{:02}{:04}", player_idx,
                                      amount)); // Simply echoing logic

        // If this raises the highest bet
        if (amount > current_highest_bet) {
          current_highest_bet = amount;
          requeue_players_for_bet(player_idx);
        }
        valid_move = true;
      }
    }

    if (valid_move) {
      waiting_for_action = false;
      action_queue.pop_front(); // Remove current actor
      start_next_turn();        // Process next in queue
    }
  }

  void requeue_players_for_bet(size_t aggressor_idx) {
    for (size_t i = 0; i < player_contexts.size(); ++i) {
      if (i == aggressor_idx)
        continue;
      if (player_contexts[i].is_folded)
        continue;

      bool in_q = false;
      for (auto q_idx : action_queue) {
        if (q_idx == i)
          in_q = true;
      }

      if (!in_q) {
        action_queue.push_back(i);
      }
    }
  }

  void process_betting_logic() {
    if (!waiting_for_action)
      return;

    auto now = std::chrono::steady_clock::now();
    if (now > turn_deadline) {
      if (!action_queue.empty()) {
        size_t p_idx = action_queue.front();
        std::cout << "Player " << p_idx << " timed out.\n";

        // TimeOUT + who
        broadcast("TOUT", std::format("{:02}", p_idx));

        player_contexts[p_idx].is_folded = true;

        waiting_for_action = false;
        action_queue.pop_front();
        start_next_turn();
      }
    }
  }

  void advance_phase() {
    if (round_phase == RoundPhase::PreFlop) {
      round_phase = RoundPhase::Flop;
      state = RoomState::RevealCommunity;
    } else if (round_phase == RoundPhase::Flop) {
      round_phase = RoundPhase::Turn;
      state = RoomState::RevealCommunity;
    } else if (round_phase == RoundPhase::Turn) {
      round_phase = RoundPhase::River;
      state = RoomState::RevealCommunity;
    } else if (round_phase == RoundPhase::River) {
      state = RoomState::Showdown;
    }
  }

  void reveal_next_cards() {
    int cards_to_reveal = 0;
    if (round_phase == RoundPhase::Flop)
      cards_to_reveal = 3;
    else
      cards_to_reveal = 1;

    for (int i = 0; i < cards_to_reveal; ++i) {
      uint8_t c = deck.draw();
      community_cards.push_back(c);
      // Card RiVeR
      broadcast("CRVR", std::format("{:02}", c));
    }

    prepare_betting_round();
  }

  void reset_to_lobby() {
    state = RoomState::WaitingForStart;
    // Game Done
    broadcast("GMDN", null);
    {
      std::lock_guard g{return_mtx};
      for (usize i = 0; i < players.size(); i++) {
        // give back players to MainThread
        return_arr.emplace_back(std::move(players[i]));
      }
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
    room_thread.join();
  }

  // delete copy semantics
  Room(const Room& other) = delete;
  Room& operator=(const Room& other) = delete;

  // delete move semantics
  Room(Room&&) = delete;
  Room& operator=(Room&&) = delete;

  str to_payload_string() const {
    using namespace Net::Serde;
    return write_bg_int(id) + write_net_str(name) +
           write_sm_int(current_players) + write_sm_int(max_players);
  }

  auto can_player_join() const noexcept -> bool {
    return state == RoomState::WaitingForStart && current_players < max_players;
  }

  auto accept_player(uq_ptr<PlayerInfo>&& p) -> void {
    players.emplace_back(std::move(p));
    current_players++;
  }
};
