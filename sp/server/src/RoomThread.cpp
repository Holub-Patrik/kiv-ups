#include "RoomThread.hpp"
#include "Babel.hpp"
#include "MessageSerde.hpp"
#include "PlayerInfo.hpp"
#include "PokerScoring.hpp"

#include <algorithm>
#include <sstream>

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

static const Scoring scoring{};

void PlayerSeat::reset_round() { round_bet = 0; }

void PlayerSeat::reset_game() {
  is_folded = false;
  is_ready = false;
  showdowm_okay = false;
  round_bet = 0;
  total_bet = 0;
  hand.clear();
}

bool PlayerSeat::is_active() const {
  return is_occupied && connection != nullptr && connection->is_connected();
}

int RoomContext::count_active_players() const {
  int c = 0;
  for (const auto& s : seats) {
    if (s.is_active()) {
      c++;
    }
  }
  return c;
}

int RoomContext::count_occupied_seats() const {
  int c = 0;
  for (const auto& s : seats) {
    if (s.is_occupied) {
      c++;
    }
  }
  return c;
}

void RoomContext::broadcast(const str_v& code, const opt<str>& payload) {
  for (auto& seat : seats) {
    if (seat.is_active()) {
      seat.connection->send_message({str{code}, payload});
    }
  }
}

void RoomContext::broadcast_ex(const int seat_idx, const str_v& code,
                               const opt<str>& payload) {
  for (usize i = 0; i < seats.size(); i++) {
    if (i == seat_idx) {
      continue;
    }

    auto& seat = seats[i];

    if (seat.is_active()) {
      seat.connection->send_message({str{code}, payload});
    }
  }
}

void RoomContext::send_to(int seat_idx, const str_v& code,
                          const opt<str>& payload) {
  if (seat_idx >= 0 && seat_idx < seats.size() && seats[seat_idx].is_active()) {
    seats[seat_idx].connection->send_message({str{code}, payload});
  }
}

static str ser_player(const int& seat_idx, const RoomContext& ctx) {
  std::stringstream ss;
  using namespace Net::Serde;
  const auto& seat = ctx.seats[seat_idx];
  ss << write_net_str(seat.nickname);
  ss << write_var_int(seat.chips);
  ss << write_sm_int(seat.is_folded ? 1 : 0);
  ss << write_sm_int(seat.is_ready ? 1 : 0);
  ss << write_sm_int(seat_idx == ctx.current_actor ? 1 : 0);
  std::cout << std::format("Senging turn {}: {}", seat.nickname,
                           seat_idx == ctx.current_actor ? "True" : "False");
  ss << write_sm_int(static_cast<u8>(seat.action_taken));
  ss << write_var_int(seat.action_amount);
  ss << write_var_int(seat.round_bet);
  ss << write_var_int(seat.total_bet);
  return ss.str();
}

str RoomContext::serialize() const {
  using namespace Net::Serde;
  std::stringstream ss{};

  ss << write_var_int(pot);
  ss << write_var_int(current_high_bet);
  ss << write_sm_int(community_cards.size());

  for (const auto& card : community_cards) {
    ss << write_sm_int(card);
  }

  ss << write_sm_int(count_occupied_seats());
  for (usize i = 0; i < seats.size(); i++) {
    if (!seats[i].is_occupied) {
      continue;
    }
    ss << ser_player(i, *this);
  }

  return ss.str();
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
  {
    std::lock_guard g{incoming_mtx};
    incoming_queue.emplace_back(std::move(p));
  }
}

void Room::reconnect_player(uq_ptr<PlayerInfo>&& p) {
  accept_player(std::move(p));
}

str Room::serialize() const {
  using namespace Net::Serde;

  int occ = 0;
  for (const auto& s : ctx.seats)
    if (s.is_occupied)
      occ++;

  return write_bg_int(id) + write_net_str(name) + write_sm_int(occ) +
         write_sm_int(ROOM_MAX_PLAYERS);
}

str Room::serialize_up() {
  using namespace Net::Serde;

  std::lock_guard g{up_mtx};
  std::stringstream ss;

  ss << write_bg_int(updates.size());
  for (const auto& update : updates) {
    ss << update;
  }
  updates.clear();

  return ss.str();
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
  // Here we should send general room information to players
  // If we are reconnecting players, extended information should be sent, since
  // he doesn't know his own state
  std::lock_guard g{incoming_mtx};
  if (incoming_queue.empty())
    return;

  for (auto& p : incoming_queue) {
    bool seated = false;

    for (int seat_idx = 0; seat_idx < ctx.seats.size(); seat_idx++) {
      auto& seat = ctx.seats[seat_idx];
      if (seat.is_occupied && seat.nickname == p->nickname &&
          seat.connection == nullptr) {
        std::cout << std::format("Reconnecting {} to seat\n", p->nickname);
        seat.connection = std::move(p);
        seat.connection->state = PlayerState::InRoom;
        seated = true;

        // In this case chips have to be sent back to player
        // No argument means that the player themselves will be included in the
        // message
        seat.connection->send_message(Net::MsgStruct{"RMST", ctx.serialize()});
        ctx.broadcast_ex(seat_idx, Msg::PJIN, ser_player(seat_idx, ctx));
        break;
      }
    }

    if (!seated) {
      for (int i = 0; i < ctx.seats.size(); ++i) {
        if (!ctx.seats[i].is_occupied) {
          auto& seat = ctx.seats[i];
          seat.nickname = p->nickname;
          seat.chips = p->chips;
          seat.connection = std::move(p);
          seat.connection->state = PlayerState::InRoom;
          seated = true;

          seat.connection->send_message(
              Net::MsgStruct{"RMST", ctx.serialize()});

          ctx.broadcast_ex(i, Msg::PJIN, ser_player(i, ctx));
          std::cout << std::format("New player {} at seat {} ({}))\n",
                                   ctx.seats[i].nickname, i, seat.chips);

          seat.is_occupied = true;
          break;
        }
      }
    }

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
  static const arr<str_v, 21> valid_codes = {
      Msg::RDY1, Msg::GMLV, Msg::CHCK, Msg::FOLD, Msg::CALL,
      Msg::BETT, Msg::CDOK, Msg::CDFL, Msg::STOK, Msg::STFL,
      Msg::DNOK, Msg::DNFL, Msg::SDOK};
  return std::find(valid_codes.begin(), valid_codes.end(), code) !=
         valid_codes.end();
}

void Room::process_network_io() {
  for (int i = 0; i < ctx.seats.size(); ++i) {
    auto& seat = ctx.seats[i];

    if (seat.is_occupied && seat.connection && seat.connection->disconnected) {
      std::cout << std::format("Player {} disconnected (seat {})\n",
                               seat.nickname, i);
      seat.connection = nullptr;
      seat.is_ready = false;
      continue;
    }

    if (seat.is_active()) {
      auto& p = seat.connection;
      while (const auto& msg_opt = p->msg_client.reader.read()) {
        const auto& msg = msg_opt.value();

        // Validate message code
        if (!is_valid_room_code(msg.code)) {
          std::cerr << std::format(
              "Unknown room message {} from {}, disconnecting\n", msg.code,
              seat.nickname);
          p->disconnected = true;
          break;
        }

        // Global leave command
        if (msg.code == Msg::GMLV) {
          std::cout << std::format("Player {} leaving room\n", seat.nickname);

          seat.action_taken = GameUtils::PlayerAction::Left;
          const auto act_str =
              Net::Serde::write_net_str(seat.nickname) +
              Net::Serde::write_sm_int(static_cast<u8>(seat.action_taken)) +
              Net::Serde::write_sm_int(seat.action_amount);

          {
            std::lock_guard lg{return_mtx};
            p->state = PlayerState::AwaitingJoin;
            return_arr.emplace_back(std::move(p));
            std::cout << "Player Moved back to Main Thread" << std::endl;
          }

          seat.connection = nullptr;
          if (current_state->get_name() == "Lobby") {
            seat.is_occupied = false;
            seat.nickname = "";
          }
          ctx.broadcast_ex(i, Msg::PACT, act_str);

          break;
        }

        // Route to current state
        if (current_state) {
          current_state->on_message(*this, ctx, i, msg);
        }
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
      ctx.broadcast(Msg::PRDY,
                    Net::Serde::write_net_str(ctx.seats[seat_idx].nickname));
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
  ctx.current_high_bet = 0;
  has_bet_occurred = false;
  ctx.broadcast(Msg::GMRD, null);

  for (auto& s : ctx.seats) {
    s.total_bet += s.round_bet;
    s.round_bet = 0;
    s.action_amount = 0;
    // do not reset history for players who left or folded
    if (s.action_taken == GameUtils::PlayerAction::Left ||
        s.action_taken == GameUtils::PlayerAction::Fold) {
      continue;
    }
    s.action_taken = GameUtils::PlayerAction::None;
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
    ctx.current_actor = -1;
  } else {
    start_next_turn(ctx);
  }
}

void BettingState::start_next_turn(RoomContext& ctx) {
  if (action_queue.empty()) {
    ctx.current_actor = -1;
    return;
  }

  ctx.current_actor = action_queue.front();
  action_queue.pop_front();

  if (!ctx.seats[ctx.current_actor].is_active() ||
      ctx.seats[ctx.current_actor].is_folded) {
    start_next_turn(ctx);
    return;
  }

  std::cout << std::format("Turn: Seat {} ({})\n", ctx.current_actor,
                           ctx.seats[ctx.current_actor].nickname);
  ctx.broadcast(Msg::PTRN, Net::Serde::write_net_str(
                               ctx.seats[ctx.current_actor].nickname));
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
  if (ctx.current_actor == -1) {
    if (ctx.round_phase == RoundPhase::River) {
      room.transition_to<ShowdownState>();
    } else {
      room.transition_to<CommunityCardState>();
    }
  }
}

static str ser_act(const PlayerSeat& seat) {
  using namespace Net::Serde;
  return write_net_str(seat.nickname) +
         write_sm_int(static_cast<u8>(seat.action_taken)) +
         write_var_int(seat.action_amount);
}

opt<str> BettingState::check_bet_conditions(const PlayerSeat& seat,
                                            const Net::MsgStruct& msg) {
  if (has_bet_occurred) {
    return "Cannot raise (limit 1 bet/round)";
  }

  if (!msg.payload) {
    return "Bet amount required";
  }

  const auto& mb_bet_amount = Net::Serde::read_var_int(msg.payload.value());
  if (!mb_bet_amount) {
    return "Please send a numeric value";
  }

  const auto& [amount, _] = mb_bet_amount.value();
  if (amount > seat.chips) {
    return "Not enough chips to bet that amount";
  }

  return null;
}

void BettingState::on_message(Room& room, RoomContext& ctx, int seat_idx,
                              const Net::MsgStruct& msg) {
  if (seat_idx != ctx.current_actor) {
    ctx.send_to(seat_idx, Msg::NYET, null);
    return;
  }

  auto& seat = ctx.seats[seat_idx];
  bool turn_completed = false;

  if (msg.code == Msg::FOLD) {
    seat.is_folded = true;
    seat.action_taken = GameUtils::PlayerAction::Fold;

    ctx.send_to(seat_idx, Msg::ACOK, null);
    ctx.broadcast_ex(seat_idx, Msg::PACT, ser_act(seat));

    turn_completed = true;
    std::cout << std::format("Player {} folded\n", seat.nickname);
  } else if (msg.code == Msg::CHCK) {
    if (ctx.current_high_bet > seat.round_bet) {
      ctx.send_to(seat_idx, Msg::ACFL, "Cannot check, must call");
    } else {
      seat.action_taken = GameUtils::PlayerAction::Check;

      ctx.send_to(seat_idx, Msg::ACOK, null);
      ctx.broadcast_ex(seat_idx, Msg::PACT, ser_act(seat));

      turn_completed = true;
    }
  } else if (msg.code == Msg::BETT) {
    const auto& mb_err = check_bet_conditions(seat, msg);
    if (mb_err) {
      ctx.send_to(seat_idx, Msg::ACFL, mb_err.value());
    } else {
      const auto [amount, _] =
          Net::Serde::read_var_int(msg.payload.value()).value();
      seat.action_taken = GameUtils::PlayerAction::Bet;
      seat.action_amount = amount;
      ctx.current_high_bet = amount;
      seat.round_bet = amount;
      seat.chips -= amount;
      ctx.pot += amount;
      has_bet_occurred = true;

      ctx.send_to(seat_idx, Msg::ACOK, null);
      ctx.broadcast_ex(seat_idx, Msg::PACT, ser_act(seat));

      requeue_others(ctx, seat_idx);
      turn_completed = true;
      std::cout << std::format("Player {} bets {}\n", seat.nickname, amount);
    }

  } else if (msg.code == Msg::CALL) {
    // Basically checking if an all_in is happening
    const auto chip_amount =
        ctx.current_high_bet > seat.chips ? seat.chips : ctx.current_high_bet;
    seat.chips -= chip_amount;
    seat.round_bet += chip_amount;
    ctx.pot += chip_amount;
    seat.action_taken = GameUtils::PlayerAction::Call;
    seat.action_amount = chip_amount;

    ctx.send_to(seat_idx, Msg::ACOK, null);
    ctx.broadcast_ex(seat_idx, Msg::PACT, ser_act(seat));

    turn_completed = true;
    std::cout << std::format("Player {} calls {}\n", seat.nickname,
                             chip_amount);
  }

  if (turn_completed) {
    start_next_turn(ctx);
  }
}

void ShowdownState::on_enter(Room& room, RoomContext& ctx) {
  std::cout << "State: Enter Showdown" << std::endl;

  str payload = Net::Serde::write_sm_int(ctx.count_occupied_seats());

  vec<pair<PokerScore, str>> scores;
  scores.reserve(ctx.count_occupied_seats());

  for (int i = 0; i < ctx.seats.size(); ++i) {
    const auto& seat = ctx.seats[i];
    if (seat.is_occupied) {
      const arr<u8, 2> hand{seat.hand[0], seat.hand[1]};
      const arr<u8, 5> river{
          ctx.community_cards[0], ctx.community_cards[1],
          ctx.community_cards[2], ctx.community_cards[3],
          ctx.community_cards[4],
      };

      const auto score =
          scoring.evaluate_poker_hand(std::move(hand), std::move(river));
      scores.emplace_back(pair{std::move(score), seat.nickname});

      payload += Net::Serde::write_net_str(seat.nickname);
      payload += std::format("{:02}{:02}", seat.hand[0], seat.hand[1]);
    }
  }

  std::sort(
      scores.begin(), scores.end(),
      [](const pair<PokerScore, str>& lhs, const pair<PokerScore, str>& rhs) {
        const auto& lhs_score = std::get<0>(lhs);
        const auto& rhs_score = std::get<0>(rhs);

        return lhs_score < rhs_score;
      });

  const auto& [winner_score, winner_nick] = scores[0];

  const str winner_payload = Net::Serde::write_net_str(winner_nick) +
                             Net::Serde::write_var_int(ctx.pot);

  ctx.broadcast(Msg::SDWN, payload);
  ctx.broadcast(Msg::GWIN, winner_payload);
}

void ShowdownState::on_leave(Room& room, RoomContext& ctx) {}

void ShowdownState::on_tick(Room& room, RoomContext& ctx) {
  int count_players_accepted = 0;
  for (int i = 0; i < ROOM_MAX_PLAYERS; ++i) {
    if (ctx.seats[i].showdowm_okay) {
      count_players_accepted++;
    }
  }

  if (count_players_accepted == ctx.count_active_players()) {
    ctx.broadcast(Msg::GMDN, null);
    room.transition_to<LobbyState>();
  }
}

void ShowdownState::on_message(Room& room, RoomContext& ctx, int seat_idx,
                               const Net::MsgStruct& msg) {
  std::cerr << std::format("Unexpected message {} in Showdown state\n",
                           msg.code);
  if (msg.code == "SDOK") {
    ctx.seats[seat_idx].showdowm_okay = true;
  }
}
