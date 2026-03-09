#pragma once

#ifdef HAVE_NATS

#include "SignalServiceClient.h"

#include <atomic>
#include <mutex>
#include <string>
#include <vector>

// Forward declaration for NATS C client
struct __natsConnection;
typedef struct __natsConnection natsConnection;

namespace collector {

class NatsSignalServiceClient : public ISignalServiceClient {
 public:
  explicit NatsSignalServiceClient(const std::string& url,
                                   const std::string& cluster_id);
  ~NatsSignalServiceClient() override;

  void Start() override;
  void Stop() override;
  SignalHandler::Result PushSignals(const SignalStreamMessage& msg) override;

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

#endif  // HAVE_NATS
