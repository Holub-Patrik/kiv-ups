#include <algorithm>
#include <optional>

#include "Babel.hpp"

struct PokerScore {
  u8 category = 0;
  arr<u8, 5> tie_breakers = {0, 0, 0, 0, 0};

  bool operator<(const PokerScore& o) const noexcept {
    if (category != o.category) {
      return category < o.category;
    }
    return tie_breakers < o.tie_breakers;
  }
  bool operator==(const PokerScore& o) const noexcept = default;
  bool operator>(const PokerScore& o) const noexcept { return o < *this; }
};

inline u8 rank_of(u8 card) { return card % 13; }
inline u8 suit_of(u8 card) { return card / 13; }

struct Counts {
  arr<int, 13> rank = {};
  arr<int, 4> suit = {};
  arr<int, 5> freq = {};

  explicit Counts(const arr<u8, 7>& cards) {
    for (u8 c : cards) {
      rank[rank_of(c)]++;
      suit[suit_of(c)]++;
    }

    for (int r = 0; r < 13; ++r) {
      if (rank[r] >= 2 && rank[r] <= 4) {
        freq[rank[r]]++;
      }
    }
  }
};

class Scoring {
private:
  opt<u8> find_straight_high(const arr<int, 13>& rank_counts) const noexcept {
    u8 streak = 0, high = 0;

    for (int r = 12; r >= 0; --r) {
      if (rank_counts[r] == 0) {
        streak = 0;
        if (r < 4)
          break;
        continue;
      }

      if (streak == 0) {
        high = r;
        streak = 1;
      } else if (r == high - streak) {
        if (++streak == 5)
          return high;
      } else {
        high = r;
        streak = 1;
        if (r < 4)
          break;
      }
    }

    if (rank_counts[12] && rank_counts[0] && rank_counts[1] && rank_counts[2] &&
        rank_counts[3])
      return 3;
    return null;
  }

  opt<PokerScore> try_straight_flush(const arr<u8, 7>& cards,
                                     const Counts& c) const noexcept {
    u8 flush_suit = 4;
    for (u8 s = 0; s < 4; ++s)
      if (c.suit[s] >= 5) {
        flush_suit = s;
        break;
      }
    if (flush_suit == 4)
      return null;

    arr<int, 13> flush_ranks = {};
    for (u8 c2 : cards)
      if (suit_of(c2) == flush_suit)
        flush_ranks[rank_of(c2)]++;

    if (auto h = find_straight_high(flush_ranks))
      return PokerScore{8, {*h}};
    return null;
  }

  opt<PokerScore> try_four_of_a_kind(const Counts& c) const noexcept {
    for (int r = 12; r >= 0; --r)
      if (c.rank[r] == 4)
        for (int k = 12; k >= 0; --k)
          if (k != r && c.rank[k] > 0)
            return PokerScore{7, {scast<u8>(r), scast<u8>(k)}};
    return null;
  }

  opt<PokerScore> try_full_house(const Counts& c) const noexcept {
    int trips = -1, pair = -1;

    for (int r = 12; r >= 0; --r) {
      if (c.rank[r] >= 3 && trips == -1)
        trips = r;
      else if (c.rank[r] >= 2 && pair == -1)
        pair = r;
    }

    if (trips >= 0 && pair >= 0)
      return PokerScore{6, {scast<u8>(trips), scast<u8>(pair)}};
    return null;
  }

  opt<PokerScore> try_flush(const arr<u8, 7>& cards,
                            const Counts& c) const noexcept {
    u8 fs = 4;
    for (u8 s = 0; s < 4; ++s)
      if (c.suit[s] >= 5) {
        fs = s;
        break;
      }
    if (fs == 4)
      return null;

    arr<u8, 7> ranks;
    int idx = 0;
    for (u8 c2 : cards)
      if (suit_of(c2) == fs)
        ranks[idx++] = rank_of(c2);
    std::sort(ranks.begin(), ranks.begin() + idx, std::greater<u8>());

    return PokerScore{5, {ranks[0], ranks[1], ranks[2], ranks[3], ranks[4]}};
  }

  opt<PokerScore> try_straight(const Counts& c) const noexcept {
    if (auto h = find_straight_high(c.rank))
      return PokerScore{4, {*h}};
    return null;
  }

  opt<PokerScore> try_three_of_a_kind(const Counts& c) const noexcept {
    int t = -1;
    for (int r = 12; r >= 0; --r)
      if (c.rank[r] == 3) {
        t = r;
        break;
      }
    if (t == -1)
      return null;

    PokerScore s{3, {scast<u8>(t)}};
    u8 i = 1;
    for (int r = 12; r >= 0 && i < 3; --r)
      if (r != t && c.rank[r] > 0)
        s.tie_breakers[i++] = r;
    return s;
  }

  opt<PokerScore> try_two_pair(const Counts& c) const noexcept {
    if (c.freq[2] < 2)
      return null;

    int p1 = -1, p2 = -1;
    for (int r = 12; r >= 0; --r) {
      if (c.rank[r] >= 2) {
        if (p1 == -1)
          p1 = r;
        else if (p2 == -1) {
          p2 = r;
          break;
        }
      }
    }

    if (p1 == -1 || p2 == -1)
      return null;

    for (int k = 12; k >= 0; --k)
      if (k != p1 && k != p2 && c.rank[k] > 0)
        return PokerScore{2, {scast<u8>(p1), scast<u8>(p2), scast<u8>(k)}};
    return null;
  }

  opt<PokerScore> try_one_pair(const Counts& c) const noexcept {
    int p = -1;
    for (int r = 12; r >= 0; --r)
      if (c.rank[r] >= 2) {
        p = r;
        break;
      }
    if (p == -1)
      return null;

    PokerScore s{1, {scast<u8>(p)}};
    u8 i = 1;
    for (int r = 12; r >= 0 && i < 4; --r)
      if (r != p && c.rank[r] > 0)
        s.tie_breakers[i++] = r;
    return s;
  }

public:
  PokerScore evaluate_poker_hand(const arr<u8, 2>& hand,
                                 const arr<u8, 5>& river) const noexcept {
    arr<u8, 7> cards = {hand[0],  hand[1],  river[0], river[1],
                        river[2], river[3], river[4]};
    Counts c(cards);

    if (const auto sf = try_straight_flush(cards, c))
      return sf.value();
    if (const auto fk = try_four_of_a_kind(c))
      return fk.value();
    if (const auto fh = try_full_house(c))
      return fh.value();
    if (const auto f = try_flush(cards, c))
      return f.value();
    if (const auto s = try_straight(c))
      return s.value();
    if (const auto tk = try_three_of_a_kind(c))
      return tk.value();
    if (const auto tp = try_two_pair(c))
      return tp.value();
    if (const auto op = try_one_pair(c))
      return op.value();

    // High card
    PokerScore s{0, {}};
    u8 i = 0;
    for (int r = 12; r >= 0 && i < 5; --r)
      if (c.rank[r] > 0)
        s.tie_breakers[i++] = r;
    return s;
  }
};
