#include "MessageSerde.hpp"

namespace {
using namespace NetSerde;

ParserState parse_byte(parser_state_header& state, const char& byte) {
  switch (state.phase) {
  case MsgPart::MAGIC:
    switch (state.index) {
    case 0:
      if (byte != 'P') {
        return ParserState::INVALID;
      } else {
        state.index++;
        return ParserState::OK;
      }
    case 1:
      if (byte != 'K') {
        return ParserState::INVALID;
      } else {
        state.index++;
        return ParserState::OK;
      }
    case 2:
      if (byte != 'R') {
        return ParserState::INVALID;
      } else {
        state.index++;
        state.phase = MsgPart::TYPE;
        return ParserState::OK;
      }
    }
  default:
    return ParserState::INVALID;
    break;
  case NetSerde::MsgPart::TYPE:
    if (!NetSerde::msg_type_set.contains(byte)) {
      return ParserState::INVALID;
    } else {
      state.index++;
      state.phase = NetSerde::MsgPart::SIZE;
      return ParserState::DONE;
    }
    break;
  case NetSerde::MsgPart::SIZE:
    if (byte < '0' || byte > '9') {
      return ParserState::INVALID;
    } else {
      state.msg_size =
          state.msg_size * 10 + static_cast<std::size_t>(byte - '0');
    }
    state.size_index++;
    state.index++;
    if (state.size_index > 3) {
      state.phase = NetSerde::MsgPart::PAYLOAD;
    }
    return ParserState::OK;
    break;
  case NetSerde::MsgPart::PAYLOAD:
    if (state.msg.size() == state.msg_size) {
      return ParserState::DONE;
    }
    state.msg.push_back(byte);
    return ParserState::OK;
  }
}

std::string get_payload(const parser_state_header& state) {
  return std::string{state.msg.begin(), state.msg.end()};
}

} // namespace
