package autopeering

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"

	"github.com/iotaledger/goshimmer/plugins/autopeering/local"
	"github.com/iotaledger/goshimmer/plugins/config"
	"github.com/iotaledger/hive.go/autopeering/discover"
	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/autopeering/peer/service"
	"github.com/iotaledger/hive.go/autopeering/selection"
	"github.com/iotaledger/hive.go/autopeering/server"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/logger"
	"github.com/mr-tron/base58"
)

// autopeering constants
const (
	ProtocolVersion = 0 // update on protocol changes
	NetworkVersion  = 5 // update on network changes
)

var (
	// ErrParsingMasterNode is returned for an invalid master node.
	ErrParsingMasterNode = errors.New("cannot parse master node")

	// Conn contains the network connection.
	Conn *NetConnMetric
)

var (
	// the peer discovery protocol
	peerDisc     *discover.Protocol
	peerDiscOnce sync.Once

	// the peer selection protocol
	peerSel     *selection.Protocol
	peerSelOnce sync.Once

	// block until the peering server has been started
	srvBarrier = struct {
		once sync.Once
		c    chan *server.Server
	}{c: make(chan *server.Server, 1)}
)

// Discovery returns the peer discovery instance.
func Discovery() *discover.Protocol {
	peerDiscOnce.Do(createPeerDisc)
	return peerDisc
}

// Selection returns the neighbor selection instance.
func Selection() *selection.Protocol {
	peerSelOnce.Do(createPeerSel)
	return peerSel
}

// BindAddress returns the string form of the autopeering bind address.
func BindAddress() string {
	peering := local.GetInstance().Services().Get(service.PeeringKey)
	host := config.Node().GetString(local.CfgBind)
	port := strconv.Itoa(peering.Port())
	return net.JoinHostPort(host, port)
}

// StartSelection starts the neighbor selection process.
// It blocks until the peer discovery has been started. Multiple calls of StartSelection are ignored.
func StartSelection() {
	srvBarrier.once.Do(func() {
		srv := <-srvBarrier.c
		close(srvBarrier.c)

		Selection().Start(srv)
	})
}

func createPeerDisc() {
	// assure that the logger is available
	log := logger.NewLogger(PluginName).Named("disc")

	masterPeers, err := parseEntryNodes()
	if err != nil {
		log.Errorf("Invalid entry nodes; ignoring: %v", err)
	}
	log.Debugf("Master peers: %v", masterPeers)

	peerDisc = discover.New(local.GetInstance(), ProtocolVersion, NetworkVersion,
		discover.Logger(log),
		discover.MasterPeers(masterPeers),
	)
}

func createPeerSel() {
	// assure that the logger is available
	log := logger.NewLogger(PluginName).Named("sel")

	peerSel = selection.New(local.GetInstance(), Discovery(),
		selection.Logger(log),
		selection.NeighborValidator(selection.ValidatorFunc(isValidNeighbor)),
	)
}

// isValidNeighbor checks whether a peer is a valid neighbor.
func isValidNeighbor(p *peer.Peer) bool {
	// gossip must be supported
	gossipService := p.Services().Get(service.GossipKey)
	if gossipService == nil {
		return false
	}
	// gossip service must be valid
	if gossipService.Network() != "tcp" || gossipService.Port() < 0 || gossipService.Port() > 65535 {
		return false
	}
	return true
}

func start(shutdownSignal <-chan struct{}) {
	defer log.Info("Stopping " + PluginName + " ... done")

	lPeer := local.GetInstance()
	peering := lPeer.Services().Get(service.PeeringKey)

	// resolve the bind address
	localAddr, err := net.ResolveUDPAddr(peering.Network(), BindAddress())
	if err != nil {
		log.Fatalf("Error resolving %s: %v", local.CfgBind, err)
	}

	conn, err := net.ListenUDP(peering.Network(), localAddr)
	if err != nil {
		log.Fatalf("Error listening: %v", err)
	}
	defer conn.Close()

	Conn = &NetConnMetric{UDPConn: conn}

	// start a server doing peerDisc and peering
	srv := server.Serve(lPeer, Conn, log.Named("srv"), Discovery(), Selection())
	defer srv.Close()

	// start the peer discovery on that connection
	Discovery().Start(srv)
	srvBarrier.c <- srv

	log.Infof("%s started: ID=%s Address=%s/%s", PluginName, lPeer.ID(), localAddr.String(), localAddr.Network())

	<-shutdownSignal

	log.Infof("Stopping %s ...", PluginName)

	Discovery().Close()
	Selection().Close()

	lPeer.Database().Close()
}

func parseEntryNodes() (result []*peer.Peer, err error) {
	for _, entryNodeDefinition := range config.Node().GetStringSlice(CfgEntryNodes) {
		if entryNodeDefinition == "" {
			continue
		}

		parts := strings.Split(entryNodeDefinition, "@")
		if len(parts) != 2 {
			return nil, fmt.Errorf("%w: master node parts must be 2, is %d", ErrParsingMasterNode, len(parts))
		}
		pubKey, err := base58.Decode(parts[0])
		if err != nil {
			return nil, fmt.Errorf("%w: invalid public key: %s", ErrParsingMasterNode, err)
		}
		addr, err := net.ResolveUDPAddr("udp", parts[1])
		if err != nil {
			return nil, fmt.Errorf("%w: host cannot be resolved: %s", ErrParsingMasterNode, err)
		}
		publicKey, _, err := ed25519.PublicKeyFromBytes(pubKey)
		if err != nil {
			return nil, err
		}

		services := service.New()
		services.Update(service.PeeringKey, addr.Network(), addr.Port)

		result = append(result, peer.NewPeer(identity.New(publicKey), addr.IP, services))
	}

	return result, nil
}
