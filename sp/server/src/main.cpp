#include <cstddef>
#include <cstdlib>
#include <cstring>
#include <exception>
#include <iostream>
#include <optional>
#include <string>
#include <vector>

extern "C" {
#include <asm-generic/socket.h>
#include <netinet/in.h>
#include <sys/socket.h>
#include <sys/types.h>
#include <sys/un.h>
#include <unistd.h>
}

#include "MainThread.hpp"

constexpr long max_port = 65535;
constexpr int player_count = 4;
constexpr std::size_t TIMEOUT_DISCONNECT = 5000;

struct usr_args {
  std::uint16_t port;
};

auto parse_args(std::vector<std::string> args) -> std::optional<usr_args> {
  struct usr_args ret{0};

  try {
    unsigned long usr_part = std::stoul(args[0]);
    if (usr_part > max_port) {
      return std::nullopt;
    } else {
      std::uint16_t port = static_cast<std::uint16_t>(usr_part);
      ret.port = port;
    }
  } catch (std::exception e) {
    return std::nullopt;
  }

  return ret;
}

auto main(int argc, char* argv[]) -> int {
  if (argc <= 1) {
    std::cout << "Not enough arguments." << std::endl;
    return EXIT_FAILURE;
  }

  std::vector<std::string> args{};
  for (int i = 1; i < argc; i++) {
    args.emplace_back(std::string{argv[i]});
  }

  const auto maybe_parsed_args = parse_args(args);
  if (!maybe_parsed_args) {
    std::cout << "Error during parsing arguments." << std::endl;
    return -1;
  }

  std::cout << "Parsed args" << std::endl;
  const auto parsed_args = maybe_parsed_args.value();

  Server s{};
  s.run(parsed_args.port);

  return EXIT_SUCCESS;
}
