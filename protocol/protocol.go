// Package protocol implements the network
// layer of the kademlia protocol.
package protocol

//go:generate protoc --go_out=plugins=grpc:. kademlia.proto
