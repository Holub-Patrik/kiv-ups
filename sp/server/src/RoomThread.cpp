#include "RoomThread.hpp"

namespace GameUtils {
str Card::to_string() const { return std::format("{:02}", value); }

Deck::Deck() { reset(); }

void Deck::shuffle() { std::shuffle(cards.begin(), cards.end(), rng); }

uint8_t Deck::draw() {
  if (cards.empty())
    return 255;
  uint8_t c = cards.back();
  cards.pop_back();
  return c;
}

void Deck::reset() {
  cards.resize(52);
  std::iota(cards.begin(), cards.end(), 0);
  shuffle();
}
} // namespace GameUtils

void PlayerSeat::reset_round() { current_bet = 0; }

void PlayerSeat::reset_game() {
  is_folded = false;
  is_ready = false;
  current_bet = 0;
  hand.clear();
}

bool PlayerSeat::is_active() const {
  return is_occupied && connection != nullptr && connection->is_connected();
}

int RoomContext::count_active_players() const {
  int c = 0;
  for (const auto& s : seats)
    if (s.is_active())
      c++;
  return c;
}

void RoomContext::broadcast(const str& code, const opt<str>& payload) {
  for (auto& seat : seats) {
    if (seat.is_active()) {
      seat.connection->msg_client.writer.wait_and_insert({code, payload});
    }
  }
}

void RoomContext::send_to(int seat_idx, const str& code,
                          const opt<str>& payload) {
  if (seat_idx >= 0 && seat_idx < ROOM_MAX_PLAYERS &&
      seats[seat_idx].is_active()) {
    seats[seat_idx].connection->msg_client.writer.wait_and_insert(
        {code, payload});
  }
}

Room::Room(std::size_t id, str name, vec<uq_ptr<PlayerInfo>>& return_vec,
           std::mutex& return_mutex)
    : id(id), name(name), return_arr(return_vec), return_mtx(return_mutex) {
  current_state = std::make_unique<LobbyState>();
  running = true;
  room_thread = std::thread(&Room::room_logic, this);
}

Room::~Room() {
  running = false;
  if (room_thread.joinable())
    room_thread.join();
}

void Room::accept_player(uq_ptr<PlayerInfo>&& p) {
  std::lock_guard g{incoming_mtx};
  incoming_queue.emplace_back(std::move(p));
}

str Room::to_payload_string() const {
  using namespace Net::Serde;
  int occ = 0;
  for (const auto& s : ctx.seats)
    if (s.is_occupied)
      occ++;
  return write_bg_int(id) + write_net_str(name) + write_sm_int(occ) +
         write_sm_int(ROOM_MAX_PLAYERS);
}

bool Room::can_player_join() const {
  int occ = 0;
  for (const auto& s : ctx.seats)
    if (s.is_occupied)
      occ++;
  return occ < ROOM_MAX_PLAYERS;
}

void Room::room_logic() {
  if (current_state)
    current_state->on_enter(*this, ctx);

  while (running) {
    process_incoming_players();
    process_network_io();

    if (current_state)
      current_state->on_tick(*this, ctx);

    if (pending_transition && next_state_ptr) {
      if (current_state)
        current_state->on_leave(*this, ctx);
      current_state = std::move(next_state_ptr);
      if (current_state)
        current_state->on_enter(*this, ctx);
      pending_transition = false;
    }

    std::this_thread::sleep_for(std::chrono::milliseconds(50));
  }
}

void Room::process_incoming_players() {
  std::lock_guard g{incoming_mtx};
  if (incoming_queue.empty())
    return;

  for (auto& p : incoming_queue) {
    bool seated = false;

    for (auto& seat : ctx.seats) {
      if (seat.is_occupied && seat.nickname == p->nickname &&
          seat.connection == nullptr) {
        std::cout << "Reconnect: " << p->nickname << " to seat." << std::endl;
        seat.connection = p.release();
        seated = true;
        break;
      }
    }

    if (!seated) {
      for (int i = 0; i < ROOM_MAX_PLAYERS; ++i) {
        if (!ctx.seats[i].is_occupied) {
          ctx.seats[i].is_occupied = true;
          ctx.seats[i].nickname = p->nickname;
          ctx.seats[i].connection = p.release();
          ctx.seats[i].chips = 1000;
          seated = true;
          std::cout << "New Player: " << ctx.seats[i].nickname << " at seat "
                    << i << std::endl;
          break;
        }
      }
    }

    if (!seated) {
      std::lock_guard lg{return_mtx};
      return_arr.push_back(std::move(p));
    }
  }
  incoming_queue.clear();
}

void Room::process_network_io() {
  for (int i = 0; i < ROOM_MAX_PLAYERS; ++i) {
    auto& seat = ctx.seats[i];

    if (seat.is_occupied && seat.connection && seat.connection->disconnected) {
      std::cout << "Player " << seat.nickname << " disconnected (seat " << i
                << ")" << std::endl;
      delete seat.connection;
      seat.connection = nullptr;
      seat.is_ready = false;
    }

    if (seat.is_active()) {
      auto* p = seat.connection;
      p->accept_messages();

      while (const auto& msg_opt = p->msg_client.reader.read()) {
        const auto& msg = msg_opt.value();

        // Global "Leave Room" Command
        if (msg.code == "GMLV") {
          std::cout << "Player leaving room entirely." << std::endl;
          std::lock_guard lg{return_mtx};
          return_arr.emplace_back(std::unique_ptr<PlayerInfo>(p));
          seat.connection = nullptr;
          seat.is_occupied = false;
          seat.nickname = "";
          continue;
        }

        if (current_state) {
          current_state->on_message(*this, ctx, i, msg);
        }
      }

      if (seat.connection) {
        seat.connection->send_messages();
      }
    }
  }
}

void LobbyState::on_enter(Room& room, RoomContext& ctx) {
  std::cout << "State: Enter Lobby" << std::endl;
  ctx.broadcast("GMLB", null);

  for (int i = 0; i < ROOM_MAX_PLAYERS; ++i) {
    if (ctx.seats[i].is_occupied && ctx.seats[i].connection == nullptr) {
      std::cout << "Cleanup: Removing disconnected player from seat " << i
                << std::endl;
      ctx.seats[i] = PlayerSeat{};
    } else {
      ctx.seats[i].reset_game();
    }
  }

  ctx.pot = 0;
  ctx.community_cards.clear();
  ctx.deck.reset();
}

void LobbyState::on_leave(Room& room, RoomContext& ctx) {
  std::cout << "State: Leave Lobby" << std::endl;
}

void LobbyState::on_tick(Room& room, RoomContext& ctx) {
  for (int i = 0; i < ROOM_MAX_PLAYERS; ++i) {
    if (ctx.seats[i].is_occupied && ctx.seats[i].connection == nullptr) {
      std::cout << "Lobby Cleanup: Removing disconnected player from seat " << i
                << std::endl;
      ctx.seats[i] = PlayerSeat{};
    }
  }

  int ready_count = 0;
  int player_count = 0;
  for (const auto& s : ctx.seats) {
    if (s.is_active()) {
      player_count++;
      if (s.is_ready)
        ready_count++;
    }
  }
  if (player_count >= 2 && ready_count == player_count) {
    room.transition_to<DealingState>();
  }
}

void LobbyState::on_message(Room& room, RoomContext& ctx, int seat_idx,
                            const Net::MsgStruct& msg) {
  if (msg.code == "RDY1") {
    ctx.seats[seat_idx].is_ready = true;
    ctx.broadcast("RDOK", Net::Serde::write_sm_int(seat_idx));
  }
}

void DealingState::on_enter(Room& room, RoomContext& ctx) {
  std::cout << "State: Enter Dealing" << std::endl;
  ctx.broadcast("GMST", null);

  ctx.round_phase = RoundPhase::PreFlop;

  for (int i = 0; i < ROOM_MAX_PLAYERS; ++i) {
    if (ctx.seats[i].is_active() && ctx.seats[i].is_ready) {
      u8 c1 = ctx.deck.draw();
      u8 c2 = ctx.deck.draw();
      ctx.seats[i].hand = {c1, c2};
      ctx.send_to(i, "CDTP", std::format("{:02}", c1));
      ctx.send_to(i, "CDTP", std::format("{:02}", c2));
    }
  }
}

void DealingState::on_leave(Room& room, RoomContext& ctx) {}

void DealingState::on_tick(Room& room, RoomContext& ctx) {
  room.transition_to<BettingState>();
}

void DealingState::on_message(Room& room, RoomContext& ctx, int seat_idx,
                              const Net::MsgStruct& msg) {}

void CommunityCardState::on_enter(Room& room, RoomContext& ctx) {
  std::cout << "State: Revealing Cards" << std::endl;

  int cards_to_draw = 0;
  if (ctx.round_phase == RoundPhase::PreFlop) {
    ctx.round_phase = RoundPhase::Flop;
    cards_to_draw = 3;
  } else if (ctx.round_phase == RoundPhase::Flop) {
    ctx.round_phase = RoundPhase::Turn;
    cards_to_draw = 1;
  } else if (ctx.round_phase == RoundPhase::Turn) {
    ctx.round_phase = RoundPhase::River;
    cards_to_draw = 1;
  }

  for (int i = 0; i < cards_to_draw; ++i) {
    u8 c = ctx.deck.draw();
    ctx.community_cards.push_back(c);
    ctx.broadcast("CRVR", std::format("{:02}", c));
    std::cout << "Revealed Card: " << (int)c << std::endl;
  }
}

void CommunityCardState::on_leave(Room& room, RoomContext& ctx) {}

void CommunityCardState::on_tick(Room& room, RoomContext& ctx) {
  room.transition_to<BettingState>();
}

void CommunityCardState::on_message(Room& room, RoomContext& ctx, int seat_idx,
                                    const Net::MsgStruct& msg) {}

void BettingState::on_enter(Room& room, RoomContext& ctx) {
  std::cout << "State: Enter Betting" << std::endl;

  action_queue.clear();
  current_high_bet = 0;
  has_bet_occurred = false;

  for (auto& s : ctx.seats) {
    s.current_bet = 0;
  }

  int start_idx = (ctx.dealer_idx + 1) % ROOM_MAX_PLAYERS;

  for (int i = 0; i < ROOM_MAX_PLAYERS; ++i) {
    int idx = (start_idx + i) % ROOM_MAX_PLAYERS;
    if (ctx.seats[idx].is_active() && !ctx.seats[idx].is_folded &&
        ctx.seats[idx].is_ready) {
      action_queue.push_back(idx);
    }
  }

  if (action_queue.empty()) {
    current_actor = -1;
  } else {
    start_next_turn(ctx);
  }
}

void BettingState::start_next_turn(RoomContext& ctx) {
  if (action_queue.empty()) {
    current_actor = -1;
    return;
  }

  current_actor = action_queue.front();
  action_queue.pop_front();

  if (!ctx.seats[current_actor].is_active() ||
      ctx.seats[current_actor].is_folded) {
    start_next_turn(ctx);
    return;
  }

  std::cout << "Turn: Seat " << current_actor << std::endl;
  ctx.broadcast("TURN", std::format("{:02}", current_actor));
}

void BettingState::requeue_others(RoomContext& ctx, int aggressor_idx) {
  action_queue.clear();

  int start_idx = (aggressor_idx + 1) % ROOM_MAX_PLAYERS;

  for (int i = 0; i < ROOM_MAX_PLAYERS; ++i) {
    int idx = (start_idx + i) % ROOM_MAX_PLAYERS;
    if (idx == aggressor_idx)
      continue; // Don't queue self

    if (ctx.seats[idx].is_active() && !ctx.seats[idx].is_folded) {
      action_queue.push_back(idx);
    }
  }
}

void BettingState::on_leave(Room& room, RoomContext& ctx) {
  std::cout << "State: Leave Betting" << std::endl;
}

void BettingState::on_tick(Room& room, RoomContext& ctx) {
  if (current_actor == -1) {
    if (ctx.round_phase == RoundPhase::River) {
      room.transition_to<ShowdownState>();
    } else {
      room.transition_to<CommunityCardState>();
    }
  }
}

void BettingState::on_message(Room& room, RoomContext& ctx, int seat_idx,
                              const Net::MsgStruct& msg) {
  if (seat_idx != current_actor) {
    ctx.send_to(seat_idx, "ERR_", "Not your turn");
    return;
  }

  auto& seat = ctx.seats[seat_idx];
  bool turn_completed = false;

  if (msg.code == "FOLD") {
    seat.is_folded = true;
    ctx.broadcast("FOLD", std::format("{:02}", seat_idx));
    turn_completed = true;
  }

  else if (msg.code == "CHCK") {
    if (current_high_bet > seat.current_bet) {
      ctx.send_to(seat_idx, "ERR_", "Cannot Check, must Call or Fold");
    } else {
      ctx.broadcast("CHCK", std::format("{:02}", seat_idx));
      turn_completed = true;
    }
  }

  else if (msg.code == "BETT") {
    if (has_bet_occurred) {
      ctx.send_to(seat_idx, "ERR_", "Cannot Raise (limit 1 bet/rnd)");
    } else {
      // Let's assume fixed simple logic or parse payload
      int amount = 100; // Default or parse from msg.payload

      if (msg.payload) {
        try {
          amount = std::stoi(msg.payload.value());
        } catch (...) {
        }
      }

      if (seat.chips < amount) {
        ctx.send_to(seat_idx, "ERR_", "Not enough chips");
      } else {
        current_high_bet = amount;
        seat.current_bet = amount;
        seat.chips -= amount;
        ctx.pot += amount;
        has_bet_occurred = true;

        ctx.broadcast("BETT", std::format("{:02}{:04}", seat_idx, amount));

        requeue_others(ctx, seat_idx);
        turn_completed = true;
      }
    }
  }

  else if (msg.code == "CALL") {
    if (!has_bet_occurred && current_high_bet == 0) {
      ctx.broadcast("CHCK", std::format("{:02}", seat_idx));
      turn_completed = true;
    } else {
      int to_call = current_high_bet - seat.current_bet;
      if (seat.chips < to_call) {
        to_call = seat.chips;
      }

      seat.chips -= to_call;
      seat.current_bet += to_call;
      ctx.pot += to_call;

      ctx.broadcast("CALL",
                    std::format("{:02}{:04}", seat_idx, seat.current_bet));
      turn_completed = true;
    }
  }

  if (turn_completed) {
    start_next_turn(ctx);
  }
}

void ShowdownState::on_enter(Room& room, RoomContext& ctx) {
  std::cout << "State: Enter Showdown" << std::endl;
  ctx.broadcast("GMDN", null);
}

void ShowdownState::on_leave(Room& room, RoomContext& ctx) {}

void ShowdownState::on_tick(Room& room, RoomContext& ctx) {
  room.transition_to<LobbyState>();
}

void ShowdownState::on_message(Room& room, RoomContext& ctx, int seat_idx,
                               const Net::MsgStruct& msg) {}
