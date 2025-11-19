#pragma once

#include <cstddef>
#include <iostream>
#include <iterator>
#include <optional>
#include <sstream>
#include <string>

#include "Babel.hpp"

constexpr std::size_t MSG_CODE_SIZE = 4;
constexpr std::size_t PAYLOAD_LEN_SIZE = 4;
constexpr std::size_t BG_INT_STR_LEN = 4;
constexpr std::size_t SM_INT_STR_LEN = 2;

static constexpr std::size_t sm_int_byte_count = 2;
static constexpr std::size_t bg_int_byte_count = 4;

namespace Net {

struct MsgStruct {
  std::string code;
  std::optional<std::string> payload = std::nullopt;

  std::string to_string() const {
    std::stringstream ss;
    ss << "PKR";

    if (payload) {
      ss << "P";
    } else {
      ss << "N";
    }

    ss << code;
    if (payload) {
      const auto& contents = payload.value();
      const auto& len_str = std::format("{:04d}", contents.size());
      ss << len_str << contents;
    }

    ss << "\n";
    return ss.str();
  }
};

namespace Serde {

inline opt<pair<usize, usize>> read_sm_int(const str& payload,
                                           const usize begin_index = 0) {
  if ((payload.size() - begin_index) < SM_INT_STR_LEN) {
    return null;
  }

  const auto& start = payload.begin() + begin_index;
  const auto& end = start + SM_INT_STR_LEN;

  const auto& sm_int_str_v = str_v{start, end};

  usize sm_int = 0;
  for (const auto& byte : sm_int_str_v) {
    if (byte < '0' || byte > '9') {
      return null;
    }

    sm_int = sm_int * 10 + static_cast<usize>(byte - '0');
  }

  return pair{sm_int, sm_int_byte_count};
}

inline opt<std::pair<usize, usize>> read_bg_int(const str& payload,
                                                const usize begin_index = 0) {
  if ((payload.size() - begin_index) < BG_INT_STR_LEN) {
    return null;
  }

  const auto& start = payload.begin() + begin_index;
  const auto& end = start + BG_INT_STR_LEN;

  const auto& big_int_str_v = str_v{start, end};

  usize bg_int = 0;
  for (const auto& byte : big_int_str_v) {
    if (byte < '0' || byte > '9') {
      return null;
    }

    bg_int = bg_int * 10 + static_cast<usize>(byte - '0');
  }

  return pair{bg_int, bg_int_byte_count};
}

inline opt<std::pair<str, usize>> read_str(const str& payload,
                                           const usize begin_index = 0) {
  const auto& m_size = read_bg_int(payload, begin_index);
  if (!m_size) {
    return null; // invalid characters within size
  }

  const auto& [size, bytes_read] = m_size.value();
  if ((payload.size() - BG_INT_STR_LEN) < size) {
    return null; // size specifies longer string than is present
  }

  const auto& start = payload.begin() + begin_index + BG_INT_STR_LEN;
  const auto& end = start + size;

  const usize total_bytes_read = bytes_read + std::distance(start, end);

  return pair{str{start, end}, total_bytes_read};
}

inline str write_sm_int(usize num) { return std::format("{:02d}", num); }
inline str write_bg_int(usize num) { return std::format("{:04d}", num); }
inline str write_net_str(const str& usr_str) {
  return write_bg_int(usr_str.size()) + usr_str;
}

enum class MainPart {
  Magic_1,
  Magic_2,
  Magic_3,
  Type,
  Code,
  Size,
  Payload,
  Endline
};

enum class ParserState { OK, Done, Invalid };

enum class MsgType {
  Payload = 'P',
  NoPayload = 'N',
};

struct ParseResults {
  bool error_occured = false;
  bool parser_done = false;
  str code;
  usize bytes_parsed = 0;
  MsgType type;
  opt<str> payload = null;
};

class MainParser final {
private:
  vec<char> payload{};
  str code{};
  MainPart phase = MainPart::Magic_1;
  MsgType type;
  usize size_index = 0;
  usize code_index = 0;
  usize payload_len = 0;

public:
  void reset_parser() noexcept {
    payload.clear();
    code.clear();
    phase = MainPart::Magic_1;
    size_index = 0;
    code_index = 0;
    payload_len = 0;
  }

  ParserState parse_byte(const char& byte) {
    switch (phase) {
    case MainPart::Magic_1:
      if (byte != 'P') {
        std::cout << "Invalid Magic" << std::endl;
        return ParserState::Invalid;
      }
      phase = MainPart::Magic_2;
      break;

    case MainPart::Magic_2:
      if (byte != 'K') {
        std::cout << "Invalid Magic" << std::endl;
        return ParserState::Invalid;
      }
      phase = MainPart::Magic_3;
      break;

    case MainPart::Magic_3:
      if (byte != 'R') {
        std::cout << "Invalid Magic" << std::endl;
        return ParserState::Invalid;
      }
      phase = MainPart::Type;
      break;

    case MainPart::Type:
      if (byte != 'N' && byte != 'P') {
        std::cout << "Unknown Msg Type" << std::endl;
        return ParserState::Invalid;
      }

      type = static_cast<MsgType>(byte);
      phase = MainPart::Code;
      break;

    case MainPart::Code:
      code.push_back(byte);
      code_index++;

      if (code_index >= MSG_CODE_SIZE) {
        if (type == MsgType::NoPayload) {
          // skip to end if No Payload message is sent
          phase = MainPart::Endline;
        } else {
          phase = MainPart::Size;
        }
      }
      break;

    case MainPart::Size:
      if (byte < '0' || byte > '9') {
        std::cout << std::format("Non numeric character in size: {}", byte)
                  << std::endl;
        return ParserState::Invalid;
      }

      payload_len = payload_len * 10 + static_cast<usize>(byte - '0');
      size_index++;

      if (size_index >= PAYLOAD_LEN_SIZE) {
        phase = MainPart::Payload;
      }
      break;

    case MainPart::Payload:
      payload.push_back(byte);

      if (payload.size() == payload_len) {
        phase = MainPart::Endline;
      }

      break;

    case MainPart::Endline:
      if (byte == '\n') {
        return ParserState::Done;
      } else {
        return ParserState::Invalid;
      }
      break;
    }

    return ParserState::OK;
  }

  str get_payload() const noexcept {
    return str{payload.begin(), payload.end()};
  }

  // potentially returns a payload
  // if it does, it returns how many bytes were used to get that payload
  // otherwise it returns nullopt and
  struct ParseResults parse_bytes(const vec<char> bytes) {
    struct ParseResults res{};
    usize i = 0;

    while (true) {
      if (i >= bytes.size()) {
        break;
      }

      if (res.parser_done || res.error_occured) {
        break;
      }

      const ParserState state = parse_byte(bytes[i]);
      i++;

      switch (state) {
      case ParserState::OK:
        break;
      case ParserState::Done:
        res.parser_done = true;
        break;
      case ParserState::Invalid:
        res.error_occured = true;
        break;
      }
    }

    if (res.parser_done) {
      res.type = type;
      res.code = code;
      if (type == MsgType::Payload) {
        res.payload = get_payload();
      }
    }

    res.bytes_parsed = i;
    return res;
  }

  struct ParseResults parse_bytes(const str_v& byte_view) {
    std::cout << "Parsing start" << std::endl;
    struct ParseResults res{};
    usize i = 0;

    while (true) {
      if (i >= byte_view.size()) {
        break;
      }

      if (res.parser_done == true || res.error_occured == true) {
        break;
      }
      const ParserState state = parse_byte(byte_view[i]);
      i++;

      switch (state) {
      case ParserState::OK:
        break;
      case ParserState::Done:
        res.parser_done = true;
        break;
      case ParserState::Invalid:
        res.error_occured = true;
        break;
      }
    }

    if (res.parser_done) {
      res.type = type;
      res.code = code;
      if (type == MsgType::Payload) {
        res.payload = get_payload();
      }
    }

    res.bytes_parsed = i;
    std::cout << "Parsing end" << std::endl;
    return res;
  }
};

} // namespace Serde

} // namespace Net
