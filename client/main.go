package main

import (
	"context"
	
	log "github.com/sirupsen/logrus"
	ntraversal "github.com/libp2p-tutorial"
)

const (
	protocolService = "/traversal/1.0.0"
)

func main() {
	log.Info("creating host")
	ctx := context.Background()
	h, d, err := ntraversal.CreateHost(ctx)
	if err != nil {
		log.Error("Error creating host ", err)
	}
	
	node := ntraversal.CreateNode(&h, d)
	log.Info("Node: ", (*node.Host).ID())
	log.Info("Addrs: ", (*node.Host).Addrs())
	if err = d.Bootstrap(ctx); err != nil {
		log.Error("Error in bootstraping node", err)
	}
	err = node.ConnectToServiceNode(ctx,
		[]string{"/ip4/18.219.31.81/tcp/4000/p2p/QmYrohCZnHXPfSZ17nPyuGFwq1eEhsUnFkN8XTFzFiKeEw"},
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
