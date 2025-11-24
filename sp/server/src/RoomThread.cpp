#include "RoomThread.hpp"
#include "Babel.hpp"

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

void RoomContext::broadcast(const str_v& code, const opt<str>& payload) {
  for (auto& seat : seats) {
    if (seat.is_active()) {
      seat.connection->msg_client.writer.wait_and_insert({str{code}, payload});
    }
  }
}

void RoomContext::send_to(int seat_idx, const str_v& code,
                          const opt<str>& payload) {
  if (seat_idx >= 0 && seat_idx < ROOM_MAX_PLAYERS &&
      seats[seat_idx].is_active()) {
    seats[seat_idx].connection->msg_client.writer.wait_and_insert(
        {str{code}, payload});
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

    // Try reconnect (player already has a seat but disconnected)
    for (auto& seat : ctx.seats) {
      if (seat.is_occupied && seat.nickname == p->nickname &&
          seat.connection == nullptr) {
        std::cout << std::format("Reconnecting {} to seat\n", p->nickname);
        seat.connection = p.release();
        seat.connection->state = PlayerState::InRoom;
        seated = true;
        break;
      }
    }

    // Assign new seat
    if (!seated) {
      for (int i = 0; i < ROOM_MAX_PLAYERS; ++i) {
        if (!ctx.seats[i].is_occupied) {
          ctx.seats[i].is_occupied = true;
          ctx.seats[i].nickname = p->nickname;
          ctx.seats[i].connection = p.release();
          ctx.seats[i].chips = 1000;
          ctx.seats[i].connection->state = PlayerState::InRoom;
          seated = true;
          std::cout << std::format("New player {} at seat {}\n",
                                   ctx.seats[i].nickname, i);
          break;
        }
      }
    }

    // If no seat available, return to main list
    if (!seated) {
      std::cout << std::format("No seat for {}, returning to main list\n",
                               p->nickname);
      std::lock_guard lg{return_mtx};
      return_arr.push_back(std::move(p));
    }
  }
  incoming_queue.clear();
}

static bool is_valid_room_code(const str_v& code) {
  static const arr<str_v, 20> valid_codes = {
      Msg::RDY1, Msg::GMLV, Msg::CHCK, Msg::FOLD, Msg::CALL, Msg::BETT,
      Msg::CDOK, Msg::CDFL, Msg::STOK, Msg::STFL, Msg::DNOK, Msg::DNFL};
  return std::find(valid_codes.begin(), valid_codes.end(), code) !=
         valid_codes.end();
}

void Room::process_network_io() {
  for (int i = 0; i < ROOM_MAX_PLAYERS; ++i) {
    auto& seat = ctx.seats[i];

    // Handle disconnections
    if (seat.is_occupied && seat.connection && seat.connection->disconnected) {
      std::cout << std::format("Player {} disconnected (seat {})\n",
                               seat.nickname, i);
      delete seat.connection;
      seat.connection = nullptr;
      seat.is_ready = false;
      continue;
    }

    // Process messages from active players
    if (seat.is_active()) {
      auto* p = seat.connection;
      p->accept_messages();

      while (const auto& msg_opt = p->msg_client.reader.read()) {
        const auto& msg = msg_opt.value();

        // Global leave command
        if (msg.code == Msg::GMLV) {
          std::cout << std::format("Player {} leaving room\n", seat.nickname);
          std::lock_guard lg{return_mtx};
          return_arr.emplace_back(std::unique_ptr<PlayerInfo>(p));
          seat.connection = nullptr;
          seat.is_occupied = false;
          seat.nickname = "";
          continue;
        }

        // Validate message code
        if (!is_valid_room_code(msg.code)) {
          std::cerr << std::format(
              "Unknown room message {} from {}, disconnecting\n", msg.code,
              seat.nickname);
          p->disconnected = true;
          break;
        }

        // Route to current state
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

  for (int i = 0; i < ROOM_MAX_PLAYERS; ++i) {
    if (ctx.seats[i].is_occupied && ctx.seats[i].connection == nullptr) {
      std::cout << std::format(
          "Lobby cleanup: Removing disconnected player from seat {}\n", i);
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
      std::cout << std::format(
          "Lobby cleanup: Removing disconnected player from seat {}\n", i);
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

  // Start game if all active players are ready (min 2 players)
  if (player_count >= 2 && ready_count == player_count) {
    std::cout << std::format("All {} players ready, starting game\n",
                             player_count);
    room.transition_to<DealingState>();
  }
}

void LobbyState::on_message(Room& room, RoomContext& ctx, int seat_idx,
                            const Net::MsgStruct& msg) {
  if (msg.code == Msg::RDY1) {
    if (ctx.seats[seat_idx].is_active()) {
      ctx.seats[seat_idx].is_ready = true;
      ctx.broadcast(Msg::PRDY, Net::Serde::write_sm_int(seat_idx));
      std::cout << std::format("Player {} ready\n",
                               ctx.seats[seat_idx].nickname);
    }
  }
}

void DealingState::on_enter(Room& room, RoomContext& ctx) {
  std::cout << "State: Enter Dealing" << std::endl;
  ctx.broadcast(Msg::GMST, null); // Game starting

  ctx.round_phase = RoundPhase::PreFlop;

  for (int i = 0; i < ROOM_MAX_PLAYERS; ++i) {
    if (ctx.seats[i].is_active() && ctx.seats[i].is_ready) {
      u8 c1 = ctx.deck.draw();
      u8 c2 = ctx.deck.draw();
      ctx.seats[i].hand = {c1, c2};
      ctx.send_to(i, Msg::CDTP, std::format("{:02}{:02}", c1, c2));
      std::cout << std::format("Dealt cards to {}: {} {}\n",
                               ctx.seats[i].nickname, (int)c1, (int)c2);
    }
  }
}

void DealingState::on_leave(Room& room, RoomContext& ctx) {}

void DealingState::on_tick(Room& room, RoomContext& ctx) {
  room.transition_to<BettingState>();
}

void DealingState::on_message(Room& room, RoomContext& ctx, int seat_idx,
                              const Net::MsgStruct& msg) {
  std::cerr << std::format("Unexpected message {} in Dealing state\n",
                           msg.code);
}

void CommunityCardState::on_enter(Room& room, RoomContext& ctx) {
  std::cout << "State: Revealing Community Cards" << std::endl;

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
    ctx.broadcast(Msg::CRVR, std::format("{:02}", c));
    std::cout << std::format("Revealed community card: {:d}\n", c);
  }
}

void CommunityCardState::on_leave(Room& room, RoomContext& ctx) {}

void CommunityCardState::on_tick(Room& room, RoomContext& ctx) {
  room.transition_to<BettingState>();
}

void CommunityCardState::on_message(Room& room, RoomContext& ctx, int seat_idx,
                                    const Net::MsgStruct& msg) {
  // No messages expected during card reveal
  std::cerr << std::format("Unexpected message {} in CommunityCard state\n",
                           msg.code);
}

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

  std::cout << std::format("Turn: Seat {} ({})\n", current_actor,
                           ctx.seats[current_actor].nickname);
  ctx.broadcast(Msg::PTRN, Net::Serde::write_sm_int(current_actor));
}

void BettingState::requeue_others(RoomContext& ctx, int aggressor_idx) {
  action_queue.clear();
  int start_idx = (aggressor_idx + 1) % ROOM_MAX_PLAYERS;

  for (int i = 0; i < ROOM_MAX_PLAYERS; ++i) {
    int idx = (start_idx + i) % ROOM_MAX_PLAYERS;
    if (idx == aggressor_idx)
      continue;

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
    ctx.send_to(seat_idx, Msg::NYET, null);
    return;
  }

  auto& seat = ctx.seats[seat_idx];
  bool turn_completed = false;

  if (msg.code == Msg::FOLD) {
    seat.is_folded = true;
    ctx.send_to(seat_idx, Msg::ACOK, Net::Serde::write_sm_int(seat_idx));
    turn_completed = true;
    std::cout << std::format("Player {} folded\n", seat.nickname);
  } else if (msg.code == Msg::CHCK) {
    if (current_high_bet > seat.current_bet) {
      ctx.send_to(seat_idx, Msg::ACFL, "Cannot check, must call");
    } else {
      ctx.send_to(seat_idx, Msg::ACOK, Net::Serde::write_sm_int(seat_idx));
      turn_completed = true;
    }
  } else if (msg.code == Msg::BETT) {
    if (has_bet_occurred) {
      ctx.send_to(seat_idx, Msg::ACFL, "Cannot raise (limit 1 bet/round)");
    } else if (!msg.payload) {
      ctx.send_to(seat_idx, Msg::ACFL, "Bet amount required");
    } else {
      int amount = 100;
      if (msg.payload) {
        try {
          amount = std::stoi(msg.payload.value());
        } catch (...) {
        }
      }

      current_high_bet = amount;
      seat.current_bet = amount;
      seat.chips -= amount;
      ctx.pot += amount;
      has_bet_occurred = true;

      ctx.send_to(seat_idx, Msg::ACOK,
                  std::format("{:02}{:04}", seat_idx, amount));
      requeue_others(ctx, seat_idx);
      turn_completed = true;
      std::cout << std::format("Player {} bets {}\n", seat.nickname, amount);
    }
  } else if (msg.code == Msg::CALL) {
    int to_call = current_high_bet - seat.current_bet;
    if (to_call > 0) {
      seat.chips -= to_call;
      seat.current_bet += to_call;
      ctx.pot += to_call;
    }
    ctx.send_to(seat_idx, Msg::ACOK, Net::Serde::write_sm_int(seat_idx));
    turn_completed = true;
    std::cout << std::format("Player {} calls {}\n", seat.nickname, to_call);
  }

  if (turn_completed) {
    start_next_turn(ctx);
  }
}

void ShowdownState::on_enter(Room& room, RoomContext& ctx) {
  std::cout << "State: Enter Showdown" << std::endl;

  str payload = Net::Serde::write_sm_int(ctx.count_active_players());
  for (int i = 0; i < ROOM_MAX_PLAYERS; ++i) {
    if (ctx.seats[i].is_active() && !ctx.seats[i].is_folded) {
      payload += Net::Serde::write_sm_int(i);
      payload +=
          std::format("{:02}{:02}", ctx.seats[i].hand[0], ctx.seats[i].hand[1]);
    }
  }

  ctx.broadcast(Msg::SDWN, payload);
}

void ShowdownState::on_leave(Room& room, RoomContext& ctx) {}

void ShowdownState::on_tick(Room& room, RoomContext& ctx) {
  room.transition_to<LobbyState>();
}

void ShowdownState::on_message(Room& room, RoomContext& ctx, int seat_idx,
                               const Net::MsgStruct& msg) {
  std::cerr << std::format("Unexpected message {} in Showdown state\n",
                           msg.code);
}
