package metrics

import (
	"github.com/iotaledger/goshimmer/plugins/gossip"
	"github.com/iotaledger/hive.go/identity"
	"go.uber.org/atomic"
)

var (
	_FPCInboundBytes  atomic.Uint64
	_FPCOutboundBytes atomic.Uint64

	previousNeighbors = make(map[identity.ID]gossipTrafficMetric)
	gossipOldTx       uint32
	gossipOldRx       uint32

	analysisOutboundBytes atomic.Uint64
)

func FPCInboundBytes() uint64 {
	return _FPCInboundBytes.Load()
}

func FPCOutboundBytes() uint64 {
	return _FPCOutboundBytes.Load()
}

func AnalysisOutboundBytes() uint64 {
	return analysisOutboundBytes.Load()
}

type gossipTrafficMetric struct {
	BytesRead    uint32
	BytesWritten uint32
}

func gossipCurrentTraffic() (g gossipTrafficMetric) {
	neighbors := gossip.Manager().AllNeighbors()

	currentNeighbors := make(map[identity.ID]bool)
	for _, neighbor := range neighbors {
		currentNeighbors[neighbor.ID()] = true

		if _, ok := previousNeighbors[neighbor.ID()]; !ok {
			previousNeighbors[neighbor.ID()] = gossipTrafficMetric{
				BytesRead:    neighbor.BytesRead(),
				BytesWritten: neighbor.BytesWritten(),
			}
		}

		g.BytesRead += neighbor.BytesRead()
		g.BytesWritten += neighbor.BytesWritten()
	}

	for prevNeighbor := range previousNeighbors {
		if _, ok := currentNeighbors[prevNeighbor]; !ok {
			gossipOldRx += previousNeighbors[prevNeighbor].BytesRead
			gossipOldTx += previousNeighbors[prevNeighbor].BytesWritten
			delete(currentNeighbors, prevNeighbor)
		}
	}

	g.BytesRead += gossipOldRx
	g.BytesWritten += gossipOldTx

	return
}