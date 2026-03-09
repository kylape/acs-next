package streams

// Stream names for ACS broker
const (
	ProcessEventsStream = "PROCESS_EVENTS"
	NetworkEventsStream = "NETWORK_EVENTS"
)

// Subject patterns for publishing events
const (
	// ProcessEventsSubjectPattern is "acs.<cluster_id>.process-events"
	ProcessEventsSubjectPattern = "acs.%s.process-events"
	// NetworkEventsSubjectPattern is "acs.<cluster_id>.network-events"
	NetworkEventsSubjectPattern = "acs.%s.network-events"
)
