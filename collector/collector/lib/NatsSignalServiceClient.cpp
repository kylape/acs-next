#ifdef HAVE_NATS

#include "NatsSignalServiceClient.h"

#include <nats/nats.h>

#include "Logging.h"

namespace collector {

NatsSignalServiceClient::NatsSignalServiceClient(const std::string& url,
                                                 const std::string& cluster_id)
    : url_(url), cluster_id_(cluster_id) {}

NatsSignalServiceClient::~NatsSignalServiceClient() {
  Stop();
}

void NatsSignalServiceClient::Start() {
  std::lock_guard<std::mutex> lock(mutex_);
  if (connected_.load()) {
    return;
  }

  if (!Connect()) {
    CLOG(ERROR) << "Failed to connect to NATS broker at " << url_;
    return;
  }

  CLOG(INFO) << "Connected to NATS broker at " << url_;
}

void NatsSignalServiceClient::Stop() {
  std::lock_guard<std::mutex> lock(mutex_);
  Disconnect();
}

bool NatsSignalServiceClient::Connect() {
  natsStatus status = natsConnection_ConnectTo(&conn_, url_.c_str());
  if (status != NATS_OK) {
    CLOG(ERROR) << "NATS connection failed: " << natsStatus_GetText(status);
    return false;
  }
  connected_.store(true);
  return true;
}

void NatsSignalServiceClient::Disconnect() {
  if (conn_ != nullptr) {
    natsConnection_Close(conn_);
    natsConnection_Destroy(conn_);
    conn_ = nullptr;
  }
  connected_.store(false);
}

std::string NatsSignalServiceClient::BuildSubject(const std::string& event_type) const {
  return "acs." + cluster_id_ + "." + event_type;
}

SignalHandler::Result NatsSignalServiceClient::PushSignals(const SignalStreamMessage& msg) {
  if (!connected_.load()) {
    CLOG_THROTTLED(ERROR, std::chrono::seconds(10))
        << "NATS connection not established";
    return SignalHandler::ERROR;
  }

  // Serialize the protobuf message
  std::string serialized;
  if (!msg.SerializeToString(&serialized)) {
    CLOG(ERROR) << "Failed to serialize signal message";
    return SignalHandler::ERROR;
  }

  // Determine subject based on message content
  std::string subject;
  if (msg.has_signal()) {
    subject = BuildSubject("process-events");
  } else {
    subject = BuildSubject("network-events");
  }

  // Publish to NATS
  natsStatus status = natsConnection_Publish(
      conn_,
      subject.c_str(),
      serialized.data(),
      static_cast<int>(serialized.size()));

  if (status != NATS_OK) {
    CLOG(ERROR) << "NATS publish failed: " << natsStatus_GetText(status);

    // Check if connection is still valid
    if (natsConnection_IsClosed(conn_)) {
      connected_.store(false);
      CLOG(ERROR) << "NATS connection closed, will attempt reconnect";
    }
    return SignalHandler::ERROR;
  }

  return SignalHandler::PROCESSED;
}

}  // namespace collector

#endif  // HAVE_NATS
