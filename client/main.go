package main

import (
	"context"
	"crypto/rand"
	"io"
	"sync"
	"bufio"

	cid "github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/host"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	peer "github.com/libp2p/go-libp2p-peer"
	peerstore "github.com/libp2p/go-libp2p-peerstore"
	"github.com/libp2p/go-tcp-transport"
	ma "github.com/multiformats/go-multiaddr"
	net "github.com/libp2p/go-libp2p-core/network"
	mh "github.com/multiformats/go-multihash"
	// discovery "github.com/libp2p/go-libp2p-discovery"
	log "github.com/sirupsen/logrus"
	"github.com/libp2p/go-libp2p-core/protocol"
)

type Node struct {
	Host           *host.Host
	DHT            *dht.IpfsDHT
	Stop           chan bool
	serviceNodes   []peer.ID
	bootstrapPeers StramContainer
}

type StramContainer struct {
	mtx      *sync.Mutex
	peerList map[peer.ID]*StreamWrapper
}

type StreamWrapper struct {
	rw *bufio.ReadWriter
}

const (
	protocolService = "/traversal/1.0.0"
)

func createHost(ctx context.Context) (host.Host, *dht.IpfsDHT, error) {
	var r io.Reader
	r = rand.Reader

	priv, _, err := crypto.GenerateKeyPairWithReader(crypto.RSA, 2048, r)
	if err != nil {
		return nil, nil, err
	}
	transport := libp2p.ChainOptions(
		libp2p.Transport(tcp.NewTCPTransport),
	)
	listener := libp2p.ListenAddrStrings("/ip4/0.0.0.0/tcp/0")
	h, err := libp2p.New(
		ctx,
		transport,
		listener,
		libp2p.DefaultSecurity,
		libp2p.Identity(priv),
	)
	if err != nil {
		return nil, nil, err
	}
	h.SetStreamHandler(protocol.ID("/chat/1.0.0"), StreamHandler)
	idht, err := dht.New(ctx, h)
	if err != nil {
		return nil, nil, err
	}
	return h, idht, nil
}

func StreamHandler(s net.Stream) {

}

func CreateNode(h *host.Host, d *dht.IpfsDHT) *Node {
	sc := StramContainer{
		mtx:      &sync.Mutex{},
		peerList: make(map[peer.ID]*StreamWrapper),
	}
	node := Node{
		Host:           h,
		DHT:            d,
		Stop:           make(chan bool),
		bootstrapPeers: sc,
		serviceNodes:   make([]peer.ID, 0),
	}

	return &node
}

func (node *Node) setStreamWrapper(s net.Stream) {
	rw := bufio.NewReadWriter(bufio.NewReader(s), bufio.NewWriter(s))
	sm := &StreamWrapper{
		rw: rw,
	}
	node.bootstrapPeers.mtx.Lock()
	node.bootstrapPeers.peerList[s.Conn().RemotePeer()] = sm
	node.bootstrapPeers.mtx.Unlock()
}

func (node *Node) ConnectToServiceNode(ctx context.Context, list []string) error {
	for _, peerAddr := range list {
		addr := ma.StringCast(peerAddr)
		peerInfo, _ := peerstore.InfoFromP2pAddr(addr)
		(*node.Host).Peerstore().AddAddrs(peerInfo.ID, peerInfo.Addrs, peerstore.PermanentAddrTTL)
		log.Info("Connecting to : ", peerInfo.ID)
		if s, err := (*node.Host).NewStream(ctx, peerInfo.ID, protocol.ID("/chat/1.0.0")); err != nil {
			log.Warn("Error connecting to service node ", err)
		} else {
			log.Info("Connection established with bootstrap node: ", *peerInfo)
			node.setStreamWrapper(s)
			node.serviceNodes = append(node.serviceNodes, peerInfo.ID)
		}
	}
	return nil
}

func (node *Node) SetupDescovery(ctx context.Context, rendezvous string) error {
	v1b := cid.V1Builder{Codec: cid.Raw, MhType: mh.SHA2_256}
	rendezvousPoint, _ := v1b.Sum([]byte(rendezvous))
	err := node.DHT.Provide(ctx, rendezvousPoint, true)
	if err != nil {
		return err
	}

	pis, err := node.DHT.FindProviders(ctx, rendezvousPoint)
	if err != nil {
		return err
	}
	log.Info("FOUND ", len(pis))

	return nil
}

func main() {
	log.Info("creating host")
	ctx := context.Background()
	h, d, err := createHost(ctx)
	if err != nil {
		log.Error("Error creating host ", err)
	}
	node := CreateNode(&h, d)
	log.Info("Node: ", (*node.Host).ID())
	log.Info("Addrs: ", (*node.Host).Addrs())

	err = node.ConnectToServiceNode(ctx,
		[]string{"/ip4/192.168.0.108/tcp/4000/p2p/QmUhQkZ83VENW14o5SvkHfddKnVD2znbnrU4ezxQ2VpDdS"},
	)
	if err != nil {
		log.Error("Error in connecting to service node", err)
	}
	err = node.SetupDescovery(ctx, "rendezvous")
	if err != nil {
		log.Error("Error in setting discovery", err)
	}
	<-node.Stop
}
