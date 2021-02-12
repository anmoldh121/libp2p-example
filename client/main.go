package main

import (
	"context"
	"crypto/rand"
	"io"
	"sync"

	cid "github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-kad-dht"
	peerstore "github.com/libp2p/go-libp2p-peerstore"
	// routing "github.com/libp2p/go-libp2p-routing"
	"github.com/libp2p/go-tcp-transport"
	ma "github.com/multiformats/go-multiaddr"
	mh "github.com/multiformats/go-multihash"
	log "github.com/sirupsen/logrus"
)

type Node struct {
	Host *host.Host
	DHT  *dht.IpfsDHT
	Stop chan bool
}

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
		libp2p.NATPortMap(),
	)
	if err != nil {
		return nil, nil, err
	}
	idht, err := dht.New(ctx, h)
	if err != nil {
		return nil, nil, err
	}
	return h, idht, nil
}

func CreateNode(h *host.Host, d *dht.IpfsDHT) Node {
	return Node{
		Host: h,
		DHT:  d,
		Stop: make(chan bool),
	}
}

func (node *Node) ConnectToServiceNode(ctx context.Context, list []string) error {
	var wg sync.WaitGroup
	for _, peerStr := range list {
		in := ma.StringCast(peerStr)
		peerInfo, err := peerstore.InfoFromP2pAddr(in)
		if err != nil {
			return err
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := (*node.Host).Connect(ctx, *peerInfo); err != nil {
				log.Warn(err)
			} else {
				log.Info("Connected to service node ", *peerInfo)
			}
		}()
	}
	wg.Wait()
	return nil
}

func (node *Node) SetupDescovery(ctx context.Context, rendezvous string) error {
	log.Info("Announcing")
	v1b := cid.V1Builder{Codec: cid.Raw, MhType: mh.SHA2_256}
	rendezvousPoint, err := v1b.Sum([]byte(rendezvous))
	if err != nil {
		return err
	}
	err = node.DHT.Provide(ctx, rendezvousPoint, true)
	if err != nil {
		return err
	}

	if err = node.DHT.Bootstrap(ctx); err != nil {
		log.Warn("Failed to bootstrap ", err)
	}

	log.Info("Announcement complete")
	log.Info("Searching peers")
	pis, err := node.DHT.FindProviders(ctx, rendezvousPoint)
	if err != nil {
		log.Warn("No peer found")
	}
	log.Info("FOUND", len(pis))
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
	log.Info("Host created We Are: ", (*node.Host).ID())
	log.Info("Addrs: ", (*node.Host).Addrs())

	err = node.ConnectToServiceNode(ctx, []string{"/ip4/192.168.0.108/tcp/4000/p2p/QmPSnf4n8tKqNVMowkTL9fQnsmRAeEbFopMvpWkuKHmQVm"})
	if err != nil {
		log.Error("Error in connecting to service node", err)
	}
	err = node.SetupDescovery(ctx, "rendezvous")
	if err != nil {
		log.Error("Error in setting discovery", err)
	}
	<-node.Stop
}
