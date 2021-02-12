package main

import (
	"context"
	"crypto/rand"
	"io"

	"github.com/libp2p/go-libp2p"
	crypto "github.com/libp2p/go-libp2p-core/crypto"
	host "github.com/libp2p/go-libp2p-core/host"
	net "github.com/libp2p/go-libp2p-core/network"
	dht "github.com/libp2p/go-libp2p-kad-dht"

	// routing "github.com/libp2p/go-libp2p-routing"
	tcp "github.com/libp2p/go-tcp-transport"
	log "github.com/sirupsen/logrus"
	ma "github.com/multiformats/go-multiaddr"
)

type ServiceNode struct {
	Host *host.Host
	DHT  *dht.IpfsDHT
	Stop chan bool
}

type netNotifiee struct{}

func (nn *netNotifiee) Connected(n net.Network, c net.Conn) {
	log.Info("Connected to: %s/p2p/%s\n", c.RemoteMultiaddr(), c.RemotePeer().Pretty())
}

func (nn *netNotifiee) Disconnected(n net.Network, v net.Conn)   {}
func (nn *netNotifiee) OpenedStream(n net.Network, v net.Stream) {}
func (nn *netNotifiee) ClosedStream(n net.Network, v net.Stream) {}
func (nn *netNotifiee) Listen(n net.Network, a ma.Multiaddr)      {}
func (nn *netNotifiee) ListenClose(n net.Network, a ma.Multiaddr) {}

func StreamHandler(s net.Stream) {
}

func createHost(ctx context.Context) (host.Host, *dht.IpfsDHT, error) {
	var r io.Reader
	r = rand.Reader
	var d *dht.IpfsDHT
	privKey, _, err := crypto.GenerateKeyPairWithReader(crypto.RSA, 2048, r)
	if err != nil {
		return nil, nil, err
	}
	transport := libp2p.ChainOptions(
		libp2p.Transport(tcp.NewTCPTransport),
	)
	listenAddr := libp2p.ListenAddrStrings(
		"/ip4/0.0.0.0/tcp/4000",
	)

	h, err := libp2p.New(
		ctx,
		transport,
		listenAddr,
		libp2p.Identity(privKey),
		libp2p.DefaultSecurity,
	)
	if err != nil {
		return nil, nil, err
	}
	h.SetStreamHandler("/chat/1.0.0", StreamHandler)
	d, err = dht.New(ctx, h)
	if err != nil {
		return nil, nil, err
	}
	return h, d, nil
}

func CreateNode(h *host.Host, d *dht.IpfsDHT) ServiceNode {
	return ServiceNode{
		Host: h,
		DHT:  d,
		Stop: make(chan bool),
	}
}

func main() {
	ctx := context.Background()
	log.Info("Creating host")
	basicHost, kahdemlia, err := createHost(ctx)
	if err != nil {
		log.Error("Error creating host")
	}
	basicHost.Network().Notify(&netNotifiee{})
	node := CreateNode(&basicHost, kahdemlia)
	log.Info("Host created")
	log.Info("We are: ", basicHost.ID(), basicHost.Addrs())
	<-node.Stop
}
