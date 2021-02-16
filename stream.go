package ntraversal

import (
	"bufio"
	"sync"

	ggio "github.com/gogo/protobuf/io"
	pb "github.com/golang/protobuf/proto"
	"github.com/libp2p-tutorial/proto"
	inet "github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
)

type StramContainer struct {
	mtx      *sync.Mutex
	peerList map[peer.ID]*StreamWrapper
}

type StreamWrapper struct {
	bw *bufio.Writer
	s  *inet.Stream
	r  *ggio.ReadCloser
	w  *ggio.WriteCloser
}

func (sw StreamWrapper) writeMsg(msg pb.Message) error {
	w := *sw.w
	bw := sw.bw

	err := w.WriteMsg(msg)
	if err != nil {
		return err
	}

	return bw.Flush()
}

func (sw StreamWrapper) readMsg(incoming chan PackageWPeer) error {
	r := *sw.r
	s := *sw.s

	msg := &proto.Message{}

	for {
		err := r.ReadMsg(msg)
		if err != nil {
			return err
		}
		incoming <- PackageWPeer{
			peer:   s.Conn().RemotePeer(),
			packet: msg,
		}
	}
}
