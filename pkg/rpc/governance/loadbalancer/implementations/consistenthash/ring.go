// Package consistenthash provides a consistent hash load balancer implementation.
// This file implements the hash ring data structure.
//
// 基于 prompts/features/service/governance/load-balancer/01-load-balancer-iface.md 实现
package consistenthash

import (
	"hash/crc32"
	"sort"
	"strconv"
	"sync"
)

// HashFunc defines the hash function type.
type HashFunc func(data []byte) uint32

// Ring represents a consistent hash ring.
// It maps keys to instances in a way that minimizes remapping when instances are added or removed.
type Ring struct {
	mu        sync.RWMutex
	hashFunc  HashFunc
	replicas  int               // Number of virtual nodes per instance
	ring      []uint32          // Sorted hash ring
	hashMap   map[uint32]string // hash -> instanceID
	instances map[string]bool   // Set of instance IDs
}

// NewRing creates a new hash ring.
// replicas is the number of virtual nodes per instance (default 50).
// hashFunc is the hash function to use (default CRC32).
func NewRing(replicas int, hashFunc HashFunc) *Ring {
	if replicas <= 0 {
		replicas = 50
	}
	if hashFunc == nil {
		hashFunc = crc32.ChecksumIEEE
	}

	return &Ring{
		hashFunc:  hashFunc,
		replicas:  replicas,
		hashMap:   make(map[uint32]string),
		instances: make(map[string]bool),
	}
}

// Add adds an instance to the ring.
// It creates multiple virtual nodes for better distribution.
func (r *Ring) Add(instanceID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Skip if already exists
	if r.instances[instanceID] {
		return
	}

	r.instances[instanceID] = true

	// Add virtual nodes
	for i := 0; i < r.replicas; i++ {
		hash := r.hashFunc([]byte(r.virtualNodeKey(instanceID, i)))
		r.hashMap[hash] = instanceID
		r.ring = append(r.ring, hash)
	}

	// Sort the ring
	sort.Slice(r.ring, func(i, j int) bool {
		return r.ring[i] < r.ring[j]
	})
}

// Remove removes an instance from the ring.
func (r *Ring) Remove(instanceID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Skip if doesn't exist
	if !r.instances[instanceID] {
		return
	}

	delete(r.instances, instanceID)

	// Remove virtual nodes
	for i := 0; i < r.replicas; i++ {
		hash := r.hashFunc([]byte(r.virtualNodeKey(instanceID, i)))
		delete(r.hashMap, hash)
	}

	// Rebuild ring
	r.ring = r.ring[:0]
	for hash := range r.hashMap {
		r.ring = append(r.ring, hash)
	}

	// Sort the ring
	sort.Slice(r.ring, func(i, j int) bool {
		return r.ring[i] < r.ring[j]
	})
}

// Get gets the instance ID for a given key.
// It returns the instance that the key hashes to.
func (r *Ring) Get(key string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(r.ring) == 0 {
		return ""
	}

	// Hash the key
	hash := r.hashFunc([]byte(key))

	// Binary search for the first node with hash >= key hash
	idx := sort.Search(len(r.ring), func(i int) bool {
		return r.ring[i] >= hash
	})

	// Wrap around to the first node if necessary
	if idx == len(r.ring) {
		idx = 0
	}

	return r.hashMap[r.ring[idx]]
}

// Contains checks if an instance exists in the ring.
func (r *Ring) Contains(instanceID string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.instances[instanceID]
}

// Len returns the number of instances in the ring.
func (r *Ring) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.instances)
}

// virtualNodeKey generates a key for a virtual node.
func (r *Ring) virtualNodeKey(instanceID string, replicaNum int) string {
	// Use a simple concatenation with a number for virtual nodes
	// This ensures different hash values for each replica
	return instanceID + "#" + strconv.Itoa(replicaNum)
}
