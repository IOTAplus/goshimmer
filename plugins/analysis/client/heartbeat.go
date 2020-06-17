package client

import (
	"strings"

	"github.com/iotaledger/goshimmer/packages/metrics"
	"github.com/iotaledger/goshimmer/plugins/analysis/packet"
	"github.com/iotaledger/goshimmer/plugins/autopeering"
	"github.com/iotaledger/goshimmer/plugins/autopeering/local"
	"github.com/iotaledger/hive.go/network"
	"github.com/mr-tron/base58"
)

// EventDispatchers holds the Heartbeat function.
type EventDispatchers struct {
	// Heartbeat defines the Heartbeat function.
	Heartbeat func(heartbeat *packet.Heartbeat)
}

func sendHeartbeat(conn *network.ManagedConnection, hb *packet.Heartbeat) {
	var out strings.Builder
	for _, value := range hb.OutboundIDs {
		out.WriteString(base58.Encode(value))
	}
	var in strings.Builder
	for _, value := range hb.InboundIDs {
		in.WriteString(base58.Encode(value))
	}
	log.Debugw(
		"Heartbeat",
		"nodeID", base58.Encode(hb.OwnID),
		"outboundIDs", out.String(),
		"inboundIDs", in.String(),
	)

	data, err := packet.NewHeartbeatMessage(hb)
	if err != nil {
		log.Info(err, " - heartbeat message skipped")
		return
	}

	connLock.Lock()
	defer connLock.Unlock()
	if _, err = conn.Write(data); err != nil {
		log.Debugw("Error while writing to connection", "Description", err)
	}
	// trigger AnalysisOutboundBytes event
	metrics.Events().AnalysisOutboundBytes.Trigger(uint64(len(data)))
}

func createHeartbeat() *packet.Heartbeat {
	// get own ID
	var nodeID []byte
	if local.GetInstance() != nil {
		// doesn't copy the ID, take care not to modify underlying bytearray!
		nodeID = local.GetInstance().ID().Bytes()
	}

	var outboundIDs [][]byte
	var inboundIDs [][]byte

	// get outboundIDs (chosen neighbors)
	outgoingNeighbors := autopeering.Selection().GetOutgoingNeighbors()
	outboundIDs = make([][]byte, len(outgoingNeighbors))
	for i, neighbor := range outgoingNeighbors {
		// doesn't copy the ID, take care not to modify underlying bytearray!
		outboundIDs[i] = neighbor.ID().Bytes()
	}

	// get inboundIDs (accepted neighbors)
	incomingNeighbors := autopeering.Selection().GetIncomingNeighbors()
	inboundIDs = make([][]byte, len(incomingNeighbors))
	for i, neighbor := range incomingNeighbors {
		// doesn't copy the ID, take care not to modify underlying bytearray!
		inboundIDs[i] = neighbor.ID().Bytes()
	}

	return &packet.Heartbeat{OwnID: nodeID, OutboundIDs: outboundIDs, InboundIDs: inboundIDs}
}
