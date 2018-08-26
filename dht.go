package kademlia

import (
	"bytes"
	"context"
	"encoding/hex"
	"log"
	"net"
	"sort"
	"time"

	"github.com/james-lawrence/kademlia/protocol"
	"github.com/willf/bloom"
	"google.golang.org/grpc"
)

// Option for a distributed hash table.
type Option func(*DHT)

// OptionTimeout - timeout to wait beforing timing out when locating a key.
func OptionTimeout(d time.Duration) Option {
	return func(dht *DHT) {
		dht.TLocateTimeout = d
	}
}

// OptionRefresh see DHT.TRefresh
func OptionRefresh(d time.Duration) Option {
	return func(dht *DHT) {
		dht.TRefresh = d
	}
}

// OptionNodeID set node id, overriding the fingerprint.
// only should be used in tests.
func OptionNodeID(id []byte) Option {
	return func(dht *DHT) {
		dht.n.ID = id
	}
}

// DHT represents the state of the local node in the distributed hash table
type DHT struct {
	s          Socket
	n          NetworkNode
	ht         *hashTable
	networking networking

	// Seconds after which an otherwise unaccessed bucket must be refreshed
	TRefresh time.Duration

	// The maximum time to wait for a response from a node before discarding
	// it from the bucket
	TPingMax time.Duration

	// The maximum time to wait to locate a key before timing out.
	TLocateTimeout time.Duration
}

// NewDHT initializes a new DHT node. A store and options struct must be
// provided.
func NewDHT(s Socket, options ...Option) *DHT {
	dht := DHT{
		s:              s,
		n:              s.NewNode(),
		TRefresh:       time.Hour,
		TPingMax:       time.Second,
		TLocateTimeout: 5 * time.Second,
	}.merge(options...)

	dht = dht.merge(Option(func(u *DHT) {
		u.ht = newHashTable(u.n)
		u.networking = newNetwork(u.n, s)
	}))

	return &dht
}

func (dht DHT) merge(options ...Option) DHT {
	for _, opt := range options {
		opt(&dht)
	}

	return dht
}

// NumNodes returns the total number of nodes stored in the local routing table
func (dht *DHT) NumNodes() int {
	return dht.ht.totalNodes()
}

// Nodes returns the nodes themselves sotred in the routing table.
func (dht *DHT) Nodes() []NetworkNode {
	return dht.ht.Nodes()
}

// GetSelfID returns the identifier of the local node
func (dht *DHT) GetSelfID() []byte {
	return dht.ht.Self.ID
}

// Dial a peer in the DHT.
func (dht *DHT) Dial(ctx context.Context, n NetworkNode) (net.Conn, error) {
	return dht.s.Dial(ctx, n)
}

// GetSelf returns the node information of the local node.
func (dht *DHT) GetSelf() NetworkNode {
	return dht.ht.Self
}

// Bind registers the kademlia protocol to the grpc.Server
func (dht *DHT) Bind(s *grpc.Server) error {
	protocol.RegisterKademliaServer(s, server{DHT: dht})
	go dht.timers()
	return dht.networking.listen(s)
}

// Bootstrap attempts to bootstrap the network using the BootstrapNodes provided
// to the Options struct. This will trigger an iterativeFindNode to the provided
// BootstrapNodes.
func (dht *DHT) Bootstrap(nodes ...NetworkNode) (err error) {
	if len(nodes) == 0 {
		return nil
	}

	for _, bn := range nodes {
		if bn.ID == nil {
			deadline, cancel := context.WithTimeout(context.Background(), dht.TPingMax)
			if bn, err = dht.networking.ping(deadline, bn); err != nil {
				cancel()
				log.Println("failed to send ping", err)
				continue
			}
			cancel()
		}

		dht.addNode(bn)
	}

	if dht.NumNodes() > 0 {
		_, err := dht.Locate(dht.ht.Self.ID)
		return err
	}

	return nil
}

// Disconnect will trigger a disconnect from the network. All underlying sockets
// will be closed.
func (dht *DHT) Disconnect() error {
	return dht.networking.disconnect()
}

// Locate does an iterative search through the network based on key.
// used to locate the nodes to interact with for a given key.
func (dht *DHT) Locate(key []byte) (_none []NetworkNode, err error) {
	var (
		// According to the Kademlia white paper, after a round of FIND_NODE RPCs
		// fails to provide a node closer than closestNode, we should send a
		// FIND_NODE RPC to all remaining nodes in the shortlist that have not
		// yet been contacted.
		queryRest bool
		// We keep track of nodes contacted so far. We don't contact the same node
		// twice.
		contacted = bloom.NewWithEstimates(1000, 0.0005)
	)

	sl := dht.ht.getClosestContacts(alpha, key)

	// We keep a reference to the closestNode. If after performing a search
	// we do not find a closer node, we stop searching.
	if len(sl.Nodes) == 0 {
		return _none, nil
	}

	closestNode := sl.Nodes[0]

	bucket := getBucketIndexFromDifferingBit(dht.ht.bBits, key, dht.ht.Self.ID)
	dht.ht.resetRefreshTimeForBucket(bucket)

	for {
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

			deadline, cancel := context.WithTimeout(context.Background(), dht.TLocateTimeout)
			nearest, err := dht.networking.probe(deadline, key, node)
			cancel()
			if err != nil {
				// Node was unreachable for some reason. We will have to remove
				// it from the shortlist, but we will keep it in our routing
				// table in hopes that it might come back online in the future.
				sl.RemoveNode(node)
				continue
			}

			dht.addNode(node)
			// log.Println("added node", dht.GetSelf().IP, dht.GetSelf().Port, "->", node.IP, node.Port, hex.EncodeToString(node.ID))

			sl.AppendUniqueNetworkNodes(nearest...)
		}

		if !queryRest && len(sl.Nodes) == 0 {
			return _none, nil
		}

		sort.Sort(sl)

		// If closestNode is unchanged then we are done
		if bytes.Compare(sl.Nodes[0].ID, closestNode.ID) == 0 || queryRest {
			// We are done
			if !queryRest {
				queryRest = true
				continue
			}

			// log.Println("SUCCESS", dht.GetSelf().IP, dht.GetSelf().Port, len(sl.Nodes))
			return sl.Nodes, nil
		}

		closestNode = sl.Nodes[0]
	}
}

// addNode adds a node into the appropriate k bucket
// we store these buckets in big-endian order so we look at the bits
// from right to left in order to find the appropriate bucket
func (dht *DHT) addNode(node NetworkNode) {
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
		// If the bucket is full we need to ping the oldest node to find out
		// if it responds back in a reasonable amount of time. If not - remove it.
		deadline, cancel := context.WithTimeout(context.Background(), dht.TPingMax)
		if _, err := dht.networking.ping(deadline, bucket[0]); err != nil {
			bucket = append(bucket, node)
			bucket = bucket[1:]
		}

		cancel()
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
