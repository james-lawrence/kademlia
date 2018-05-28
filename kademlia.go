package kademlia

import (
	"context"
	"crypto/sha1"
	"net"

	"github.com/james-lawrence/kademlia/protocol"
)

// ContentAddressable returns the key of the provided data.
// uses sha1.
func ContentAddressable(data []byte) []byte {
	sha := sha1.Sum(data)
	return sha[:]
}

type server struct {
	*DHT
}

func (t server) Ping(ctx context.Context, req *protocol.PingRequest) (*protocol.PingResponse, error) {
	return &protocol.PingResponse{
		Sender:   fromNetworkNode(t.ht.Self),
		Receiver: req.Sender,
	}, nil
}

func (t server) Probe(ctx context.Context, req *protocol.ProbeRequest) (*protocol.ProbeResponse, error) {
	t.DHT.addNode(toNetworkNode(req.Sender))

	nearest := t.DHT.ht.getClosestContacts(t.ht.bSize, req.Key, toNetworkNode(req.Sender))
	return &protocol.ProbeResponse{
		Sender:   fromNetworkNode(t.ht.Self),
		Receiver: req.Sender,
		Key:      req.Key,
		Nearest:  fromNetworkNodes(nearest.Nodes...),
	}, nil
}

func toNetworkNode(n *protocol.Node) *NetworkNode {
	return &NetworkNode{
		ID:   n.ID,
		IP:   net.ParseIP(n.IP),
		Port: int(n.Port),
	}
}

func fromNetworkNode(n *NetworkNode) *protocol.Node {
	return &protocol.Node{
		ID:   n.ID,
		IP:   n.IP.String(),
		Port: int32(n.Port),
	}
}

func toNetworkNodes(ns ...*protocol.Node) (out []*NetworkNode) {
	out = make([]*NetworkNode, 0, len(ns))
	for _, n := range ns {
		out = append(out, toNetworkNode(n))
	}
	return out
}

func fromNetworkNodes(ns ...*NetworkNode) (out []*protocol.Node) {
	out = make([]*protocol.Node, 0, len(ns))
	for _, n := range ns {
		out = append(out, &protocol.Node{
			ID:   n.ID,
			IP:   n.IP.String(),
			Port: int32(n.Port),
		})
	}
	return out
}
