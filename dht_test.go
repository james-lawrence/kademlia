package kademlia

import (
	"bytes"
	"fmt"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// Creates twenty DHTs and bootstraps each with the previous
// at the end all should know about each other
func TestBootstrapTwentyNodes(t *testing.T) {
	done := make(chan bool)
	port := 3000
	dhts := []*DHT{}
	for i := 0; i < 20; i++ {
		n, err := NewNode(MustNewID(), net.JoinHostPort("127.0.0.1", strconv.Itoa(port+i)))
		if !assert.NoError(t, err) {
			return
		}
		dhts = append(dhts, NewDHT(n))
	}

	for _, dht := range dhts {
		assert.Equal(t, 0, dht.NumNodes())
		go func(dht *DHT) {
			assert.Equal(t, "closed", dht.Listen().Error())
			done <- true
		}(dht)
		go func(dht *DHT, peers ...*DHT) {
			bs := make([]*NetworkNode, 0, len(peers))
			for _, b := range peers {
				if bytes.Compare(dht.ht.Self.ID, b.ht.Self.ID) != 0 {
					bs = append(bs, zeroNodeID(b.ht.Self))
				}
			}
			assert.NoError(t, dht.Bootstrap(bs...))
		}(dht, dhts...)
		time.Sleep(time.Millisecond * 200)
	}

	time.Sleep(2 * time.Second)
	fmt.Println("checking number of nodes")
	for _, dht := range dhts {
		assert.Equal(t, 19, dht.NumNodes())
		assert.NoError(t, dht.Disconnect())
		<-done
	}
}

// Creates two DHTs, bootstrap one using the other, ensure that they both know
// about each other afterwards.
func TestBootstrapTwoNodes(t *testing.T) {
	done := make(chan bool)
	dht1 := NewDHT(mustNode(MustNewID(), "127.0.0.1:3000"))
	dht2 := NewDHT(mustNode(MustNewID(), "127.0.0.1:3001"))

	assert.Equal(t, 0, dht1.NumNodes())
	assert.Equal(t, 0, dht2.NumNodes())

	go func() {
		go func() {
			assert.NoError(t, dht2.Bootstrap(zeroNodeID(dht1.ht.Self)))
			time.Sleep(50 * time.Millisecond)
			assert.NoError(t, dht2.Disconnect())
			assert.NoError(t, dht1.Disconnect())
			done <- true
		}()
		err := dht2.Listen()
		assert.Equal(t, "closed", err.Error())
		done <- true
	}()

	err := dht1.Listen()
	assert.Equal(t, "closed", err.Error())

	assert.Equal(t, 1, dht1.NumNodes())
	assert.Equal(t, 1, dht2.NumNodes())
	<-done
	<-done
}

// Creates three DHTs, bootstrap B using A, bootstrap C using B. A should know
// about both B and C
func TestBootstrapThreeNodes(t *testing.T) {
	done := make(chan bool)
	dht1 := NewDHT(mustNode(MustNewID(), "127.0.0.1:3000"))
	dht2 := NewDHT(mustNode(MustNewID(), "127.0.0.1:3001"))
	dht3 := NewDHT(mustNode(MustNewID(), "127.0.0.1:3002"))

	assert.Equal(t, 0, dht1.NumNodes())
	assert.Equal(t, 0, dht2.NumNodes())
	assert.Equal(t, 0, dht3.NumNodes())

	go func(dht1 *DHT, dht2 *DHT, dht3 *DHT) {
		go func(dht1 *DHT, dht2 *DHT, dht3 *DHT) {
			assert.NoError(t, dht2.Bootstrap(dht1.ht.Self))

			go func(dht1 *DHT, dht2 *DHT, dht3 *DHT) {
				assert.NoError(t, dht3.Bootstrap(dht2.ht.Self))
				time.Sleep(500 * time.Millisecond)

				assert.NoError(t, dht1.Disconnect())
				time.Sleep(100 * time.Millisecond)

				assert.NoError(t, dht2.Disconnect())
				assert.NoError(t, dht3.Disconnect())
				done <- true
			}(dht1, dht2, dht3)

			err := dht3.Listen()
			assert.Equal(t, "closed", err.Error())
			done <- true
		}(dht1, dht2, dht3)

		err := dht2.Listen()
		assert.Equal(t, "closed", err.Error())
		done <- true
	}(dht1, dht2, dht3)

	assert.Equal(t, "closed", dht1.Listen().Error())

	assert.Equal(t, 2, dht1.NumNodes())
	assert.Equal(t, 2, dht2.NumNodes())
	assert.Equal(t, 2, dht3.NumNodes())

	<-done
	<-done
	<-done
}

// Creates two DHTs and bootstraps using only IP:Port. Connecting node should
// ping the first node to find its ID
func TestBootstrapNoID(t *testing.T) {
	done := make(chan bool)
	dht1 := NewDHT(mustNode(MustNewID(), "127.0.0.1:3000"))
	dht2 := NewDHT(mustNode(MustNewID(), "127.0.0.1:3001"))

	assert.Equal(t, 0, dht1.NumNodes())
	assert.Equal(t, 0, dht2.NumNodes())

	go func() {
		go func() {
			assert.NoError(t, dht2.Bootstrap(zeroNodeID(dht1.ht.Self)))

			time.Sleep(50 * time.Millisecond)

			assert.NoError(t, dht2.Disconnect())
			assert.NoError(t, dht1.Disconnect())
			done <- true
		}()
		assert.Equal(t, "closed", dht2.Listen().Error())
		done <- true
	}()

	assert.Equal(t, "closed", dht1.Listen().Error())

	assert.Equal(t, 1, dht1.NumNodes())
	assert.Equal(t, 1, dht2.NumNodes())

	<-done
	<-done
}

// Create two DHTs have them connect and bootstrap, then disconnect. Repeat
// 100 times to ensure that we can use the same IP and port without EADDRINUSE
// errors.
func TestReconnect(t *testing.T) {
	for i := 0; i < 100; i++ {
		done := make(chan bool)
		dht1 := NewDHT(mustNode(MustNewID(), "127.0.0.1:3000"))
		dht2 := NewDHT(mustNode(MustNewID(), "127.0.0.1:3001"))

		assert.Equal(t, 0, dht1.NumNodes())

		go func() {
			go func() {
				assert.NoError(t, dht2.Bootstrap(dht1.ht.Self))
				assert.NoError(t, dht2.Disconnect())
				assert.NoError(t, dht1.Disconnect())
				done <- true
			}()

			assert.Equal(t, "closed", dht2.Listen().Error())
			done <- true
		}()

		err := dht1.Listen()
		assert.Equal(t, "closed", err.Error())

		assert.Equal(t, 1, dht1.NumNodes())
		assert.Equal(t, 1, dht2.NumNodes())

		<-done
		<-done
	}
}

// Create two DHTs and have them connect. Send a store message with 100mb
// payload from one node to another. Ensure that the other node now has
// this data in its store.
func TestStoreAndFindLargeValue(t *testing.T) {
	done := make(chan bool)
	dht1 := NewDHT(mustNode(MustNewID(), "127.0.0.1:3000"))
	dht2 := NewDHT(mustNode(MustNewID(), "127.0.0.1:3001"))

	go func() {
		err := dht1.Listen()
		assert.Equal(t, "closed", err.Error())
		done <- true
	}()

	go func() {
		err := dht2.Listen()
		assert.Equal(t, "closed", err.Error())
		done <- true
	}()

	time.Sleep(1 * time.Second)

	dht2.Bootstrap(dht1.ht.Self)

	payload := [1000000]byte{}
	key := ContentAddressable(payload[:])
	assert.NoError(t, dht1.Store(key, payload[:]))

	time.Sleep(1 * time.Second)

	value, exists, err := dht2.Get(key)
	assert.NoError(t, err)
	assert.Equal(t, true, exists)
	assert.Equal(t, 0, bytes.Compare(payload[:], value))
	assert.NoError(t, dht1.Disconnect())
	assert.NoError(t, dht2.Disconnect())

	<-done
	<-done
}

// Tests sending a message which results in an error when attempting to
// send over uTP
func TestNetworkingSendError(t *testing.T) {
	networking := newMockNetworking()
	done := make(chan int)
	dht := NewDHT(NetworkNode{ID: getIDWithValues(0), IP: net.ParseIP("127.0.0.1"), Port: 3000})
	dht.networking = networking

	go func() {
		dht.Listen()
	}()

	go func() {
		v := <-networking.recv
		assert.Nil(t, v)
		close(done)
	}()

	networking.failNextSendMessage()

	dht.Bootstrap(&NetworkNode{
		ID:   getZerodIDWithNthByte(1, byte(255)),
		Port: 3001,
		IP:   net.ParseIP("0.0.0.0"),
	})

	dht.Disconnect()

	<-done
}

// Tests sending a message which results in a successful send, but the node
// never responds
func TestNodeResponseSendError(t *testing.T) {
	networking := newMockNetworking()
	done := make(chan (int))

	dht := NewDHT(NetworkNode{ID: getIDWithValues(0), IP: net.ParseIP("127.0.0.1"), Port: 3000})
	dht.networking = networking

	queries := 0

	go func() {
		dht.Listen()
	}()

	go func() {
		for {
			query := <-networking.recv
			if query == nil {
				return
			}
			if queries == 1 {
				// Don't respond
				close(done)
			} else {
				queries++
				res := mockFindNodeResponse(query, getZerodIDWithNthByte(2, byte(255)))
				networking.send <- res
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

	assert.Equal(t, 1, dht.ht.totalNodes())

	dht.Disconnect()

	<-done
}

// Tests a bucket refresh by setting a very low TRefresh value, adding a single
// node to a bucket, and waiting for the refresh message for the bucket
func TestBucketRefresh(t *testing.T) {
	networking := newMockNetworking()
	done := make(chan (int))
	refresh := make(chan (int))

	dht := NewDHT(
		NetworkNode{ID: getIDWithValues(0), IP: net.ParseIP("127.0.0.1"), Port: 3000},
		OptionRefresh(time.Second),
	)
	dht.networking = networking

	queries := 0

	go dht.Listen()

	go func() {
		for {
			query := <-networking.recv
			if query == nil {
				close(done)
				return
			}
			queries++

			res := mockFindNodeResponseEmpty(query)
			networking.send <- res

			if queries == 2 {
				close(refresh)
			}
		}
	}()

	assert.NoError(
		t,
		dht.Bootstrap(
			&NetworkNode{
				ID:   getZerodIDWithNthByte(1, byte(255)),
				Port: 3001,
				IP:   net.ParseIP("0.0.0.0"),
			},
		),
	)

	assert.Equal(t, 1, dht.ht.totalNodes())
	<-refresh
	assert.NoError(t, dht.Disconnect())
	<-done
}

// Tets store replication by setting the TReplicate time to a very small value.
// Stores some data, and then expects another store message in TReplicate time
func TestStoreReplication(t *testing.T) {
	networking := newMockNetworking()
	done := make(chan (int))
	replicate := make(chan (int))

	dht := NewDHT(
		NetworkNode{ID: getIDWithValues(0), IP: net.ParseIP("127.0.0.1"), Port: 3000},
		OptionReplicate(time.Second),
	)
	dht.networking = networking

	go func() {
		dht.Listen()
	}()

	stores := 0

	go func() {
		for {
			query := <-networking.recv
			if query == nil {
				close(done)
				return
			}

			switch query.Type {
			case messageTypeFindNode:
				res := mockFindNodeResponseEmpty(query)
				networking.send <- res
			case messageTypeStore:
				stores++
				d := query.Data.(*queryDataStore)
				assert.Equal(t, []byte("foo"), d.Data)
				if stores == 2 {
					close(replicate)
				}
			}
		}
	}()

	// TODO: remove check if it causes test to fail.
	assert.NoError(
		t,
		dht.Bootstrap(
			&NetworkNode{
				ID:   getZerodIDWithNthByte(1, byte(255)),
				Port: 3001,
				IP:   net.ParseIP("0.0.0.0"),
			},
		),
	)

	payload := []byte("foo")
	dht.Store(ContentAddressable(payload), payload)

	<-replicate

	dht.Disconnect()

	<-done
}

// Test Expiration by setting TExpire to a very low value. Store a value,
// and then wait longer than TExpire. The value should no longer exist in
// the store.
func TestStoreExpiration(t *testing.T) {
	dht := NewDHT(
		mustNode(getIDWithValues(0), "127.0.0.1:3000"),
		OptionExpire(time.Second),
	)

	go dht.Listen()

	payload := []byte("foo")
	k := ContentAddressable(payload)
	assert.NoError(t, dht.Store(k, payload))

	v, exists, _ := dht.Get(k)
	assert.Equal(t, true, exists)

	assert.Equal(t, []byte("foo"), v)

	<-time.After(time.Second * 3)

	_, exists, _ = dht.Get(k)

	assert.Equal(t, false, exists)
	assert.NoError(t, dht.Disconnect())
}

func zeroNodeID(n *NetworkNode) *NetworkNode {
	return &NetworkNode{
		IP:   n.IP,
		Port: n.Port,
	}
}
