package kademlia

import (
	"context"
	"errors"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/james-lawrence/kademlia/protocol"
	"google.golang.org/grpc"
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
}

func newNetwork(n NetworkNode, s Socket) *realNetworking {
	return &realNetworking{
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
	dst := net.JoinHostPort(to.IP.String(), strconv.Itoa(to.Port))
	return grpc.Dial(dst, grpc.WithInsecure(), grpc.WithDialer(func(addr string, timeout time.Duration) (net.Conn, error) {
		deadline, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		return rn.socket.Dial(deadline, to)
	}))
}

func (rn *realNetworking) ping(deadline context.Context, to NetworkNode) (_zn NetworkNode, err error) {
	conn, err := rn.getConn(to)
	if err != nil {
		return _zn, err
	}
	defer conn.Close()

	resp, err := protocol.NewKademliaClient(conn).Ping(deadline, &protocol.PingRequest{
		Sender:   FromNetworkNode(rn.node),
		Receiver: FromNetworkNode(to),
	})

	if err != nil {
		return _zn, err
	}

	return toNetworkNode(resp.Sender), err
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
