package kademlia

import (
	"context"
	"crypto/tls"
	"net"
	"strconv"
	"sync"

	"github.com/james-lawrence/kademlia/protocol"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

var (
	errorValueNotFound = errors.New("Value not found")
)

type networking interface {
	ping(ctx context.Context, to NetworkNode) (NetworkNode, error)
	probe(ctx context.Context, key []byte, to NetworkNode) ([]NetworkNode, error)
	timersFin()
	getDisconnect() chan int
	listen(s *grpc.Server) error
	disconnect() error
	getNetworkAddr() string
	getConn(to NetworkNode) (*grpc.ClientConn, error)
}

func newNetwork(n NetworkNode, s Socket, c *tls.Config) *realNetworking {
	return &realNetworking{
		c:            c,
		socket:       s,
		node:         n,
		mutex:        &sync.Mutex{},
		dcStartChan:  make(chan int, 10),
		dcEndChan:    make(chan int),
		dcTimersChan: make(chan int),
		msgCounter:   new(int64),
		aliveConns:   &sync.WaitGroup{},
		connected:    true,
	}
}

type realNetworking struct {
	dcStartChan   chan int
	dcEndChan     chan int
	dcTimersChan  chan int
	mutex         *sync.Mutex
	connected     bool
	aliveConns    *sync.WaitGroup
	msgCounter    *int64
	remoteAddress string
	node          NetworkNode
	socket        Socket
	c             *tls.Config
}

func (rn *realNetworking) getNetworkAddr() string {
	return rn.remoteAddress
}

func (rn *realNetworking) getDisconnect() chan int {
	return rn.dcStartChan
}

func (rn *realNetworking) timersFin() {
	rn.dcTimersChan <- 1
}

func (rn *realNetworking) getConn(to NetworkNode) (*grpc.ClientConn, error) {
	creds := grpc.WithInsecure()
	if rn.c != nil {
		creds = grpc.WithTransportCredentials(credentials.NewTLS(rn.c))
	}

	return grpc.Dial(net.JoinHostPort(to.IP.String(), strconv.Itoa(to.Port)), creds, WithUDPNodeDialer(rn.socket))
}

func (rn *realNetworking) ping(deadline context.Context, to NetworkNode) (_zn NetworkNode, err error) {
	conn, err := rn.getConn(to)
	if err != nil {
		return _zn, errors.Wrap(err, "ping failed")
	}
	defer conn.Close()

	resp, err := protocol.NewKademliaClient(conn).Ping(deadline, &protocol.PingRequest{
		Sender:   FromNetworkNode(rn.node),
		Receiver: FromNetworkNode(to),
	})

	if err != nil {
		return _zn, errors.Wrap(err, "ping failed")
	}

	return toNetworkNode(resp.Sender), nil
}

func (rn *realNetworking) probe(deadline context.Context, key []byte, to NetworkNode) ([]NetworkNode, error) {
	conn, err := rn.getConn(to)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	resp, err := protocol.NewKademliaClient(conn).Probe(deadline, &protocol.ProbeRequest{
		Sender:   FromNetworkNode(rn.node),
		Receiver: FromNetworkNode(to),
		Key:      key,
	})

	if err != nil {
		return nil, err
	}

	return toNetworkNodes(resp.Nearest...), err
}

func (rn *realNetworking) disconnect() error {
	rn.mutex.Lock()
	defer rn.mutex.Unlock()
	if !rn.connected {
		return errors.New("not connected")
	}
	rn.dcStartChan <- 1
	rn.dcStartChan <- 1
	<-rn.dcTimersChan
	close(rn.dcTimersChan)
	err := rn.socket.utps.CloseNow()
	rn.connected = false
	close(rn.dcEndChan)
	return err
}

func (rn *realNetworking) listen(s *grpc.Server) error {
	return s.Serve(rn.socket.utps)
}
