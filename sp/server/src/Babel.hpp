#pragma once

#include <chrono>
#include <cstdint>
#include <memory>
#include <optional>
#include <string>
#include <string_view>
#include <vector>

using str = std::string;
using str_v = std::string_view;
using usize = std::size_t;
using u8 = std::uint8_t;
using u16 = std::uint16_t;
using u32 = std::uint32_t;
using u64 = std::uint64_t;
using i8 = std::int8_t;
using i16 = std::int16_t;
using i32 = std::int32_t;
using i64 = std::int64_t;

using hr_clock = std::chrono::high_resolution_clock;
using seconds = std::chrono::seconds;
using millis = std::chrono::milliseconds;
using micros = std::chrono::microseconds;
using nanos = std::chrono::nanoseconds;

template <typename T> using opt = std::optional<T>;
template <typename T> using vec = std::vector<T>;
template <typename T, usize S> using arr = std::array<T, S>;
template <typename T1, typename T2> using pair = std::pair<T1, T2>;
template <typename T> using time_point = std::chrono::time_point<T>;

template <typename T> using uq_ptr = std::unique_ptr<T>;

constexpr std::nullopt_t null = std::nullopt;

using Result = struct res_info {
  bool connect = false;
  bool reconnect = false;
  usize room_idx = 0;

  void reset() {
    connect = false;
    reconnect = false;
    room_idx = 0;
  }
};

template <typename T, typename Arg>
auto scast(Arg&& arg) -> decltype(static_cast<T>(std::forward<Arg>(arg))) {
  return static_cast<T>(std::forward<Arg>(arg));
}

template <typename T, typename Arg>
auto dur_cast(Arg&& arg)
    -> decltype(std::chrono::duration_cast<T>(std::forward<Arg>(arg))) {
  return std::chrono::duration_cast<T>(std::forward<Arg>(arg));
}

namespace Msg {
// here so the comments can be next to messages
#define cmp constexpr
// Connection handshake (Client <-> Server)

cmp str_v CONN = "CONN"; // Client: Conn with nick
cmp str_v PNOK = "PNOK"; // Server: Player nick OK
cmp str_v RCON = "RCON"; // Server: Ask reconnect
cmp str_v FAIL = "FAIL"; // Server: Generic failure
cmp str_v PINF = "PINF"; // Client: Send player info
cmp str_v PIOK = "PIOK"; // Server: Player info OK

// Room listing (Client <-> Server)
cmp str_v RMRQ = "RMRQ"; // Client: Request room list
cmp str_v ROOM = "ROOM"; // Server: Room info
cmp str_v DONE = "DONE"; // Server: End of room list
cmp str_v RMOK = "RMOK"; // Client: Room received OK
cmp str_v RMFL = "RMFL"; // Client: Room received fail

// Room updates (Server -> Client)
cmp str_v RMUP = "RMUP"; // Server: Room update
cmp str_v UPOK = "UPOK"; // Client: Update OK
cmp str_v UPFL = "UPFL"; // Client: Update fail
cmp str_v CRVR = "CRVR";

// Join room (Client <-> Server)
cmp str_v JOIN = "JOIN"; // Client: Join request
cmp str_v JNOK = "JNOK"; // Server: Join OK
cmp str_v JNFL = "JNFL"; // Server: Join failed

// Room state sync (Server <-> Client)
cmp str_v RMST = "RMST"; // Server: Room state
cmp str_v STOK = "STOK"; // Client: State OK
cmp str_v STFL = "STFL"; // Client: State fail
cmp str_v PJIN = "PJIN"; // Server: Player joined

// In-room actions (Client -> Room)
cmp str_v RDY1 = "RDY1"; // Client: Player ready
cmp str_v GMLV = "GMLV"; // Client: Leave room
cmp str_v CHCK = "CHCK"; // Client: Check
cmp str_v FOLD = "FOLD"; // Client: Fold
cmp str_v CALL = "CALL"; // Client: Call
cmp str_v BETT = "BETT"; // Client: Bet amount

// In-room responses (Room -> Client)
cmp str_v PRDY = "PRDY"; // Server: Player X ready broadcast
cmp str_v GMST = "GMST"; // Server: Game started (room locked)
cmp str_v GMRD = "GMRD"; // Server: Game round
cmp str_v CDTP = "CDTP"; // Server: Card to player (2 cards)
cmp str_v PTRN = "PTRN"; // Server: Player [Nick] turn
cmp str_v ACOK = "ACOK"; // Server: Action OK
cmp str_v ACFL = "ACFL"; // Server: Action failed
cmp str_v NYET = "NYET"; // Server: Not your turn
cmp str_v PACT = "PACT"; // Server: Player action (EBcast)

// In-room responses (Client -> Room)
cmp str_v CDOK = "CDOK";
cmp str_v CDFL = "CDFL";

// Showdown (Server -> Client)
cmp str_v SDWN = "SDWN"; // Server: Showdown with all cards
cmp str_v SDOK = "SDOK"; // Client: Showdown OK
cmp str_v SDFL = "SDFL"; // Client: Showdown fail

// Win (Server -> Client)
cmp str_v GWIN = "GWIN"; // Server: Win with winner's nick
cmp str_v GWOK = "GWOK"; // Client: Win OK
cmp str_v GWFL = "GWFL"; // Client: Win fail

// Game end (Server -> Client)
cmp str_v GMDN = "GMDN"; // Server: Winner info
cmp str_v DNOK = "DNOK"; // Client: Done OK
cmp str_v DNFL = "DNFL"; // Client: Done fail

// Disconnect (Both directions)
cmp str_v DCON = "DCON"; // Forceful disconnect

#undef cmp
} // namespace Msg
