package main

import (
	"context"
	
	"github.com/libp2p/go-libp2p"
	host "github.com/libp2p/go-libp2p-core/host"
	net "github.com/libp2p/go-libp2p-core/network"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	// routing "github.com/libp2p/go-libp2p-routing"
	log "github.com/sirupsen/logrus"
	ma "github.com/multiformats/go-multiaddr"
	ntraversal "github.com/libp2p-tutorial"
)

type netNotifiee struct{}

func (nn *netNotifiee) Connected(n net.Network, c net.Conn) {
	log.Info("Connected to: %s/p2p/%s\n", c.RemoteMultiaddr(), c.RemotePeer().Pretty())
}

func (nn *netNotifiee) Disconnected(n net.Network, v net.Conn)   {}
func (nn *netNotifiee) OpenedStream(n net.Network, v net.Stream) {}
func (nn *netNotifiee) ClosedStream(n net.Network, v net.Stream) {}
func (nn *netNotifiee) Listen(n net.Network, a ma.Multiaddr)      {}
func (nn *netNotifiee) ListenClose(n net.Network, a ma.Multiaddr) {}

func createHost(ctx context.Context) (host.Host, *dht.IpfsDHT, error) {
	var d *dht.IpfsDHT
	sourceMultiAddr, _ := ma.NewMultiaddr("/ip4/0.0.0.0/tcp/4000")
	h, err := libp2p.New(
		ctx,
		libp2p.ListenAddrs(sourceMultiAddr),
		libp2p.DefaultSecurity,
	)
	if err != nil {
		return nil, nil, err
	}
	var mode dht.ModeOpt = 2
	d, err = dht.New(ctx, h, dht.Mode(mode))
	if err != nil {
		return nil, nil, err
	}
	return h, d, nil
}

func main() {
	ctx := context.Background()
	log.Info("Creating host")
	basicHost, kahdemlia, err := createHost(ctx)
	if err != nil {
		log.Error("Error creating host")
	}
	
	basicHost.Network().Notify(&netNotifiee{})
	node := ntraversal.CreateNode(&basicHost, kahdemlia)
	log.Info("Host created")
	log.Info("We are: ", basicHost.ID(), basicHost.Addrs())
	log.Info("DHT MODE: ", node.DHT.Mode())
	<-node.Stop
}
