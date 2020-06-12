package metrics

import "github.com/iotaledger/hive.go/events"

// CollectionEvents defines the events fot the metrics package
type CollectionEvents struct {
	FPCInboundBytes  *events.Event
	FPCOutboundBytes *events.Event
	CPUUsage         *events.Event
	MemUsage         *events.Event

	GossipPacketDropped *events.Event
	DBSize              *events.Event
	Synced              *events.Event
}

func uint64Caller(handler interface{}, params ...interface{}) {
	handler.(func(uint64))(params[0].(uint64))
}

func float64Caller(handler interface{}, params ...interface{}) {
	handler.(func(float64))(params[0].(float64))
}

func boolCaller(handler interface{}, params ...interface{}) {
	handler.(func(bool))(params[0].(bool))
}
