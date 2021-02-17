package ntraversal

import (
	"bufio"
	"context"
	"fmt"
	"sync"
	"strings"

	ggio "github.com/gogo/protobuf/io"
	cid "github.com/ipfs/go-cid"
	"github.com/libp2p-tutorial/proto"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/host"
	net "github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/protocol"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	peer "github.com/libp2p/go-libp2p-peer"
	peerstore "github.com/libp2p/go-libp2p-peerstore"
	swarm "github.com/libp2p/go-libp2p-swarm"
	ma "github.com/multiformats/go-multiaddr"
	mh "github.com/multiformats/go-multihash"
	log "github.com/sirupsen/logrus"
	tcp "github.com/libp2p/go-tcp-transport"
)

type PackageWPeer struct {
	peer   peer.ID
	packet *proto.Message
}

type Node struct {
	Host           *host.Host
	DHT            *dht.IpfsDHT
	Stop           chan bool
	serviceNodes   []peer.ID
	bootstrapPeers StramContainer
	send           chan PackageWPeer
	receive        chan PackageWPeer
}

func CreateHost(ctx context.Context) (host.Host, *dht.IpfsDHT, error) {
	transport := libp2p.Transport(tcp.NewTCPTransport)
	addr := libp2p.ListenAddrStrings("/ip4/0.0.0.0/tcp/0")
	h, err := libp2p.New(
		ctx,
		transport,
		addr,
		libp2p.DefaultSecurity,
		// libp2p.Identity(priv),
	)
	if err != nil {
		return nil, nil, err
	}
	var mode dht.ModeOpt = 2
	idht, err := dht.New(ctx, h, dht.Mode(mode))
	if err != nil {
		return nil, nil, err
	}
	return h, idht, nil
}

func (node *Node) StreamHandler(s net.Stream) {
	log.Info("Connected to ", s.Conn().RemotePeer())
	node.setStreamWrapper(s)
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
		send:           make(chan PackageWPeer, 10),
		receive:        make(chan PackageWPeer, 10),
		bootstrapPeers: sc,
		serviceNodes:   make([]peer.ID, 0),
	}
	(*h).SetStreamHandler(protocol.ID("/chat/1.0.0"), node.StreamHandler)
	go node.MsgHandler()
	return &node
}

func (node *Node) setStreamWrapper(s net.Stream) {
	rw := bufio.NewWriter(s)

	r := ggio.NewDelimitedReader(s, 1<<20)
	w := ggio.NewDelimitedWriter(rw)

	sm := &StreamWrapper{
		bw: rw,
		r:  &r,
		w:  &w,
		s:  &s,
	}
	node.bootstrapPeers.mtx.Lock()
	node.bootstrapPeers.peerList[s.Conn().RemotePeer()] = sm
	node.bootstrapPeers.mtx.Unlock()

	go sm.readMsg(node.receive)
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

func (node *Node) MsgHandler() {
	for {
		select {
		case i := <-node.receive:
			log.Info("incoming packet")
			switch i.packet.Type {
			case proto.Message_CONNECTION_REQUEST:
				go node.HandleConnectionRequest(i)
			case proto.Message_HOLE_PUNCH_REQUEST:
				go node.HandleHolePunchRequest(i)
			}
		case o := <-node.send:
			log.Info("Sending packet")
			go node.bootstrapPeers.peerList[o.peer].writeMsg(o.packet)
		}
	}
}

func (node *Node) HandleConnectionRequest(m PackageWPeer) {
	id, _ := peer.IDHexDecode(string(m.packet.PeerID.Id))

	pi, err := node.findPeerInfo(m.peer)
	if err != nil {
		log.Error(err)
		return
	}
	node.sendPunchRequest(id, pi)
}

func (node *Node) findPeerInfo(p peer.ID) ([]byte, error) {
	pi, err := node.DHT.FindPeer(context.Background(), p)
	if err != nil {
		return nil, fmt.Errorf("Cound not find peer")
	}
	piPublic := peerstore.PeerInfo{}
	for _, addr := range pi.Addrs {
		if strings.Contains(addr.String(), "127.") ||
			strings.Contains(addr.String(), "192.") ||
			strings.Contains(addr.String(), "10.") ||
			strings.Contains(addr.String(), "p2p-circuit") {
			continue
		}

		piPublic.Addrs = append(piPublic.Addrs, addr)
	}
	piPublic.ID = pi.ID
	data, err := piPublic.MarshalJSON()
	if err != nil {
		log.Error(err)
		return nil, fmt.Errorf("Can not marshal")
	}
	return data, nil
}

func (node *Node) HandleHolePunchRequest(m PackageWPeer) {
	pi := peerstore.PeerInfo{}
	pi.UnmarshalJSON(m.packet.PeerInfo.Info)
	log.Info("Got a punch request", pi)

	cnt := 3
	var err error
	for i := 0; i < cnt; i++ {
		err = (*node.Host).Connect(context.Background(), pi)
		if err == nil {
			log.Info(i+1, "trial succeeded: ", err)
			break
		}
		(*node.Host).Network().(*swarm.Swarm).Backoff().Clear(pi.ID)
		log.Error(err)
	}
	if err != nil {
		log.Error("All attempts failed")
	}
}

func (node *Node) sendPunchRequest(to peer.ID, pi []byte) {
	node.send <- PackageWPeer{
		peer: to,
		packet: &proto.Message{
			Type: proto.Message_HOLE_PUNCH_REQUEST,
			PeerInfo: &proto.Message_PeerInfo{
				Info: pi,
			},
		},
	}
}

func (node *Node) ConnectWithHolePunching(ctx context.Context, p peer.ID) (chan error, error) {
	if len(node.serviceNodes) == 0 {
		log.Error("not connected to any service node")
		return nil, fmt.Errorf("Not connected to any serivice node")
	}

	log.Info("Connecting to peer: ", p)

	res := make(chan error)
	
	node.send <- PackageWPeer{
		peer: node.serviceNodes[0],
		packet: &proto.Message{
			Type: proto.Message_CONNECTION_REQUEST,
			PeerID: &proto.Message_PeerID{
				Id: []byte(peer.IDHexEncode(p)),
			},
		},
	}

	return res, nil
}

func (node *Node) SetupDescovery(ctx context.Context, rendezvous string) error {
	v1b := cid.V1Builder{Codec: cid.Raw, MhType: mh.SHA2_256}
	rendezvousPoint, _ := v1b.Sum([]byte(rendezvous))
	cnt := 3
	for i := 0; i < cnt; i++ {
		err := node.DHT.Provide(ctx, rendezvousPoint, true)
		if err != nil {
			log.Warn("Did not find any peer retrying")
			continue
		}
		break
	}

	pis, err := node.DHT.FindProviders(ctx, rendezvousPoint)
	if err != nil {
		log.Warn(err)
	}
	log.Info("FOUND ", len(pis))

	for _, pi := range pis {
		if pi.ID != (*node.Host).ID() {
			if true {
				// log.Info("Error connecting peer ", err)
				// log.Info("COnnecting with hole punching")
				_, err = node.ConnectWithHolePunching(ctx, pi.ID)
				if err != nil {
					log.Error(err)
				} else {
					log.Info("Connected")
				}
			} else {
				log.Info("Host connected", pi)
			}
		}
	}
	return nil
}
