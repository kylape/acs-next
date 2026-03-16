#ifdef HAVE_NATS

#include "NatsSignalServiceClient.h"

#include <nats/nats.h>

#include "Logging.h"
#include "storage/process_indicator.pb.h"
#include "acs/broker/v1/process.pb.h"
#include "acs/broker/v1/network.pb.h"

namespace collector {

namespace {

// Convert StackRox storage::ProcessSignal to ACS ProcessEvent
acs::broker::v1::ProcessEvent ConvertToProcessEvent(
    const storage::ProcessSignal& signal,
    const std::string& cluster_id) {
  acs::broker::v1::ProcessEvent event;

  event.set_cluster_id(cluster_id);

  if (signal.has_time()) {
    *event.mutable_timestamp() = signal.time();
  }

  event.set_container_id(signal.container_id());
  event.set_id(signal.id());
  event.set_name(signal.name());
  event.set_exec_file_path(signal.exec_file_path());
  event.set_args(signal.args());
  event.set_pid(signal.pid());
  event.set_uid(signal.uid());
  event.set_gid(signal.gid());

  for (const auto& lineage : signal.lineage_info()) {
    auto* li = event.add_lineage();
    li->set_parent_exec_file_path(lineage.parent_exec_file_path());
    li->set_parent_uid(lineage.parent_uid());
  }

  return event;
}

}  // namespace

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

  std::string serialized;
  std::string subject;

  // Convert StackRox messages to ACS proto format
  if (msg.has_signal() && msg.signal().has_process_signal()) {
    auto event = ConvertToProcessEvent(msg.signal().process_signal(), cluster_id_);
    if (!event.SerializeToString(&serialized)) {
      CLOG(ERROR) << "Failed to serialize ProcessEvent";
      return SignalHandler::ERROR;
    }
    subject = BuildSubject("process-events");
  } else {
    // TODO: Handle network events when needed
    return SignalHandler::PROCESSED;
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
