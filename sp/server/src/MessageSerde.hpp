#include <cstddef>
#include <iostream>
#include <optional>
#include <string>
#include <string_view>
#include <vector>

constexpr std::size_t MSG_CODE_SIZE = 4;
constexpr std::size_t PAYLOAD_LEN_SIZE = 5;
constexpr std::size_t SIZESTR_LEN = 4;

using str = std::string;
using str_v = std::string_view;
using usize = std::size_t;

template <typename T> using opt = std::optional<T>;
template <typename T> using vec = std::vector<T>;
template <typename T, usize S> using arr = std::array<T, S>;

constexpr std::nullopt_t null = std::nullopt;

static opt<usize> read_size(const str& payload, const usize begin_index = 0) {
  if ((payload.size() - begin_index) < SIZESTR_LEN) {
    return null;
  }

  const auto& start = payload.begin() + begin_index;
  const auto& end = start + SIZESTR_LEN;

  const auto& size_str_v = str_v{start, end};

  usize size = 0;
  for (const auto& byte : size_str_v) {
    if (byte < '0' || byte > '9') {
      return null;
    }

    size = size * 10 + static_cast<usize>(byte - '0');
  }

  return size;
}

static opt<str> read_var_str(const str& payload, const usize begin_index = 0) {
  const auto& m_size = read_size(payload, begin_index);
  if (!m_size) {
    return null; // invalid characters within size
  }
  const auto& size = m_size.value();

  if ((payload.size() - SIZESTR_LEN) < size) {
    return null; // size specifies longer string than is present
  }

  const auto& start = payload.begin() + begin_index + SIZESTR_LEN;
  const auto& end = start + size;

  return str{start, end};
}

namespace Net {

namespace Msg {

struct GeneralString {
  str msg;

  static opt<GeneralString> emit_msg(const str& payload) {
    const auto& m_msg = read_var_str(payload);

    if (!m_msg) {
      return null;
    }

    return GeneralString{m_msg.value()};
  }
};

} // namespace Msg

namespace Serde {

enum class MainPart { Magic_1, Magic_2, Magic_3, Size, Code, Payload };

enum class ParserState { OK, Done, Invalid };

struct ParseResults {
  bool payload_reached;
  bool error_occured;
  usize bytes_parsed;
  str code;
  str payload;
};

class MainParser final {
private:
  vec<char> payload{};
  vec<char> code{MSG_CODE_SIZE};
  MainPart phase = MainPart::Magic_1;
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
      phase = MainPart::Size;
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
        payload_len -= MSG_CODE_SIZE;
        phase = MainPart::Code;
      }
      break;

    case MainPart::Code:
      code.push_back(byte);
      code_index++;

      if (code_index >= MSG_CODE_SIZE) {
        if (payload_len <= 0) {
          return ParserState::Done;
        }

        phase = MainPart::Payload;
      }
      break;

    case MainPart::Payload:
      payload.push_back(byte);

      if (payload.size() == payload_len) {
        return ParserState::Done;
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
    while (i < bytes.size()) {
      const ParserState state = parse_byte(bytes[i]);
      i++;

      switch (state) {
      case ParserState::OK:
        continue;
      case ParserState::Done:
        res.payload = get_payload();
        res.bytes_parsed = i;
        res.payload_reached = true;
        res.error_occured = false;
        return res;
      case ParserState::Invalid:
        res.error_occured = true;
        res.bytes_parsed = i;
        return res;
      }
    }

    res.error_occured = false;
    res.bytes_parsed = i;
    res.payload_reached = false;

    return res;
  }
};

} // namespace Serde

} // namespace Net
