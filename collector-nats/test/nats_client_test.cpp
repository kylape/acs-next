#include "NatsClient.h"

#include <cassert>
#include <cstdio>
#include <cstring>
#include <string>
#include <thread>
#include <vector>

// ---- Unit tests (no server required) ----

void test_build_subject() {
  collector::NatsClient client("nats://localhost:4222", "test-cluster");
  // We can't call BuildSubject directly (private), but we can test
  // via PublishProcessEvent/PublishNetworkEvent with a disconnected client.
  // These should return false when disconnected.
  assert(!client.IsConnected());
  assert(!client.PublishProcessEvent("data", 4));
  assert(!client.PublishNetworkEvent("data", 4));
  printf("PASS: test_build_subject\n");
}

void test_lifecycle() {
  collector::NatsClient client("nats://localhost:4222", "test-cluster");
  assert(!client.IsConnected());
  // Stop on unconnected client should not crash
  client.Stop();
  assert(!client.IsConnected());
  printf("PASS: test_lifecycle\n");
}

void test_publish_disconnected() {
  collector::NatsClient client("nats://localhost:4222", "test-cluster");
  const char* data = "test payload";
  assert(!client.Publish("test.subject", data, strlen(data)));
  printf("PASS: test_publish_disconnected\n");
}

// ---- Integration tests (require running NATS server) ----

void test_connect_and_publish(const std::string& nats_url) {
  collector::NatsClient client(nats_url, "integration-test");

  assert(client.Start());
  assert(client.IsConnected());

  const char* process_data = R"({"pid":1234,"name":"test-process"})";
  assert(client.PublishProcessEvent(process_data, strlen(process_data)));

  const char* network_data = R"({"src_ip":"10.0.0.1","dst_ip":"10.0.0.2"})";
  assert(client.PublishNetworkEvent(network_data, strlen(network_data)));

  client.Stop();
  assert(!client.IsConnected());

  printf("PASS: test_connect_and_publish\n");
}

void test_reconnect(const std::string& nats_url) {
  collector::NatsClient client(nats_url, "reconnect-test");

  assert(client.Start());
  assert(client.IsConnected());

  client.Stop();
  assert(!client.IsConnected());

  // Reconnect
  assert(client.Start());
  assert(client.IsConnected());

  const char* data = "reconnect-test-data";
  assert(client.Publish("acs.reconnect-test.process-events", data, strlen(data)));

  client.Stop();
  printf("PASS: test_reconnect\n");
}

void test_concurrent_publish(const std::string& nats_url) {
  collector::NatsClient client(nats_url, "concurrent-test");
  assert(client.Start());

  std::vector<std::thread> threads;
  std::atomic<int> failures{0};

  for (int i = 0; i < 4; i++) {
    threads.emplace_back([&client, &failures, i]() {
      for (int j = 0; j < 100; j++) {
        std::string payload = "{\"thread\":" + std::to_string(i) +
                              ",\"seq\":" + std::to_string(j) + "}";
        if (!client.PublishProcessEvent(payload.data(), payload.size())) {
          failures++;
        }
      }
    });
  }

  for (auto& t : threads) {
    t.join();
  }

  assert(failures.load() == 0);
  client.Stop();
  printf("PASS: test_concurrent_publish\n");
}

int main(int argc, char* argv[]) {
  printf("=== Unit tests (no server required) ===\n");
  test_build_subject();
  test_lifecycle();
  test_publish_disconnected();

  // Integration tests require NATS_URL env var
  const char* nats_url_env = std::getenv("NATS_URL");
  if (nats_url_env != nullptr && strlen(nats_url_env) > 0) {
    std::string nats_url(nats_url_env);
    printf("\n=== Integration tests (NATS_URL=%s) ===\n", nats_url.c_str());
    test_connect_and_publish(nats_url);
    test_reconnect(nats_url);
    test_concurrent_publish(nats_url);
  } else {
    printf("\nSkipping integration tests (set NATS_URL to enable)\n");
  }

  printf("\nAll tests passed!\n");
  return 0;
}
