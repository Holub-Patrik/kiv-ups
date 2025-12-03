#pragma once

#include "Babel.hpp"
#include "MessageSerde.hpp"
#include "PlayerInfo.hpp"

extern "C" {
#include <sys/poll.h>
#include <sys/socket.h>
}

#include <unordered_map>

struct PollResult {
  int fd;
  PlayerInfo* p;
  vec<Net::MsgStruct> messages;
};

class SockPool final {
private:
  vec<struct pollfd> fds;
  // socket fd -> PlayerInfo
  std::unordered_map<int, std::unique_ptr<PlayerInfo>> fd_to_pinfo;

  void remove_member(int fd) {
    std::erase_if(fds, [fd](const struct pollfd e) { return e.fd == fd; });
    fd_to_pinfo.erase(fd);
  }

public:
  ~SockPool() {
    for (const auto& fd : fds) {
      shutdown(fd.fd, SHUT_RDWR);
      close(fd.fd);
    }
  }

  void accept_fd(int fd, std::unique_ptr<PlayerInfo>&& pinf) {
    fds.emplace_back(pollfd{fd, POLLIN, POLLERR | POLLHUP});
    fd_to_pinfo.insert_or_assign(fd, std::move(pinf));
  }

  void remove_fd(int fd) {
    if (!fd_to_pinfo.contains(fd)) {
      return;
    }

    remove_member(fd);
  }

  void transfer_fd(int fd, SockPool& other) {
    if (!fd_to_pinfo.contains(fd)) {
      return;
    }

    other.accept_fd(fd, std::move(fd_to_pinfo[fd]));

    remove_member(fd);
  }

  vec<PollResult> accept_messages() {
    vec<PollResult> accepted_messages;
    const int ret = poll(fds.data(), fds.size(), 500);
    if (ret < 0) {
      for (auto& fd : fds) {
        if (fd.revents & POLLIN) {
          auto& player_info = fd_to_pinfo[fd.fd];
          const vec<Net::MsgStruct> messages;
          player_info->accept_messages();
          if (messages.size() > 0) {
            accepted_messages.emplace_back(
                PollResult{fd.fd, player_info.get(), messages});
          }
        }

        if (fd.revents & POLLHUP || fd.revents & POLLERR) {
          auto& player_info = fd_to_pinfo[fd.fd];
          player_info->disconnect();
        }
      }
    }

    return accepted_messages;
  }
};
