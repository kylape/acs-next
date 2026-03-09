#pragma once

#include <atomic>
#include <mutex>
#include <string>
#include <vector>

struct __natsConnection;
typedef struct __natsConnection natsConnection;

namespace collector {

class NatsClient {
 public:
  explicit NatsClient(const std::string& url, const std::string& cluster_id);
  ~NatsClient();

  NatsClient(const NatsClient&) = delete;
  NatsClient& operator=(const NatsClient&) = delete;

  bool Start();
  void Stop();
  bool IsConnected() const { return connected_.load(); }

  // Publish raw bytes to a subject
  bool Publish(const std::string& subject, const void* data, int len);

  // Publish to the appropriate stream subject
  bool PublishProcessEvent(const void* data, int len);
  bool PublishNetworkEvent(const void* data, int len);

  std::string ClientURL() const { return url_; }

 private:
  std::string url_;
  std::string cluster_id_;
  natsConnection* conn_ = nullptr;
  std::atomic<bool> connected_{false};
  mutable std::mutex mutex_;

  std::string BuildSubject(const std::string& event_type) const;
  bool Connect();
  void Disconnect();
};

}  // namespace collector
