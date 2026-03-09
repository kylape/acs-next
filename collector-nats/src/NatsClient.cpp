#include "NatsClient.h"

#include <nats/nats.h>

#include <cstdio>

namespace collector {

NatsClient::NatsClient(const std::string& url, const std::string& cluster_id)
    : url_(url), cluster_id_(cluster_id) {}

NatsClient::~NatsClient() {
  Stop();
}

bool NatsClient::Start() {
  std::lock_guard<std::mutex> lock(mutex_);
  if (connected_.load()) {
    return true;
  }

  if (!Connect()) {
    fprintf(stderr, "Failed to connect to NATS broker at %s\n", url_.c_str());
    return false;
  }

  fprintf(stdout, "Connected to NATS broker at %s\n", url_.c_str());
  return true;
}

void NatsClient::Stop() {
  std::lock_guard<std::mutex> lock(mutex_);
  Disconnect();
}

bool NatsClient::Connect() {
  natsStatus status = natsConnection_ConnectTo(&conn_, url_.c_str());
  if (status != NATS_OK) {
    fprintf(stderr, "NATS connection failed: %s\n", natsStatus_GetText(status));
    return false;
  }
  connected_.store(true);
  return true;
}

void NatsClient::Disconnect() {
  if (conn_ != nullptr) {
    natsConnection_Close(conn_);
    natsConnection_Destroy(conn_);
    conn_ = nullptr;
  }
  connected_.store(false);
}

std::string NatsClient::BuildSubject(const std::string& event_type) const {
  return "acs." + cluster_id_ + "." + event_type;
}

bool NatsClient::Publish(const std::string& subject, const void* data, int len) {
  if (!connected_.load()) {
    return false;
  }

  natsStatus status = natsConnection_Publish(
      conn_, subject.c_str(),
      static_cast<const void*>(data), len);

  if (status != NATS_OK) {
    fprintf(stderr, "NATS publish failed: %s\n", natsStatus_GetText(status));

    if (natsConnection_IsClosed(conn_)) {
      connected_.store(false);
    }
    return false;
  }

  return true;
}

bool NatsClient::PublishProcessEvent(const void* data, int len) {
  return Publish(BuildSubject("process-events"), data, len);
}

bool NatsClient::PublishNetworkEvent(const void* data, int len) {
  return Publish(BuildSubject("network-events"), data, len);
}

}  // namespace collector
