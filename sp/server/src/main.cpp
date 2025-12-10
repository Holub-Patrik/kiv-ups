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

#include "Babel.hpp"
#include "MainThread.hpp"

constexpr long max_port = 65535;
constexpr unsigned long max_ip_part = 255;

struct usr_args {
  u16 port;
  u32 ip;
};

opt<usr_args> parse_args(vec<str> args) {
  struct usr_args ret{0};
  unsigned long usr_port;

  try {
    usr_port = std::stoul(args[0]);
  } catch (std::exception e) {
    std::cout << "Parsing error: " << e.what() << std::endl;
    return null;
  }

  if (usr_port > max_port) {
    return null;
  } else {
    u16 port = scast<u16>(usr_port);
    ret.port = port;
  }

  auto ip_str_s = std::stringstream{args[1]};
  vec<str> parts{};
  parts.reserve(4);
  char del = '.';
  for (str part; std::getline(ip_str_s, part, '.');) {
    parts.push_back(part);
  }

  if (parts.size() != 4) {
    return null;
  }

  arr<u8, 4> ip_num_parts;
  usize index = 0;
  for (const auto& part : parts) {
    unsigned long ip_part;
    try {
      ip_part = std::stoul(part);
    } catch (std::exception e) {
      std::cout << "Parsing error: " << e.what() << std::endl;
    }

    if (ip_part > max_ip_part) {
      std::cout << "Individual ip parts must be from range <0;255>"
                << std::endl;
      return null;
    }

    ip_num_parts[index] = scast<u8>(ip_part);
    index++;
  }

  u32 ip = 0;
  ip |= ip_num_parts[0] << 24;
  ip |= ip_num_parts[1] << 16;
  ip |= ip_num_parts[2] << 8;
  ip |= ip_num_parts[3];
  ret.ip = ip;

  return ret;
}

auto main(int argc, char* argv[]) -> int {
  if (argc <= 2) {
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

  uq_ptr<Server> pa;
  try {
    pa = std::make_unique<Server>();
  } catch (std::exception e) {
    std::cout << e.what() << std::endl;
    return EXIT_FAILURE;
  }

  Server& s = *pa;
  s.run(parsed_args.port);

  return EXIT_SUCCESS;
}
