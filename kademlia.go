package kademlia

import "crypto/sha1"

// ContentAddressable returns the key of the provided data.
// uses sha1.
func ContentAddressable(data []byte) []byte {
	sha := sha1.Sum(data)
	return sha[:]
}
