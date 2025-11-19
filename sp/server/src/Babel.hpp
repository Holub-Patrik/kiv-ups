#pragma once

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

template <typename T> using opt = std::optional<T>;
template <typename T> using vec = std::vector<T>;
template <typename T, usize S> using arr = std::array<T, S>;
template <typename T1, typename T2> using pair = std::pair<T1, T2>;

template <typename T> using uq_ptr = std::unique_ptr<T>;

constexpr std::nullopt_t null = std::nullopt;
