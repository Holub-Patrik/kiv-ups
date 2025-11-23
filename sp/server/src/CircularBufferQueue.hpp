/*
 * An implementation of lockless thread-safe queue
 *
 * Thread safety is guaranteed only when certain conditions are met:
 * - There are only 2 threads accessing the queue
 * - Each thread has a clear role, and never switch during the run of the
 * program
 *
 * The conditions need to be met, since the implementation expects that:
 * - only the writer can advance the write position
 * - only the reader can advance the read position
 *
 * Similar concept applies to the Server and Client. That is just a wrapper
 * so instead of creating 2 Buffers, 2 Writers and 2 Readers, instead you can
 * just TwinBuffer, Client and Server to achieve the same
 */

#pragma once

#include "Babel.hpp"

#include <atomic>
#include <chrono>
#include <thread>

constexpr auto wait_time = std::chrono::milliseconds(20);

namespace CB {

template <typename Type, std::size_t Size> struct Buffer {
  arr<Type, Size> data;
  std::atomic<u64> read_pos = 0;
  std::atomic<u64> write_pos = 1;

  auto advance_read() -> void { read_pos = (read_pos + 1) % Size; }
  auto advance_write() -> void { write_pos = (write_pos + 1) % Size; }

  // doesn't advance position, only returns the position as if it was advanced
  auto advanced_pos(const auto& cur_pos) -> u64 { return (cur_pos + 1) % Size; }
};

template <typename Type, std::size_t Size> class Reader final {
private:
  Buffer<Type, Size>& buffer;

public:
  Reader<Type, Size>(Buffer<Type, Size>& buf) : buffer(buf) {}

  // The same as read, but doesn't advance the read position
  const Type& peek() const { return buffer.data[buffer.read_pos]; }

  void advance() const { buffer.advance_read(); }

  std::optional<Type> read() const {
    if (buffer.advanced_pos(buffer.read_pos) == buffer.write_pos) {
      return std::nullopt;
    } else {
      buffer.advance_read();
      const auto& ret_val = buffer.data[buffer.read_pos];
      return ret_val;
    }
  }

  const Type& wait_and_read() const {
    while (buffer.advanced_pos(buffer.read_pos) == buffer.write_pos) {
      std::this_thread::sleep_for(wait_time);
    }

    const auto& ret_val = buffer.data[buffer.read_pos];
    buffer.advance_read();
    return ret_val;
  }
};

template <typename Type, std::size_t Size> class Writer final {
private:
  Buffer<Type, Size>& buffer;

public:
  Writer<Type, Size>(Buffer<Type, Size>& buf) : buffer(buf) {}

  bool insert(const Type& item) const {
    if (buffer.write_pos == buffer.read_pos) {
      return false;
    } else {
      buffer.data[buffer.write_pos] = item;
      buffer.advance_write();
      return true;
    }
  }

  void wait_and_insert(const Type& item) const {
    while (buffer.write_pos == buffer.read_pos) {
      std::this_thread::sleep_for(std::chrono::milliseconds(20));
    }

    buffer.data[buffer.write_pos] = item;
    buffer.advance_write();
  }
};

template <typename Type, std::size_t Size> struct TwinBuffer {
  Buffer<Type, Size> buffer_one{};
  Buffer<Type, Size> buffer_two{};
};

template <typename Type, std::size_t Size> class Server final {
public:
  Reader<Type, Size> reader;
  Writer<Type, Size> writer;

  Server<Type, Size>(TwinBuffer<Type, Size>& twin_buf)
      : reader(twin_buf.buffer_one), writer(twin_buf.buffer_two) {}
};

template <typename Type, std::size_t Size> class Client final {
public:
  Reader<Type, Size> reader;
  Writer<Type, Size> writer;

  Client<Type, Size>(TwinBuffer<Type, Size>& twin_buf)
      : reader(twin_buf.buffer_two), writer(twin_buf.buffer_one) {}
};

} // namespace CB
