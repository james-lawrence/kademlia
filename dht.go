package kademlia

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/willf/bloom"
)

// Option for a distributed hash table.
type Option func(*DHT)

// OptionExpire see DHT.TExpire
func OptionExpire(d time.Duration) Option {
	return func(dht *DHT) {
		dht.TExpire = d
	}
}

// OptionReplicate see DHT.TReplicate
func OptionReplicate(d time.Duration) Option {
	return func(dht *DHT) {
		dht.TReplicate = d
	}
}

// OptionRefresh see DHT.TRefresh
func OptionRefresh(d time.Duration) Option {
	return func(dht *DHT) {
		dht.TRefresh = d
	}
}

// DHT represents the state of the local node in the distributed hash table
type DHT struct {
	ht         *hashTable
	networking networking

	// time after which a key/value pair expires;
	// this is a time-to-live (TTL) from the original publication date
	TExpire time.Duration

	// Seconds after which an otherwise unaccessed bucket must be refreshed
	TRefresh time.Duration

	// The interval between Kademlia replication events, when a node is
	// required to publish its entire database
	TReplicate time.Duration

	// The maximum time to wait for a response from a node before discarding
	// it from the bucket
	TPingMax time.Duration

	// The maximum time to wait for a response to any message
	TMsgTimeout time.Duration
}

// NewDHT initializes a new DHT node. A store and options struct must be
// provided.
func NewDHT(n NetworkNode, options ...Option) *DHT {
	dht := &DHT{
		ht:          newHashTable(&n),
		networking:  newNetwork(&n),
		TExpire:     24 * time.Hour,
		TRefresh:    time.Hour,
		TReplicate:  time.Hour,
		TPingMax:    time.Second,
		TMsgTimeout: 5 * time.Second,
	}

	for _, opt := range options {
		opt(dht)
	}

	return dht
}

func (dht *DHT) getExpirationTime(key []byte) time.Time {
	bucket := getBucketIndexFromDifferingBit(dht.ht.bBits, key, dht.ht.Self.ID)
	var total int
	for i := 0; i < bucket; i++ {
		total += dht.ht.getTotalNodesInBucket(i)
	}
	closer := dht.ht.getAllNodesInBucketCloserThan(bucket, key)
	score := total + len(closer)

	if score == 0 {
		score = 1
	}

	if score > dht.ht.bSize {
		return time.Now().Add(dht.TExpire)
	}

	day := dht.TExpire
	seconds := day.Nanoseconds() * int64(math.Exp(float64(dht.ht.bSize/score)))
	dur := time.Second * time.Duration(seconds)
	return time.Now().Add(dur)
}

// // Store stores data at the provided key on the network. This will trigger an iterateStore message.
// func (dht *DHT) Store(key, data []byte) (err error) {
// 	expiration := dht.getExpirationTime(key)
// 	replication := time.Now().Add(dht.TReplicate)
// 	dht.store.Store(key, data, replication, expiration, true)
// 	_, _, err = dht.iterate(iterateStore, key, data)
// 	if err != nil {
// 		return err
// 	}
//
// 	return nil
// }

// // Get retrieves data from the networking using key. Key is the base58 encoded
// // identifier of the data.
// func (dht *DHT) Get(key []byte) (data []byte, found bool, err error) {
// 	if len(key) != dht.ht.bSize {
// 		return nil, false, errors.New("Invalid key")
// 	}
//
// 	value, exists := dht.store.Retrieve(key)
//
// 	if !exists {
// 		var err error
// 		value, _, err = dht.iterate(iterateFindValue, key, nil)
// 		if err != nil {
// 			return nil, false, err
// 		}
//
// 		if value != nil {
// 			exists = true
// 		}
// 	}
//
// 	return value, exists, nil
// }

// NumNodes returns the total number of nodes stored in the local routing table
func (dht *DHT) NumNodes() int {
	return dht.ht.totalNodes()
}

// Nodes returns the nodes themselves sotred in the routing table.
func (dht *DHT) Nodes() []*NetworkNode {
	return dht.ht.Nodes()
}

// GetSelfID returns the identifier of the local node
func (dht *DHT) GetSelfID() []byte {
	return dht.ht.Self.ID
}

// GetSelf returns the node information of the local node.
func (dht *DHT) GetSelf() *NetworkNode {
	return dht.ht.Self
}

// Listen begins listening on the socket for incoming messages
func (dht *DHT) Listen(out chan *Message) error {
	go dht.listen(out)
	go dht.timers()
	return dht.networking.listen()
}

// Bootstrap attempts to bootstrap the network using the BootstrapNodes provided
// to the Options struct. This will trigger an iterativeFindNode to the provided
// BootstrapNodes.
func (dht *DHT) Bootstrap(nodes ...*NetworkNode) error {
	if len(nodes) == 0 {
		return nil
	}
	expectedResponses := []*expectedResponse{}
	wg := &sync.WaitGroup{}
	for _, bn := range nodes {
		query := &Message{}
		query.Sender = dht.ht.Self
		query.Receiver = bn
		query.Type = messageTypePing
		if bn.ID == nil {
			res, err := dht.networking.sendMessage(query, true, -1)
			if err != nil {
				fmt.Println("failed to send message", err)
				continue
			}

			wg.Add(1)
			expectedResponses = append(expectedResponses, res)
		} else {
			dht.addNode(bn)
		}
	}

	numExpectedResponses := len(expectedResponses)

	if numExpectedResponses > 0 {
		for _, r := range expectedResponses {
			go func(r *expectedResponse) {
				select {
				case result := <-r.ch:
					// If result is nil, channel was closed
					if result != nil {
						dht.addNode(result.Sender)
					}
					wg.Done()
					return
				case <-time.After(dht.TMsgTimeout):
					dht.networking.cancelResponse(r)
					wg.Done()
					return
				}
			}(r)
		}
	}

	wg.Wait()

	if dht.NumNodes() > 0 {
		_, err := dht.Locate(dht.ht.Self.ID)
		// _, _, err := dht.iterate(iterateFindNode, dht.ht.Self.ID, nil)
		return err
	}

	return nil
}

// Disconnect will trigger a disconnect from the network. All underlying sockets
// will be closed.
func (dht *DHT) Disconnect() error {
	return dht.networking.disconnect()
}

// Send invoke an RPC call.
func (dht *DHT) Send(m Message) (Message, error) {
	details := spew.Sdump(m)
	log.Println("sending", details, dht.ht.Self.IP, dht.ht.Self.Port)

	MessageOptionSender(dht.ht.Self)(&m)

	// Send the async queries and wait for a response
	res, err := dht.networking.sendMessage(&m, true, -1)
	if err != nil {
		return Message{}, err
	}

	if res == nil {
		return m, err
	}

	log.Println("sent", details, m.ID, dht.ht.Self.IP, dht.ht.Self.Port)
	select {
	case rmsg := <-res.ch:
		log.Println("received response", details, res.id, dht.ht.Self.IP, dht.ht.Self.Port)
		return *rmsg, nil
	case <-time.After(15 * time.Second):
		return Message{}, errors.New("timeout")
	}
}

// Locate does an iterative search through the network based on key.
// used to locate the nodes to interact with for a given key.
func (dht *DHT) Locate(key []byte) (closest []*NetworkNode, err error) {
	var (
		// According to the Kademlia white paper, after a round of FIND_NODE RPCs
		// fails to provide a node closer than closestNode, we should send a
		// FIND_NODE RPC to all remaining nodes in the shortlist that have not
		// yet been contacted.
		queryRest bool
		// We keep track of nodes contacted so far. We don't contact the same node
		// twice.
		contacted = bloom.NewWithEstimates(1000, 0.005)
	)

	sl := dht.ht.getClosestContacts(alpha, key, []*NetworkNode{})

	// We keep a reference to the closestNode. If after performing a search
	// we do not find a closer node, we stop searching.
	if len(sl.Nodes) == 0 {
		return closest, nil
	}

	closestNode := sl.Nodes[0]

	bucket := getBucketIndexFromDifferingBit(dht.ht.bBits, key, dht.ht.Self.ID)
	dht.ht.resetRefreshTimeForBucket(bucket)

	removeFromShortlist := []*NetworkNode{}

	for {
		var (
			numExpectedResponses int
		)
		expectedResponses := []*expectedResponse{}

		// Next we send messages to the first (closest) alpha nodes in the
		// shortlist and wait for a response
		for i, node := range sl.Nodes {
			// Contact only alpha nodes
			if i >= alpha && !queryRest {
				break
			}

			// Don't contact nodes already contacted
			if contacted.TestAndAdd(node.ID) {
				continue
			}

			query := &Message{
				Sender:   dht.ht.Self,
				Receiver: node,
				Type:     messageTypeFindNode,
				Data: &queryDataFindNode{
					Target: key,
				},
			}

			// Send the async queries and wait for a response
			res, err := dht.networking.sendMessage(query, true, -1)
			if err != nil {
				// Node was unreachable for some reason. We will have to remove
				// it from the shortlist, but we will keep it in our routing
				// table in hopes that it might come back online in the future.
				removeFromShortlist = append(removeFromShortlist, query.Receiver)
				continue
			}

			expectedResponses = append(expectedResponses, res)
		}

		for _, n := range removeFromShortlist {
			sl.RemoveNode(n)
		}

		numExpectedResponses = len(expectedResponses)

		resultChan := make(chan (*Message))
		for _, r := range expectedResponses {
			go func(r *expectedResponse) {
				select {
				case result := <-r.ch:
					if result == nil {
						// Channel was closed
						return
					}
					dht.addNode(result.Sender)
					resultChan <- result
					return
				case <-time.After(dht.TMsgTimeout):
					dht.networking.cancelResponse(r)
					return
				}
			}(r)
		}

		var results []*Message
		if numExpectedResponses > 0 {
		Loop:
			for {
				select {
				case result := <-resultChan:
					if result != nil {
						results = append(results, result)
					} else {
						numExpectedResponses--
					}
					if len(results) == numExpectedResponses {
						close(resultChan)
						break Loop
					}
				case <-time.After(dht.TMsgTimeout):
					close(resultChan)
					break Loop
				}
			}

			for _, result := range results {
				if result.Error != nil {
					sl.RemoveNode(result.Receiver)
					continue
				}
				responseData := result.Data.(*responseDataFindNode)
				sl.AppendUniqueNetworkNodes(responseData.Closest)
			}
		}

		if !queryRest && len(sl.Nodes) == 0 {
			return closest, nil
		}

		sort.Sort(sl)

		// If closestNode is unchanged then we are done
		if bytes.Compare(sl.Nodes[0].ID, closestNode.ID) == 0 || queryRest {
			// We are done
			if !queryRest {
				queryRest = true
				continue
			}
			return sl.Nodes, nil
		}

		closestNode = sl.Nodes[0]
	}
}

// // Iterate does an iterative search through the network. This can be done
// // for multiple reasons. These reasons include:
// //     iterativeStore - Used to store new information in the network.
// //     iterativeFindNode - Used to bootstrap the network.
// //     iterativeFindValue - Used to find a value among the network given a key.
// func (dht *DHT) iterate(t int, target []byte, data []byte) (value []byte, closest []*NetworkNode, err error) {
// 	sl := dht.ht.getClosestContacts(alpha, target, []*NetworkNode{})
//
// 	// We keep track of nodes contacted so far. We don't contact the same node
// 	// twice.
// 	var contacted = make(map[string]bool)
//
// 	// According to the Kademlia white paper, after a round of FIND_NODE RPCs
// 	// fails to provide a node closer than closestNode, we should send a
// 	// FIND_NODE RPC to all remaining nodes in the shortlist that have not
// 	// yet been contacted.
// 	queryRest := false
//
// 	// We keep a reference to the closestNode. If after performing a search
// 	// we do not find a closer node, we stop searching.
// 	if len(sl.Nodes) == 0 {
// 		return nil, nil, nil
// 	}
//
// 	closestNode := sl.Nodes[0]
//
// 	if t == iterateFindNode {
// 		bucket := getBucketIndexFromDifferingBit(dht.ht.bBits, target, dht.ht.Self.ID)
// 		dht.ht.resetRefreshTimeForBucket(bucket)
// 	}
//
// 	removeFromShortlist := []*NetworkNode{}
//
// 	for {
// 		var (
// 			numExpectedResponses int
// 		)
// 		expectedResponses := []*expectedResponse{}
//
// 		// Next we send messages to the first (closest) alpha nodes in the
// 		// shortlist and wait for a response
//
// 		for i, node := range sl.Nodes {
// 			// Contact only alpha nodes
// 			if i >= alpha && !queryRest {
// 				break
// 			}
//
// 			// Don't contact nodes already contacted
// 			if contacted[string(node.ID)] == true {
// 				continue
// 			}
//
// 			contacted[string(node.ID)] = true
// 			query := &message{}
// 			query.Sender = dht.ht.Self
// 			query.Receiver = node
//
// 			switch t {
// 			case iterateFindNode:
// 				query.Type = messageTypeFindNode
// 				queryData := &queryDataFindNode{}
// 				queryData.Target = target
// 				query.Data = queryData
// 			case iterateFindValue:
// 				query.Type = messageTypeFindValue
// 				queryData := &queryDataFindValue{}
// 				queryData.Target = target
// 				query.Data = queryData
// 			case iterateStore:
// 				query.Type = messageTypeFindNode
// 				queryData := &queryDataFindNode{}
// 				queryData.Target = target
// 				query.Data = queryData
// 			default:
// 				panic("Unknown iterate type")
// 			}
//
// 			// Send the async queries and wait for a response
// 			res, err := dht.networking.sendMessage(query, true, -1)
// 			if err != nil {
// 				// Node was unreachable for some reason. We will have to remove
// 				// it from the shortlist, but we will keep it in our routing
// 				// table in hopes that it might come back online in the future.
// 				removeFromShortlist = append(removeFromShortlist, query.Receiver)
// 				continue
// 			}
//
// 			expectedResponses = append(expectedResponses, res)
// 		}
//
// 		for _, n := range removeFromShortlist {
// 			sl.RemoveNode(n)
// 		}
//
// 		numExpectedResponses = len(expectedResponses)
//
// 		resultChan := make(chan (*message))
// 		for _, r := range expectedResponses {
// 			go func(r *expectedResponse) {
// 				select {
// 				case result := <-r.ch:
// 					if result == nil {
// 						// Channel was closed
// 						return
// 					}
// 					dht.addNode(newNode(result.Sender))
// 					resultChan <- result
// 					return
// 				case <-time.After(dht.TMsgTimeout):
// 					dht.networking.cancelResponse(r)
// 					return
// 				}
// 			}(r)
// 		}
//
// 		var results []*message
// 		if numExpectedResponses > 0 {
// 		Loop:
// 			for {
// 				select {
// 				case result := <-resultChan:
// 					if result != nil {
// 						results = append(results, result)
// 					} else {
// 						numExpectedResponses--
// 					}
// 					if len(results) == numExpectedResponses {
// 						close(resultChan)
// 						break Loop
// 					}
// 				case <-time.After(dht.TMsgTimeout):
// 					close(resultChan)
// 					break Loop
// 				}
// 			}
//
// 			for _, result := range results {
// 				if result.Error != nil {
// 					sl.RemoveNode(result.Receiver)
// 					continue
// 				}
// 				switch t {
// 				case iterateFindNode:
// 					responseData := result.Data.(*responseDataFindNode)
// 					sl.AppendUniqueNetworkNodes(responseData.Closest)
// 				case iterateFindValue:
// 					responseData := result.Data.(*responseDataFindValue)
// 					// TODO When an iterativeFindValue succeeds, the initiator must
// 					// store the key/value pair at the closest node seen which did
// 					// not return the value.
// 					if responseData.Value != nil {
// 						return responseData.Value, nil, nil
// 					}
// 					sl.AppendUniqueNetworkNodes(responseData.Closest)
// 				case iterateStore:
// 					responseData := result.Data.(*responseDataFindNode)
// 					sl.AppendUniqueNetworkNodes(responseData.Closest)
// 				}
// 			}
// 		}
//
// 		if !queryRest && len(sl.Nodes) == 0 {
// 			return nil, nil, nil
// 		}
//
// 		sort.Sort(sl)
//
// 		// If closestNode is unchanged then we are done
// 		if bytes.Compare(sl.Nodes[0].ID, closestNode.ID) == 0 || queryRest {
// 			// We are done
// 			switch t {
// 			case iterateFindNode:
// 				if !queryRest {
// 					queryRest = true
// 					continue
// 				}
// 				return nil, sl.Nodes, nil
// 			case iterateFindValue:
// 				return nil, sl.Nodes, nil
// 			case iterateStore:
// 				for i, n := range sl.Nodes {
// 					if i >= dht.ht.bSize {
// 						return nil, nil, nil
// 					}
//
// 					query := &message{
// 						Type:     messageTypeStore,
// 						Receiver: n,
// 						Sender:   dht.ht.Self,
// 						Data: &queryDataStore{
// 							Data: data,
// 						},
// 					}
// 					dht.networking.sendMessage(query, false, -1)
// 				}
// 				return nil, nil, nil
// 			}
// 		} else {
// 			closestNode = sl.Nodes[0]
// 		}
// 	}
// }

// addNode adds a node into the appropriate k bucket
// we store these buckets in big-endian order so we look at the bits
// from right to left in order to find the appropriate bucket
func (dht *DHT) addNode(node *NetworkNode) {
	index := getBucketIndexFromDifferingBit(dht.ht.bBits, dht.ht.Self.ID, node.ID)

	// Make sure node doesn't already exist
	// If it does, mark it as seen
	if dht.ht.doesNodeExistInBucket(index, node.ID) {
		dht.ht.markNodeAsSeen(node.ID)
		return
	}

	dht.ht.mutex.Lock()
	defer dht.ht.mutex.Unlock()

	bucket := dht.ht.RoutingTable[index]

	if len(bucket) == dht.ht.bSize {
		// If the bucket is full we need to ping the first node to find out
		// if it responds back in a reasonable amount of time. If not -
		// we may remove it
		n := bucket[0]
		query := &Message{}
		query.Receiver = n
		query.Sender = dht.ht.Self
		query.Type = messageTypePing
		res, err := dht.networking.sendMessage(query, true, -1)
		if err != nil {
			bucket = append(bucket, node)
			bucket = bucket[1:]
		} else {
			select {
			case <-res.ch:
				return
			case <-time.After(dht.TPingMax):
				bucket = bucket[1:]
				bucket = append(bucket, node)
			}
		}
	} else {
		bucket = append(bucket, node)
	}

	dht.ht.RoutingTable[index] = bucket
}

func (dht *DHT) timers() {
	t := time.NewTicker(time.Second)
	for {
		select {
		case <-t.C:
			// Refresh
			for i := 0; i < dht.ht.bBits; i++ {
				if time.Since(dht.ht.getRefreshTimeForBucket(i)) > dht.TRefresh {
					id := dht.ht.getRandomIDFromBucket(dht.ht.bSize)
					if _, err := dht.Locate(id); err != nil {
						log.Println("failed to ping", hex.EncodeToString(id))
					}
				}
			}
		case <-dht.networking.getDisconnect():
			t.Stop()
			dht.networking.timersFin()
			return
		}
	}
}

func (dht *DHT) listen(out chan *Message) {
	defer func() {
		select {
		case <-out:
		default:
			if out != nil {
				close(out)
			}
		}
	}()

	for {
		select {
		case msg := <-dht.networking.getMessage():
			if msg == nil {
				// Disconnected
				dht.networking.messagesFin()
				return
			}

			switch msg.Type {
			case messageTypeFindNode:
				data := msg.Data.(*queryDataFindNode)
				closest := dht.ht.getClosestContacts(dht.ht.bSize, data.Target, []*NetworkNode{msg.Sender})
				response := &Message{
					Type:     messageTypeFindNode,
					Sender:   dht.ht.Self,
					Receiver: msg.Sender,
					Data: &responseDataFindNode{
						Closest: closest.Nodes,
					},
					IsResponse: true,
				}
				dht.networking.sendMessage(response, false, msg.ID)
			case messageTypePing:
				response := &Message{
					Type:       messageTypePing,
					Sender:     dht.ht.Self,
					Receiver:   msg.Sender,
					IsResponse: true,
				}
				dht.networking.sendMessage(response, false, msg.ID)
				// not interested in adding nodes due to a ping.
				continue
			default:
				select {
				case out <- msg:
				default:
				}
			}

			dht.addNode(msg.Sender)
		case <-dht.networking.getDisconnect():
			dht.networking.messagesFin()
			return
		}
	}
}
