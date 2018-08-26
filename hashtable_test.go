package kademlia

import (
	"bytes"
	"math"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
)

// Create a new node and bootstrap it. All nodes in the network know of a
// single node closer to the original node. This continues until every k bucket
// is occupied.
func TestFindNodeAllBuckets(t *testing.T) {
	networking := newMockNetworking()
	dht := NewDHT(getIDWithValues(0), Socket{Gateway: net.ParseIP("127.0.0.1"), Port: 3000})
	dht.networking = networking

	go dht.Bind(grpc.NewServer())

	var k = 0
	var i = 6

	go func() {
		for {
			networking.probes <- mockFindNodeResponse(getZerodIDWithNthByte(k, byte(math.Pow(2, float64(i)))))

			i--
			if i < 0 {
				i = 7
				k++
			}
			if k > 19 {
				k = 19
			}
		}
	}()

	dht.Bootstrap(
		NetworkNode{
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
	probes := make(chan int)
	dht := NewDHT(getIDWithValues(0), Socket{Gateway: net.ParseIP("127.0.0.1"), Port: 3000})
	dht.networking = networking

	go dht.Bind(grpc.NewServer())

	go func() {
		for i := 1; i < dht.ht.bSize; i++ {
			id := getZerodIDWithNthByte(1, byte(255-i))
			networking.probes <- mockFindNodeResponse(id)
		}

		networking.probes <- mockFindNodeResponse(getZerodIDWithNthByte(1, byte(255)))
		close(probes)
	}()

	dht.Bootstrap(
		NetworkNode{
			ID:   getZerodIDWithNthByte(1, byte(255)),
			Port: 3001,
			IP:   net.ParseIP("0.0.0.0"),
		},
	)

	// ensure all the expected nodes are in the table.
	for i := 0; i < dht.ht.bSize; i++ {
		actual := dht.ht.RoutingTable[dht.ht.bBits-9][i].ID
		expected := getZerodIDWithNthByte(1, byte(255-i))
		assert.Equal(t, 0, bytes.Compare(actual, expected))
	}

	<-probes

	assert.NoError(t, dht.Disconnect())
}

func TestGetRandomIDFromBucket(t *testing.T) {
	dht := NewDHT(getIDWithValues(0), mustSocket("127.0.0.1:3000"))
	go dht.Bind(grpc.NewServer())

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
