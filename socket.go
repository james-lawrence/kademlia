package kademlia

import (
	"bytes"
	"context"
	"net"
	"strconv"
	"time"

	"github.com/anacrolix/utp"
	"google.golang.org/grpc"
)

// SocketOption option for the utp socket.
type SocketOption func(*Socket)

// SocketOptionGateway public IP for the socket.
func SocketOptionGateway(gateway net.IP) SocketOption {
	return func(s *Socket) {
		s.Gateway = gateway
	}
}

// SocketOptionPort public for for the socket.
func SocketOptionPort(port int) SocketOption {
	return func(s *Socket) {
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
		localIP:   net.ParseIP(host),
		localPort: port,
		Gateway:   net.ParseIP(host),
		Port:      port,
		utps:      utps,
	}

	return s.Merge(options...), nil
}

// Socket network connection with public IP information.
type Socket struct {
	localIP, Gateway net.IP
	localPort, Port  int
	utps             *utp.Socket
}

// NewNode create a node from the current socket and the given id.
func (t Socket) NewNode() NetworkNode {
	return NetworkNode{
		ID:   GatewayFingerprint(t.Gateway, t.Port),
		IP:   t.Gateway,
		Port: t.Port,
	}
}

// LocalNode ...
func (t Socket) LocalNode() NetworkNode {
	return NetworkNode{
		ID:   GatewayFingerprint(t.localIP, t.localPort),
		IP:   t.localIP,
		Port: t.localPort,
	}
}

// Merge options into the socket.
func (t Socket) Merge(options ...SocketOption) Socket {
	for _, opt := range options {
		opt(&t)
	}

	return t
}

// Dial the given net.Addr
func (t Socket) Dial(ctx context.Context, addr net.Addr) (conn net.Conn, err error) {
	return t.utps.DialContext(ctx, addr.Network(), addr.String())
}

// Accept waits for and returns the next connection to the listener.
func (t Socket) Accept() (net.Conn, error) {
	return t.utps.Accept()
}

// Close closes the listener.
// Any blocked Accept operations will be unblocked and return errors.
func (t Socket) Close() error {
	return t.utps.Close()
}

// Addr returns the listener's network address.
func (t Socket) Addr() net.Addr {
	return t.utps.Addr()
}

// LocalAddr returns the listener's network address
func (t Socket) LocalAddr() net.Addr {
	return t.utps.LocalAddr()
}

// GatewayFingerprint generate a fingerprint a IP/port combination.
func GatewayFingerprint(ip net.IP, port int) []byte {
	buf := bytes.NewBufferString(ip.String() + strconv.Itoa(int(port))).Bytes()
	return ContentAddressable(buf)
}

type dialer interface {
	Dial(context.Context, net.Addr) (net.Conn, error)
}

// WithUDPNodeDialer creates a DialOption from a dialer
func WithUDPNodeDialer(d dialer) grpc.DialOption {
	return grpc.WithDialer(func(dst string, timeout time.Duration) (_ net.Conn, err error) {
		var (
			addr *net.UDPAddr
		)

		if addr, err = net.ResolveUDPAddr("udp", dst); err != nil {
			return nil, err
		}

		deadline, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		return d.Dial(deadline, addr)
	})
}

func mustSocket(addr string) Socket {
	n, err := NewSocket(addr)
	if err != nil {
		panic(err)
	}

	return n
}
