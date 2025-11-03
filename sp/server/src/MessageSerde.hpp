/*
 * Message Parser able to handle internet messages
 * This means that the parser can be advanced by a certain number of bytes
 * If during those bytes a value is produced, the value is returned
 * Otherwise std::nullopt is returned
 */

#include <cstddef>
#include <optional>
#include <string>
#include <unordered_set>
#include <vector>

namespace NetSerde {

const std::unordered_set<char> msg_type_set{'A', 'C', 'M', 'R', 'S'};
enum class MsgType {
  PlayerAction = 'A',
  Conncetion = 'C',
  Message = 'M',
  Response = 'R',
  State = 'S',
};

const std::unordered_set<char> player_action_set{'F', 'B', 'C', 'E'};
enum class PlayerAction {
  Fold = 'F',
  Bet = 'B',
  Check = 'C',
  Even = 'E',
};

const std::unordered_set<char> connection_action_set{'C', 'D'};
enum class ConnectAction {
  Connect = 'C',
  Disconnect = 'D',
};

const std::unordered_set<char> response_type_set{'E', 'A', 'F', 'N'};
enum class ResponseType {
  Evened = 'E',
  AllIn = 'A',
  Fold = 'F',
  NoCash = 'N',
};

const std::unordered_set<char> state_type_set{'N', 'S'};
enum class StateType {
  NoState = 'N',
  State = 'S',
};

struct STATE {
  int round;           // INT(2)
  bool folded;         // "T" / "F"
  unsigned long chips; // INT(4) + VAL := STR_INT(VAL_LEN)
};

struct MSG {
  char magic[3];
  MsgType msg_type;
  int msg_size; // INT(4)
  std::vector<char> payload;
};

struct parser_state {
  std::size_t remaining_expected_bytes;
};

class Reader final {
private:
  struct parser_state state;
  std::size_t parse_pos = 0;
  std::vector<char> buffer;

  // Assumes that byte_count bytes have been inserted and are present in the
  // vector
  std::optional<std::string> parse_bytes(const std::size_t byte_count);

public:
  std::optional<std::string> advance(const std::vector<char>& new_bytes);
};

std::string serialize();

enum class ParserState {
  OK,
  DONE,
  INVALID,
};

enum class MsgPart {
  MAGIC,
  TYPE,
  SIZE,
  PAYLOAD,
};

struct parser_state_header {
  std::vector<char> msg{};
  MsgPart phase = MsgPart::MAGIC;
  std::size_t index = 0;
  std::size_t size_index = 0;
  std::size_t magic_index = 0;
  std::size_t msg_size = 0;
};

std::optional<std::string> parse_bytes(parser_state_header& state,
                                       std::vector<char> bytes);
ParserState parse_byte(parser_state_header& state, const char& byte);
std::string get_payload(const parser_state_header& state);
} // namespace NetSerde
