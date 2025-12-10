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
#include "SockWrapper.hpp"

constexpr long max_port = 65535;
constexpr unsigned long max_ip_part = 255;

struct usr_args {
  u16 port;
  str ip;
};

opt<usr_args> parse_args(vec<str> args) {
  struct usr_args ret{0};
  unsigned long usr_port;

  try {
    usr_port = std::stoul(args[0]);
  } catch (const std::exception& e) {
    std::cout << "Parsing error: " << e.what() << std::endl;
    return null;
  }

  if (usr_port > max_port) {
    return null;
  } else {
    u16 port = scast<u16>(usr_port);
    ret.port = port;
  }

  if (args.size() >= 2) {
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

    for (const auto& part : parts) {
      unsigned long ip_part;
      try {
        ip_part = std::stoul(part);
      } catch (const std::exception& e) {
        std::cout << "Parsing error: " << e.what() << std::endl;
      }

      if (ip_part > max_ip_part) {
        std::cout << "Individual ip parts must be from range <0;255>"
                  << std::endl;
        return null;
      }
    }
    ret.ip = args[1];
  } else {
    ret.ip = "ANY";
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

  const auto parsed_args = maybe_parsed_args.value();
  std::cout << "Parsed args: " << parsed_args.ip << " " << parsed_args.port
            << std::endl;

  uq_ptr<ServerSocket> ps;
  uq_ptr<Server> pa;
  try {
    std::cout << "Opening socket..." << std::endl;
    ps = std::make_unique<ServerSocket>(parsed_args.port, parsed_args.ip);
    std::cout << "Socket opened" << std::endl;
    std::cout << "Creating server..." << std::endl;
    pa = std::make_unique<Server>();
    std::cout << "Server created" << std::endl;
  } catch (const std::exception& e) {
    std::cout << e.what() << std::endl;
    return EXIT_FAILURE;
  }

  const ServerSocket& sock = *ps;
  Server& s = *pa;
  s.run(sock);

  return EXIT_SUCCESS;
}
