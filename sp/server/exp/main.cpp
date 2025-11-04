#include "../src/CircularBufferQueue.hpp"

#include <chrono>
#include <cstdio>
#include <iostream>
#include <string>
#include <thread>

class InternetMsg {
public:
  virtual ~InternetMsg() = default;

  virtual std::string serialize() const noexcept = 0;
  virtual void deserialize(const std::string& payload) = 0;
  virtual bool is_correct() const noexcept = 0;
};

class FoldMsg final : public InternetMsg {
private:
  bool correct = false;

public:
  FoldMsg() = default;
  ~FoldMsg() = default;

  virtual std::string serialize() const noexcept override { return "PAF"; }

  virtual void deserialize(const std::string& payload) override {
    if (payload == "PAF}") {
      correct = true;
    } else {
      correct = false;
    }
  }

  virtual bool is_correct() const noexcept override { return correct; }
};

class BetMsg final : public InternetMsg {
private:
  bool correct = false;
  long bet_amount = 0;

public:
  BetMsg() = default;
  ~BetMsg() = default;

  virtual std::string serialize() const noexcept override {
    return std::to_string(bet_amount);
  }
  virtual void deserialize(const std::string& payload) override {
    try {
      bet_amount = std::stol(payload);
      correct = true;
    } catch (...) {
      correct = false;
    }
  }
  virtual bool is_correct() const noexcept override { return correct; }
};

int main(int argc, char* argv[]) {
  CB::Buffer<int, 3> buf;
  CB::Reader consumer{buf};
  CB::Writer producer{buf};

  std::cout << (producer.insert(1) ? "True" : "False") << std::endl;
  std::cout << (producer.insert(2) ? "True" : "False") << std::endl;
  std::thread t_prod{[&]() -> void {
    producer.wait_and_insert(3);
    std::cout << "Insert executed" << std::endl;
    producer.wait_and_insert(4);
    std::cout << "Insert executed" << std::endl;

    std::this_thread::sleep_for(std::chrono::seconds(5));

    producer.wait_and_insert(5);
    std::cout << "Insert executed" << std::endl;

    std::this_thread::sleep_for(std::chrono::seconds(5));

    producer.wait_and_insert(6);
    std::cout << "Insert executed" << std::endl;
  }};

  for (int i = 0; i < 2; i++) {
    const auto& val = consumer.read();
    if (val) {
      std::cout << val.value() << std::endl;
    } else {
      std::cout << "No Value Yet" << std::endl;
    }
  }

  for (int i = 0; i < 2; i++) {
    const auto& val = consumer.wait_and_read();
    std::cout << val << std::endl;
  }

  const auto& val = consumer.wait_and_read();
  std::cout << "Value:" << val << std::endl;
  const auto& val_2 = consumer.wait_and_read();
  std::cout << "Value:" << val_2 << std::endl;

  t_prod.join();

  const std::string test_str = "100;90;0020";
  int a = -1, b = -1;
  int ret = sscanf(test_str.c_str(), "%d;%d", &a, &b);
  std::cout << "a: " << a << " b: " << b << " ret_val: " << ret << std::endl;

  return 0;
}
