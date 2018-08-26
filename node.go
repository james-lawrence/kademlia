package kademlia

import (
	"bytes"
	"context"
	"math/big"
	"net"
	"strconv"

	"github.com/anacrolix/utp"
)

func mustSocket(addr string) Socket {
	n, err := NewSocket(addr)
	if err != nil {
		panic(err)
	}

	return n
}

//
// // NewNode create a new socket.
// func NewNode(id []byte, addr string) (n NetworkNode, err error) {
// 	return NetworkNode{
// 		ID:   id,
// 		IP:   net.ParseIP(h),
// 		Port: p,
// 	}, nil
// }

// SocketOption option for the utp socket.
type SocketOption func(*Socket)

// SocketOptionGateway public IP for the socket.
func SocketOptionGateway(gateway net.IP, port int) SocketOption {
	return func(s *Socket) {
		s.Gateway = gateway
		s.Port = port
	}
}

// NewSocket public ip of the socket.
func NewSocket(addr string, options ...SocketOption) (s Socket, err error) {
	var (
		utps        *utp.Socket
		host, sport string
		port        int
	)

	if utps, err = utp.NewSocket("udp", addr); err != nil {
		return s, err
	}

	if host, sport, err = net.SplitHostPort(utps.LocalAddr().String()); err != nil {
		return s, err
	}

	if port, err = strconv.Atoi(sport); err != nil {
		return s, err
	}

	s = Socket{
		Gateway: net.ParseIP(host),
		Port:    port,
		utps:    utps,
	}

	return s.Merge(options...), nil
}

// Socket network connection with public IP information.
type Socket struct {
	Gateway net.IP
	Port    int
	utps    *utp.Socket
}

// NewNode create a node from the current socket and the given id.
func (t Socket) NewNode() NetworkNode {
	return NetworkNode{
		ID:   GatewayFingerprint(t.Gateway, t.Port),
		IP:   t.Gateway,
		Port: t.Port,
	}
}

// Merge options into the socket.
func (t Socket) Merge(options ...SocketOption) Socket {
	for _, opt := range options {
		opt(&t)
	}

	return t
}

// Dial a peer using this socket.
func (t Socket) Dial(ctx context.Context, n NetworkNode) (net.Conn, error) {
	return t.utps.DialContext(ctx, "udp", net.JoinHostPort(n.IP.String(), strconv.Itoa(n.Port)))
}

// GatewayFingerprint generate a fingerprint a IP/port combination.
func GatewayFingerprint(ip net.IP, port int) []byte {
	buf := bytes.NewBufferString(ip.String() + strconv.Itoa(int(port))).Bytes()
	return ContentAddressable(buf)
}

// NetworkNode is the over-the-wire representation of a node
type NetworkNode struct {
	// ID is a 20 byte unique identifier
	ID []byte

	// IP is the public address of the node
	IP net.IP

	// Port is the public port of the node
	Port int
}

// nodeList is used in order to sort a list of arbitrary nodes against a
// comparator. These nodes are sorted by xor distance
type shortList struct {
	// Nodes are a list of nodes to be compared
	Nodes []NetworkNode

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

func (n *shortList) RemoveNode(node NetworkNode) {
	for i := 0; i < n.Len(); i++ {
		if bytes.Compare(n.Nodes[i].ID, node.ID) == 0 {
			n.Nodes = append(n.Nodes[:i], n.Nodes[i+1:]...)
			return
		}
	}
}

func (n *shortList) AppendUniqueNetworkNodes(nodes ...NetworkNode) {
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

func (n *shortList) AppendUnique(nodes ...NetworkNode) {
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
