package kademlia

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/hex"
	"log"
	"math/rand"
	"sort"
	"sync"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/james-lawrence/kademlia/protocol"
	"github.com/willf/bloom"
	"google.golang.org/grpc"
)

type puncher interface {
	Dial(context.Context, NetworkNode, ...grpc.CallOption) error
}
type nodeChecksum interface {
	Valid(NetworkNode) bool
}

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

// OptionNodeIDChecksum set the function that forces node IDs to conform to a
// checksum. prevents nodes from having multiple IDs for a single address/port.
func OptionNodeIDChecksum(c nodeChecksum) Option {
	return func(dht *DHT) {
		dht.checksum = c
	}
}

// OptionTLSConfig ...
func OptionTLSConfig(c *tls.Config) Option {
	return func(dht *DHT) {
		dht.tlsc = c
	}
}

// DHT represents the state of the local node in the distributed hash table
type DHT struct {
	s  Socket
	n  NetworkNode
	ht *hashTable

	m *sync.RWMutex

	// keeps track of bad nodes so we don't repeatedly try to contact them.
	baddies *bloom.BloomFilter

	networking networking

	// checksum is used to ensure the NodeID conforms to some uniqueness rules
	checksum nodeChecksum

	// Seconds after which an otherwise unaccessed bucket must be refreshed,
	// also used to reap dead nodes from the DHT. if a node hasnt been seen in 4x
	// the refresh period then its dead.
	TRefresh time.Duration

	// The maximum time to wait for a response from a node before discarding
	// it from the bucket
	TPingMax time.Duration

	// The maximum time to wait to locate a key before timing out.
	TLocateTimeout time.Duration

	tlsc *tls.Config
}

// NewDHT initializes a new DHT node. A store and options struct must be
// provided.
func NewDHT(s Socket, options ...Option) *DHT {
	dht := DHT{
		s:              s,
		n:              s.NewNode(),
		TRefresh:       30 * time.Second,
		TPingMax:       3 * time.Second,
		TLocateTimeout: 5 * time.Second,
		checksum:       nodeChecksumFunc(gatewayFingerprintChecksum),
		baddies:        bloom.NewWithEstimates(2000, 0.0005),
		m:              &sync.RWMutex{},
	}.merge(options...)

	dht = dht.merge(Option(func(u *DHT) {
		u.ht = newHashTable(u.n)
		u.networking = newNetwork(u.n, s, dht.tlsc)
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
func (dht *DHT) Dial(ctx context.Context, n NetworkNode) (*grpc.ClientConn, error) {
	return dht.networking.getConn(n)
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
			var updated NetworkNode
			if updated, err = dht.ping(bn); err != nil {
				log.Println("ping failed", spew.Sdump(bn), err)
				continue
			}
			bn = updated
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
		contacted = dht.copyBad()
	)

	sl := dht.ht.getClosestContacts(alpha, key)

	// We keep a reference to the closestNode. If after performing a search
	// we do not find a closer node, we stop searching.
	if len(sl.Nodes) == 0 {
		return _none, nil
	}

	closestNode := sl.Nodes[0]

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
				log.Println("unreachable node", hex.EncodeToString(node.ID), node.IP, node.Port, err)

				// Node was unreachable for some reason. We will have to remove
				// it from the shortlist, but we will keep it in our routing
				// table in hopes that it might come back online in the future.
				sl.RemoveNode(node)

				// however lets mark it as bad so we don't try it for awhile.
				dht.addBad(node)
				continue
			}

			dht.addNode(node)

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

			return sl.Nodes, nil
		}

		closestNode = sl.Nodes[0]
	}
}

// addNode adds a node into the appropriate k bucket
// we store these buckets in big-endian order so we look at the bits
// from right to left in order to find the appropriate bucket
func (dht *DHT) addNode(node NetworkNode) {
	if !dht.checksum.Valid(node) {
		log.Println(hex.EncodeToString(node.ID), node.IP, node.Port, "invalid node ID")
		return
	}

	dht.ht.insertNode(node, func(test NetworkNode) error {
		// If the bucket is full we need to ping the oldest node to find out
		// if it responds back in a reasonable amount of time. If not - remove it.
		deadline, cancel := context.WithTimeout(context.Background(), dht.TPingMax)
		defer cancel()
		_, err := dht.networking.ping(deadline, test)
		return err
	})
}

func (dht *DHT) addBad(n NetworkNode) {
	log.Println("adding bad node", hex.EncodeToString(n.ID), n.IP, n.Port)
	dht.m.Lock()
	dht.baddies.Add(n.ID)
	dht.m.Unlock()
}

func (dht *DHT) copyBad() *bloom.BloomFilter {
	dht.m.RLock()
	defer dht.m.RUnlock()
	return dht.baddies.Copy()
}

func (dht *DHT) resetBad() {
	log.Println("clearing bad node set")
	dht.m.Lock()
	dht.baddies.ClearAll()
	dht.m.Unlock()
}

func (dht *DHT) ping(n NetworkNode) (_ NetworkNode, err error) {
	deadline, cancel := context.WithTimeout(context.Background(), dht.TPingMax)
	defer cancel()

	return dht.networking.ping(deadline, n)
}

func (dht *DHT) verify(nodes ...NetworkNode) {
	for _, n := range nodes {
		var (
			err error
		)
		if _, err = dht.ping(n); err != nil {
			log.Println("ping failed", hex.EncodeToString(n.ID), err)
			now := time.Now().UTC()
			death := n.LastSeen.Add(4 * dht.TRefresh)
			if death.Before(now) {
				log.Println("dead node detected, removing", hex.EncodeToString(n.ID), death, now)
				dht.ht.removeNode(n.ID)
			}
			continue
		}

		dht.ht.markNodeAsSeen(n.ID)
	}
}

func (dht *DHT) timers() {
	t2 := time.NewTicker(time.Hour)
	t := time.NewTicker(dht.TRefresh)
	for {
		select {
		case <-t2.C:
			dht.resetBad()
		case <-t.C:
			cutoff := time.Now().UTC().Add(-1 * dht.TRefresh)
			bucket := rand.Intn(dht.ht.bBits)
			id := dht.ht.getRandomIDFromBucket(bucket)
			// log.Println("refreshing bucket", bucket, hex.EncodeToString(id))
			if _, err := dht.Locate(id); err != nil {
				log.Println("bucket refresh failed", hex.EncodeToString(id), err)
			}

			old := dht.ht.lastSeenBefore(cutoff)
			// log.Println("verifying old nodes", len(old))
			dht.verify(old...)
		case <-dht.networking.getDisconnect():
			t.Stop()
			t2.Stop()
			dht.networking.timersFin()
			return
		}
	}
}
