package kademlia

import (
	"bytes"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSerializeNetMsg(t *testing.T) {
	var conn bytes.Buffer

	node := &NetworkNode{
		ID:   MustNewID(),
		Port: 3000,
		IP:   net.ParseIP("0.0.0.0"),
	}

	msg := &Message{
		Type:     messageTypeFindNode,
		Receiver: node,
		Data: &queryDataFindNode{
			Target: node.ID,
		},
	}

	serialized, err := serializeMessage(msg)
	if err != nil {
		panic(err)
	}

	conn.Write(serialized)

	deserialized, err := deserializeMessage(&conn)
	if err != nil {
		panic(err)
	}

	assert.Equal(t, msg, deserialized)
}
