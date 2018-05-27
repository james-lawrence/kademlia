package kademlia

import (
	"bytes"
	"math"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Create a new node and bootstrap it. All nodes in the network know of a
// single node closer to the original node. This continues until every k bucket
// is occupied.
func TestFindNodeAllBuckets(t *testing.T) {
	networking := newMockNetworking()
	dht := NewDHT(NetworkNode{ID: getIDWithValues(0), IP: net.ParseIP("127.0.0.1"), Port: 3000})
	dht.networking = networking

	go func() {
		dht.Listen(nil)
	}()

	var k = 0
	var i = 6

	go func() {
		for {
			query := <-networking.recv
			if query == nil {
				return
			}

			res := mockFindNodeResponse(query, getZerodIDWithNthByte(k, byte(math.Pow(2, float64(i)))))

			i--
			if i < 0 {
				i = 7
				k++
			}
			if k > 19 {
				k = 19
			}

			networking.send <- res
		}
	}()

	dht.Bootstrap(
		&NetworkNode{
			ID:   getZerodIDWithNthByte(0, byte(math.Pow(2, 7))),
			Port: 3001,
			IP:   net.ParseIP("0.0.0.0"),
		},
	)

	for _, v := range dht.ht.RoutingTable {
		assert.Equal(t, 1, len(v))
	}

	assert.NoError(t, dht.Disconnect())
}

// Tests timing out of nodes in a bucket. DHT bootstraps networks and learns
// about 20 subsequent nodes in the same bucket. Upon attempting to add the 21st
// node to the now full bucket, we should receive a ping to the very first node
// added in order to determine if it is still alive.
func TestAddNodeTimeout(t *testing.T) {
	networking := newMockNetworking()
	done := make(chan int)
	pinged := make(chan int)
	dht := NewDHT(NetworkNode{ID: getIDWithValues(0), IP: net.ParseIP("127.0.0.1"), Port: 3000})
	dht.networking = networking

	go dht.Listen(nil)

	var (
		nodesAdded = 1
		firstNode  []byte
		lastNode   []byte
	)

	go func() {
		for {
			query := <-networking.recv
			if query == nil {
				return
			}
			switch query.Type {
			case messageTypeFindNode:
				id := getIDWithValues(0)
				if nodesAdded > dht.ht.bSize+1 {
					close(done)
					return
				}

				if nodesAdded == 1 {
					firstNode = id
				}

				if nodesAdded == dht.ht.bSize {
					lastNode = id
				}

				id[1] = byte(255 - nodesAdded)
				nodesAdded++

				res := mockFindNodeResponse(query, id)
				networking.send <- res
			case messageTypePing:
				assert.Equal(t, messageTypePing, query.Type)
				assert.Equal(t, getZerodIDWithNthByte(1, byte(255)), query.Receiver.ID)
				close(pinged)
			}
		}
	}()

	dht.Bootstrap(
		&NetworkNode{
			ID:   getZerodIDWithNthByte(1, byte(255)),
			Port: 3001,
			IP:   net.ParseIP("0.0.0.0"),
		},
	)

	// ensure the first node in the table is the second node contacted, and the
	// last is the last node contacted
	assert.Equal(t, 0, bytes.Compare(dht.ht.RoutingTable[dht.ht.bBits-9][0].ID, firstNode))
	assert.Equal(t, 0, bytes.Compare(dht.ht.RoutingTable[dht.ht.bBits-9][19].ID, lastNode))

	<-done
	<-pinged

	assert.NoError(t, dht.Disconnect())
}

func TestGetRandomIDFromBucket(t *testing.T) {
	dht := NewDHT(mustNode(getIDWithValues(0), "127.0.0.1:3000"))
	go dht.Listen(nil)

	// Bytes should be equal up to the bucket index that the random ID was
	// generated for, and random afterwards
	for i := 0; i < dht.ht.bBits/8; i++ {
		r := dht.ht.getRandomIDFromBucket(i * 8)
		for j := 0; j < i; j++ {
			assert.Equal(t, byte(0), r[j])
		}
	}

	assert.NoError(t, dht.Disconnect())
}
