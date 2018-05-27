package kademlia

import (
	"bytes"
	"math/big"
	"net"
	"strconv"

	"github.com/anacrolix/utp"
	"github.com/ccding/go-stun/stun"
)

func mustNode(id []byte, addr string) NetworkNode {
	n, err := NewNode(id, addr)
	if err != nil {
		panic(err)
	}

	return n
}

// NewNode create a new socket.
func NewNode(id []byte, addr string) (n NetworkNode, err error) {
	s, err := utp.NewSocket("udp", addr)
	if err != nil {
		return n, err
	}

	h, port, err := net.SplitHostPort(s.Addr().String())
	if err != nil {
		return n, err
	}

	p, err := strconv.Atoi(port)
	if err != nil {
		return n, err
	}

	return NetworkNode{
		ID:     id,
		IP:     net.ParseIP(h),
		Port:   p,
		socket: s,
	}, nil
}

// NewStunNode enable stun for discovery using the given net.PacketConn
// and the provided stun server address.
func NewStunNode(id []byte, addr, stunAddr string) (n NetworkNode, err error) {
	if n, err = NewNode(id, addr); err != nil {
		return n, err
	}

	c := stun.NewClientWithConnection(n.socket)
	c.SetServerAddr(addr)

	_, h, err := c.Discover()
	if err != nil {
		return n, err
	}

	_, err = c.Keepalive()
	if err != nil {
		return n, err
	}

	return NetworkNode{
		IP:     net.ParseIP(h.IP()),
		Port:   int(h.Port()),
		socket: n.socket,
	}, nil
}

// NetworkNode is the over-the-wire representation of a node
type NetworkNode struct {
	// ID is a 20 byte unique identifier
	ID []byte

	// IP is the public address of the node
	IP net.IP

	// Port is the public port of the node
	Port int

	socket *utp.Socket
}

// nodeList is used in order to sort a list of arbitrary nodes against a
// comparator. These nodes are sorted by xor distance
type shortList struct {
	// Nodes are a list of nodes to be compared
	Nodes []*NetworkNode

	// Comparator is the ID to compare to
	Comparator []byte
}

func areNodesEqual(n1 *NetworkNode, n2 *NetworkNode, allowNilID bool) bool {
	if n1 == nil || n2 == nil {
		return false
	}
	if !allowNilID {
		if n1.ID == nil || n2.ID == nil {
			return false
		}
		if bytes.Compare(n1.ID, n2.ID) != 0 {
			return false
		}
	}
	if !n1.IP.Equal(n2.IP) {
		return false
	}
	if n1.Port != n2.Port {
		return false
	}
	return true
}

func (n *shortList) RemoveNode(node *NetworkNode) {
	for i := 0; i < n.Len(); i++ {
		if bytes.Compare(n.Nodes[i].ID, node.ID) == 0 {
			n.Nodes = append(n.Nodes[:i], n.Nodes[i+1:]...)
			return
		}
	}
}

func (n *shortList) AppendUniqueNetworkNodes(nodes []*NetworkNode) {
	for _, vv := range nodes {
		exists := false
		for _, v := range n.Nodes {
			if bytes.Compare(v.ID, vv.ID) == 0 {
				exists = true
			}
		}
		if !exists {
			n.Nodes = append(n.Nodes, vv)
		}
	}
}

func (n *shortList) AppendUnique(nodes ...*NetworkNode) {
	for _, vv := range nodes {
		exists := false
		for _, v := range n.Nodes {
			if bytes.Compare(v.ID, vv.ID) == 0 {
				exists = true
			}
		}
		if !exists {
			n.Nodes = append(n.Nodes, vv)
		}
	}
}

func (n *shortList) Len() int {
	return len(n.Nodes)
}

func (n *shortList) Swap(i, j int) {
	n.Nodes[i], n.Nodes[j] = n.Nodes[j], n.Nodes[i]
}

func (n *shortList) Less(i, j int) bool {
	iDist := getDistance(n.Nodes[i].ID, n.Comparator)
	jDist := getDistance(n.Nodes[j].ID, n.Comparator)

	if iDist.Cmp(jDist) == -1 {
		return true
	}

	return false
}

func getDistance(id1 []byte, id2 []byte) *big.Int {
	buf1 := new(big.Int).SetBytes(id1)
	buf2 := new(big.Int).SetBytes(id2)
	result := new(big.Int).Xor(buf1, buf2)
	return result
}
