package kademlia

import (
	"context"
	"errors"
	"log"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

var (
	errorValueNotFound = errors.New("Value not found")
)

type networking interface {
	sendMessage(*Message, bool, int64) (*expectedResponse, error)
	getMessage() chan (*Message)
	messagesFin()
	timersFin()
	getDisconnect() chan int
	listen() error
	disconnect() error
	cancelResponse(*expectedResponse)
	getNetworkAddr() string
}

type expectedResponse struct {
	ch    chan (*Message)
	query *Message
	node  *NetworkNode
	id    int64
}

func newNetwork(n *NetworkNode) *realNetworking {
	return &realNetworking{
		self:          n,
		mutex:         &sync.Mutex{},
		sendChan:      make(chan *Message),
		recvChan:      make(chan *Message),
		dcStartChan:   make(chan int, 10),
		dcEndChan:     make(chan int),
		dcTimersChan:  make(chan int),
		dcMessageChan: make(chan int),
		msgCounter:    new(int64),
		responseMap:   make(map[int64]*expectedResponse),
		aliveConns:    &sync.WaitGroup{},
		connected:     n.socket != nil,
	}
}

type realNetworking struct {
	sendChan      chan (*Message)
	recvChan      chan (*Message)
	dcStartChan   chan int
	dcEndChan     chan int
	dcTimersChan  chan int
	dcMessageChan chan int
	mutex         *sync.Mutex
	connected     bool
	responseMap   map[int64]*expectedResponse
	aliveConns    *sync.WaitGroup
	self          *NetworkNode
	msgCounter    *int64
	remoteAddress string
}

// func (rn *realNetworking) isInitialized() bool {
// 	return rn.initialized
// }

func (rn *realNetworking) getMessage() chan (*Message) {
	return rn.recvChan
}

func (rn *realNetworking) getNetworkAddr() string {
	return rn.remoteAddress
}

func (rn *realNetworking) messagesFin() {
	rn.dcMessageChan <- 1
}

func (rn *realNetworking) getDisconnect() chan int {
	return rn.dcStartChan
}

func (rn *realNetworking) timersFin() {
	rn.dcTimersChan <- 1
}

func (rn *realNetworking) sendMessage(msg *Message, expectResponse bool, id int64) (*expectedResponse, error) {
	if id == -1 {
		id = atomic.AddInt64(rn.msgCounter, 1)
	}
	msg.ID = id

	deadline, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	conn, err := rn.self.socket.DialContext(deadline, "", net.JoinHostPort(msg.Receiver.IP.String(), strconv.Itoa(msg.Receiver.Port)))
	if err != nil {
		return nil, err
	}

	data, err := serializeMessage(msg)
	if err != nil {
		return nil, err
	}

	_, err = conn.Write(data)
	if err != nil {
		return nil, err
	}

	if expectResponse {
		rn.mutex.Lock()
		defer rn.mutex.Unlock()
		expectedResponse := &expectedResponse{
			ch:    make(chan *Message),
			node:  msg.Receiver,
			query: msg,
			id:    id,
		}
		// TODO we need a way to automatically clean these up as there are
		// cases where they won't be removed manually
		rn.responseMap[id] = expectedResponse
		return expectedResponse, nil
	}

	return nil, nil
}

func (rn *realNetworking) cancelResponse(res *expectedResponse) {
	rn.mutex.Lock()
	defer rn.mutex.Unlock()
	close(rn.responseMap[res.query.ID].ch)
	delete(rn.responseMap, res.query.ID)
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
	<-rn.dcMessageChan
	close(rn.sendChan)
	close(rn.recvChan)
	close(rn.dcTimersChan)
	close(rn.dcMessageChan)
	err := rn.self.socket.CloseNow()
	rn.connected = false
	close(rn.dcEndChan)
	return err
}

func (rn *realNetworking) listen() error {
	for {
		conn, err := rn.self.socket.Accept()
		if err != nil {
			rn.disconnect()
			<-rn.dcEndChan
			return err
		}

		go func(conn net.Conn) {
			for {
				// Wait for messages
				msg, err := deserializeMessage(conn)
				if err != nil {
					if err.Error() == "EOF" {
						// Node went bye bye
					}
					// TODO should we penalize this node somehow ? Ban it ?
					return
				}

				isPing := msg.Type == messageTypePing

				if !areNodesEqual(msg.Receiver, rn.self, isPing) {
					// TODO should we penalize this node somehow ? Ban it ?
					continue
				}

				if msg.ID < 0 {
					// TODO should we penalize this node somehow ? Ban it ?
					continue
				}

				rn.mutex.Lock()
				if rn.connected {
					if msg.IsResponse {
						if rn.responseMap[msg.ID] == nil {
							// We were not expecting this response
							rn.mutex.Unlock()
							continue
						}

						if !areNodesEqual(rn.responseMap[msg.ID].node, msg.Sender, isPing) {
							// TODO should we penalize this node somehow ? Ban it ?
							rn.mutex.Unlock()
							continue
						}

						if msg.Type != rn.responseMap[msg.ID].query.Type {
							close(rn.responseMap[msg.ID].ch)
							delete(rn.responseMap, msg.ID)
							rn.mutex.Unlock()
							continue
						}

						if !msg.IsResponse {
							close(rn.responseMap[msg.ID].ch)
							delete(rn.responseMap, msg.ID)
							rn.mutex.Unlock()
							continue
						}

						resChan := rn.responseMap[msg.ID].ch
						rn.mutex.Unlock()
						resChan <- msg
						rn.mutex.Lock()
						close(rn.responseMap[msg.ID].ch)
						delete(rn.responseMap, msg.ID)
						rn.mutex.Unlock()
					} else {
						var (
							assertion bool
						)

						switch msg.Type {
						case messageTypeFindNode:
							_, assertion = msg.Data.(*queryDataFindNode)
						default:
							assertion = true
						}

						if !assertion {
							log.Printf("Received bad message %v from %+v", msg.Type, msg.Sender)
							close(rn.responseMap[msg.ID].ch)
							delete(rn.responseMap, msg.ID)
							rn.mutex.Unlock()
							continue
						}

						rn.recvChan <- msg
						rn.mutex.Unlock()
					}
				} else {
					rn.mutex.Unlock()
				}
			}
		}(conn)
	}
}
