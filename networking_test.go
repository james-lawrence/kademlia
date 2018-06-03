package kademlia

import (
	"context"
	"errors"
	"net"

	"google.golang.org/grpc"
)

func newMockNetworking() *mockNetworking {
	return &mockNetworking{
		pings:        make(chan *NetworkNode),
		probes:       make(chan []*NetworkNode),
		fail:         make(chan error),
		dcTimersChan: make(chan int),
		dc:           make(chan int),
	}
}

type mockNetworking struct {
	probes       chan []*NetworkNode
	pings        chan *NetworkNode
	fail         chan error
	dc           chan int
	dcTimersChan chan int
	msgCounter   int64
}

func (net *mockNetworking) listen(*grpc.Server) error {
	return nil
}

func (net *mockNetworking) getNetworkAddr() string {
	return ""
}

func (net *mockNetworking) disconnect() error {
	close(net.dc)
	<-net.dcTimersChan
	return nil
}

func (net *mockNetworking) createSocket(host net.IP, port string, useStun bool, stunAddr string) (publicHost string, publicPort string, err error) {
	return "", "", nil
}

func (net *mockNetworking) cancelResponse(*expectedResponse) {
}

func (net *mockNetworking) ping(deadline context.Context, to *NetworkNode) (*NetworkNode, error) {
	// log.Println("PING RECEIVED")
	// defer log.Println("PING COMPLETED")
	select {
	case <-deadline.Done():
		return nil, deadline.Err()
	case err := <-net.fail:
		return nil, err
	case from := <-net.pings:
		return from, nil
	}
}

func (net *mockNetworking) probe(deadline context.Context, key []byte, to *NetworkNode) ([]*NetworkNode, error) {
	// log.Println("PROBE RECEIVED", net.probes)
	// defer log.Println("PROBE COMPLETED")
	select {
	case <-deadline.Done():
		return []*NetworkNode{}, deadline.Err()
	case closest := <-net.probes:
		return closest, nil
	case err := <-net.fail:
		return []*NetworkNode{}, err
	}
}

func (net *mockNetworking) timersFin() {
	close(net.dcTimersChan)
}

func (net *mockNetworking) getDisconnect() chan (int) {
	return net.dc
}

func mockFindNodeResponse(nextID []byte) []*NetworkNode {
	return []*NetworkNode{{IP: net.ParseIP("0.0.0.0"), Port: 3001, ID: nextID}}
}

var ErrMockNetworking = errors.New("MockNetworking Error")
