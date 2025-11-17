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

#include <array>
#include <chrono>
#include <cstddef>
#include <optional>
#include <thread>

constexpr auto wait_time = std::chrono::milliseconds(20);

namespace CB {

template <typename Type, std::size_t Size> struct Buffer {
  std::array<Type, Size> _data;
  std::int64_t _read_pos = 0;
  std::int64_t _write_pos = 1;

  void advance_read() { _read_pos = (_read_pos + 1) % Size; }
  void advance_write() { _write_pos = (_write_pos + 1) % Size; }

  // doesn't advance position, only returns the position as if it was advanced
  std::size_t advanced_pos(const auto& cur_pos) { return (cur_pos + 1) % Size; }
};

template <typename Type, std::size_t Size> class Reader final {
private:
  Buffer<Type, Size>& _buffer;

public:
  Reader<Type, Size>(Buffer<Type, Size>& buf) : _buffer(buf) {}

  // The same as read, but doesn't advance the read position
  const Type& peek() const { return _buffer._data[_buffer._read_pos]; }

  void advance() const { _buffer.advance_read(); }

  std::optional<Type> read() const {
    if (_buffer.advanced_pos(_buffer._read_pos) == _buffer._write_pos) {
      return std::nullopt;
    } else {
      _buffer.advance_read();
      const auto& ret_val = _buffer._data[_buffer._read_pos];
      return ret_val;
    }
  }

  const Type& wait_and_read() const {
    while (_buffer.advanced_pos(_buffer._read_pos) == _buffer._write_pos) {
      std::this_thread::sleep_for(wait_time);
    }

    const auto& ret_val = _buffer._data[_buffer._read_pos];
    _buffer.advance_read();
    return ret_val;
  }
};

template <typename Type, std::size_t Size> class Writer final {
private:
  Buffer<Type, Size>& _buffer;

public:
  Writer<Type, Size>(Buffer<Type, Size>& buf) : _buffer(buf) {}

  bool insert(const Type& item) const {
    if (_buffer._write_pos == _buffer._read_pos) {
      return false;
    } else {
      _buffer._data[_buffer._write_pos] = item;
      _buffer.advance_write();
      return true;
    }
  }

  void wait_and_insert(const Type& item) const {
    while (_buffer._write_pos == _buffer._read_pos) {
      std::this_thread::sleep_for(std::chrono::milliseconds(20));
    }

    _buffer._data[_buffer._write_pos] = item;
    _buffer.advance_write();
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
