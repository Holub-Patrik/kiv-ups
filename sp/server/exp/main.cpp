#include "../src/CircularBufferQueue.hpp"

#include <chrono>
#include <iostream>
#include <thread>

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

  return 0;
}
