package kademlia

import (
	"errors"
	"net"
)

func newMockNetworking() *mockNetworking {
	return &mockNetworking{
		recv:          make(chan *Message),
		send:          make(chan *Message),
		msgChan:       make(chan *Message),
		dcMessageChan: make(chan int),
		dcTimersChan:  make(chan int),
		dc:            make(chan int),
	}
}

type mockNetworking struct {
	recv          chan (*Message)
	send          chan (*Message)
	dc            chan (int)
	dcTimersChan  chan (int)
	dcMessageChan chan (int)
	msgChan       chan (*Message)
	failNext      bool
	msgCounter    int64
}

func (net *mockNetworking) listen() error {
	return nil
}

func (net *mockNetworking) getNetworkAddr() string {
	return ""
}

func (net *mockNetworking) disconnect() error {
	close(net.dc)
	<-net.dcTimersChan
	close(net.recv)
	close(net.send)
	close(net.msgChan)
	<-net.dcMessageChan
	return nil
}

func (net *mockNetworking) isInitialized() bool {
	return true
}

func (net *mockNetworking) createSocket(host net.IP, port string, useStun bool, stunAddr string) (publicHost string, publicPort string, err error) {
	return "", "", nil
}

func (net *mockNetworking) cancelResponse(*expectedResponse) {
}

func (net *mockNetworking) init() {
	net.recv = make(chan (*Message))
	net.send = make(chan (*Message))
	net.msgChan = make(chan (*Message))
	net.dcMessageChan = make(chan (int))
	net.dcTimersChan = make(chan (int))
	net.dc = make(chan (int))
}

func (net *mockNetworking) messagesFin() {
	close(net.dcMessageChan)
}

func (net *mockNetworking) timersFin() {
	close(net.dcTimersChan)
}

func (net *mockNetworking) getDisconnect() chan (int) {
	return net.dc
}

func (net *mockNetworking) getMessage() chan (*Message) {
	return net.msgChan
}

func (net *mockNetworking) failNextSendMessage() {
	net.failNext = true
}

func (net *mockNetworking) sendMessage(q *Message, expectResponse bool, id int64) (*expectedResponse, error) {
	if id == 0 {
		id = net.msgCounter
		net.msgCounter++
	}
	if net.failNext {
		net.failNext = false
		return nil, errors.New("MockNetworking Error")
	}
	net.recv <- q
	if expectResponse {
		return &expectedResponse{ch: net.send, query: q, node: q.Receiver, id: id}, nil
	}
	return nil, nil
}

func mockFindNodeResponse(query *Message, nextID []byte) *Message {
	r := &Message{}
	n := &NetworkNode{}
	n.ID = query.Sender.ID
	n.IP = query.Sender.IP
	n.Port = query.Sender.Port
	r.Receiver = n
	r.Sender = &NetworkNode{ID: query.Receiver.ID, IP: net.ParseIP("0.0.0.0"), Port: 3001}
	r.Type = query.Type
	r.IsResponse = true
	responseData := &responseDataFindNode{}
	responseData.Closest = []*NetworkNode{{IP: net.ParseIP("0.0.0.0"), Port: 3001, ID: nextID}}
	r.Data = responseData
	return r
}

func mockFindNodeResponseEmpty(query *Message) *Message {
	r := &Message{}
	n := &NetworkNode{}
	n.ID = query.Sender.ID
	n.IP = query.Sender.IP
	n.Port = query.Sender.Port
	r.Receiver = n
	r.Sender = &NetworkNode{ID: query.Receiver.ID, IP: net.ParseIP("0.0.0.0"), Port: 3001}
	r.Type = query.Type
	r.IsResponse = true
	responseData := &responseDataFindNode{}
	responseData.Closest = []*NetworkNode{}
	r.Data = responseData
	return r
}
