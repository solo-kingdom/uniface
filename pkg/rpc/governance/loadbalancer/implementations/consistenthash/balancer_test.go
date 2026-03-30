// 基于 prompts/features/service/governance/load-balancer/01-load-balancer-iface.md 实现
package consistenthash

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"sync"
	"testing"

	"github.com/solo-kingdom/uniface/pkg/rpc/governance/loadbalancer"
)

// MockClient is a mock client for testing
type MockClient struct {
	ID     string
	Closed bool
}

// Close implements io.Closer
func (c *MockClient) Close() error {
	c.Closed = true
	return nil
}

// Ensure MockClient implements io.Closer
var _ io.Closer = (*MockClient)(nil)

func TestConsistentHashBalancer_SelectByKey(t *testing.T) {
	lb := New[*MockClient](100, nil)
	ctx := context.Background()

	// Add instances
	instances := []*loadbalancer.Instance{
		{ID: "A", Address: "192.168.1.1", Port: 8080},
		{ID: "B", Address: "192.168.1.2", Port: 8080},
		{ID: "C", Address: "192.168.1.3", Port: 8080},
	}

	for _, inst := range instances {
		if err := lb.Add(ctx, inst); err != nil {
			t.Fatalf("Add failed: %v", err)
		}
	}

	t.Run("stable routing", func(t *testing.T) {
		// Same key should always route to same instance
		key := "user-123"
		firstInstance, err := lb.Select(ctx, loadbalancer.WithKey(key))
		if err != nil {
			t.Fatalf("Select failed: %v", err)
		}

		// Select 100 times with same key
		for i := 0; i < 100; i++ {
			inst, err := lb.Select(ctx, loadbalancer.WithKey(key))
			if err != nil {
				t.Fatalf("Select failed: %v", err)
			}
			if inst.ID != firstInstance.ID {
				t.Errorf("Key %s routed to different instances: %s vs %s",
					key, firstInstance.ID, inst.ID)
			}
		}
	})

	t.Run("different keys distribution", func(t *testing.T) {
		// Different keys should distribute across instances
		results := make(map[string]int)
		numKeys := 1000

		for i := 0; i < numKeys; i++ {
			key := fmt.Sprintf("key-%d", i)
			inst, err := lb.Select(ctx, loadbalancer.WithKey(key))
			if err != nil {
				t.Fatalf("Select failed: %v", err)
			}
			results[inst.ID]++
		}

		// Each instance should get approximately 1/3 of keys (333 each)
		// Allow 40% variance for consistent hash (it's not perfectly uniform)
		expectedPerInstance := numKeys / 3
		minExpected := int(float64(expectedPerInstance) * 0.6)
		maxExpected := int(float64(expectedPerInstance) * 1.4)

		for id, count := range results {
			if count < minExpected || count > maxExpected {
				t.Errorf("Instance %s got %d keys, expected %d-%d",
					id, count, minExpected, maxExpected)
			}
		}
	})

	t.Run("minimal remapping on instance removal", func(t *testing.T) {
		// Record initial mapping for many keys
		keys := make([]string, 1000)
		initialMapping := make(map[string]string)

		for i := 0; i < 1000; i++ {
			key := fmt.Sprintf("key-%d", i)
			keys[i] = key
			inst, _ := lb.Select(ctx, loadbalancer.WithKey(key))
			initialMapping[key] = inst.ID
		}

		// Remove one instance
		lb.Remove(ctx, "B")

		// Check how many keys were remapped
		remapped := 0
		for _, key := range keys {
			inst, err := lb.Select(ctx, loadbalancer.WithKey(key))
			if err != nil {
				t.Fatalf("Select failed: %v", err)
			}
			if inst.ID != initialMapping[key] {
				remapped++
			}
		}

		// Only keys that mapped to B should be remapped
		// With 3 instances, ~1/3 should be remapped (~333)
		// Allow larger variance for consistent hash (200-466)
		if remapped < 200 || remapped > 466 {
			t.Errorf("Unexpected remapping: %d keys remapped (expected 200-466)", remapped)
		}
	})
}

func TestConsistentHashBalancer_SelectRoundRobin(t *testing.T) {
	lb := New[*MockClient](100, nil)
	ctx := context.Background()

	// Add instances
	instances := []*loadbalancer.Instance{
		{ID: "A", Address: "192.168.1.1", Port: 8080},
		{ID: "B", Address: "192.168.1.2", Port: 8080},
		{ID: "C", Address: "192.168.1.3", Port: 8080},
	}

	for _, inst := range instances {
		if err := lb.Add(ctx, inst); err != nil {
			t.Fatalf("Add failed: %v", err)
		}
	}

	t.Run("round-robin fallback", func(t *testing.T) {
		// Without key, should use round-robin
		results := make(map[string]int)
		for i := 0; i < 12; i++ {
			inst, err := lb.Select(ctx)
			if err != nil {
				t.Fatalf("Select failed: %v", err)
			}
			results[inst.ID]++
		}

		// Each instance should be selected exactly 4 times
		for id, count := range results {
			if count != 4 {
				t.Errorf("Instance %s selected %d times, expected 4", id, count)
			}
		}
	})

	t.Run("select with filter", func(t *testing.T) {
		// Filter to only select instance A
		filter := loadbalancer.WithFilter(func(inst *loadbalancer.Instance) bool {
			return inst.ID == "A"
		})

		for i := 0; i < 10; i++ {
			inst, err := lb.Select(ctx, filter)
			if err != nil {
				t.Fatalf("Select with filter failed: %v", err)
			}
			if inst.ID != "A" {
				t.Errorf("Expected instance A, got %s", inst.ID)
			}
		}
	})
}

func TestConsistentHashBalancer_SelectClient(t *testing.T) {
	lb := New[*MockClient](100, nil)
	ctx := context.Background()

	inst := &loadbalancer.Instance{
		ID:      "test-1",
		Address: "192.168.1.1",
		Port:    8080,
	}
	lb.Add(ctx, inst)

	factory := loadbalancer.WithClientFactory(func(inst *loadbalancer.Instance) (*MockClient, error) {
		return &MockClient{ID: inst.ID}, nil
	})

	t.Run("create and cache client with key", func(t *testing.T) {
		// First call - create client
		client1, err := lb.SelectClient(ctx,
			loadbalancer.WithKey("user-1"),
			factory,
		)
		if err != nil {
			t.Fatalf("SelectClient failed: %v", err)
		}

		// Second call with same key - reuse client
		client2, err := lb.SelectClient(ctx,
			loadbalancer.WithKey("user-1"),
			factory,
		)
		if err != nil {
			t.Fatalf("SelectClient failed: %v", err)
		}

		// Verify same client instance
		if client1 != client2 {
			t.Error("Expected same client instance")
		}
	})

	t.Run("select without factory", func(t *testing.T) {
		lb2 := New[*MockClient](100, nil)
		lb2.Add(ctx, inst)

		_, err := lb2.SelectClient(ctx, loadbalancer.WithKey("user-1"))
		if err != loadbalancer.ErrNoClientFactory {
			t.Errorf("Expected ErrNoClientFactory, got %v", err)
		}
	})
}

func TestConsistentHashBalancer_AddRemove(t *testing.T) {
	lb := New[*MockClient](50, nil)
	ctx := context.Background()

	// Add instances
	for i := 0; i < 5; i++ {
		inst := &loadbalancer.Instance{
			ID:      string(rune('A' + i)),
			Address: "192.168.1.1",
			Port:    8080 + i,
		}
		if err := lb.Add(ctx, inst); err != nil {
			t.Fatalf("Add failed: %v", err)
		}
	}

	// Verify all instances are in the ring
	if lb.ring.Len() != 5 {
		t.Errorf("Expected 5 instances in ring, got %d", lb.ring.Len())
	}

	// Remove an instance
	err := lb.Remove(ctx, "B")
	if err != nil {
		t.Fatalf("Remove failed: %v", err)
	}

	// Verify instance is removed from ring
	if lb.ring.Len() != 4 {
		t.Errorf("Expected 4 instances in ring, got %d", lb.ring.Len())
	}

	if lb.ring.Contains("B") {
		t.Error("Instance B should not be in ring")
	}

	// Try to remove non-existent instance
	err = lb.Remove(ctx, "Z")
	if err == nil {
		t.Error("Expected error for non-existent instance")
	}
}

func TestConsistentHashBalancer_Close(t *testing.T) {
	lb := New[*MockClient](100, nil)
	ctx := context.Background()

	// Add instances
	for i := 1; i <= 3; i++ {
		inst := &loadbalancer.Instance{
			ID:      string(rune('A' + i - 1)),
			Address: "192.168.1.1",
			Port:    8080 + i,
		}
		lb.Add(ctx, inst)
	}

	factory := loadbalancer.WithClientFactory(func(inst *loadbalancer.Instance) (*MockClient, error) {
		return &MockClient{ID: inst.ID}, nil
	})

	// Create clients
	clients := make([]*MockClient, 3)
	for i := 0; i < 3; i++ {
		client, err := lb.SelectClient(ctx,
			loadbalancer.WithKey(fmt.Sprintf("key-%d", i)),
			factory,
		)
		if err != nil {
			t.Fatalf("SelectClient failed: %v", err)
		}
		clients[i] = client
	}

	// Close load balancer
	err := lb.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Verify all clients are closed
	for _, client := range clients {
		if !client.Closed {
			t.Error("Expected client to be closed")
		}
	}

	// Verify operations fail after close
	_, err = lb.Select(ctx, loadbalancer.WithKey("test"))
	if err != loadbalancer.ErrBalancerClosed {
		t.Errorf("Expected ErrBalancerClosed, got %v", err)
	}
}

func TestConsistentHashBalancer_Concurrency(t *testing.T) {
	lb := New[*MockClient](100, nil)
	ctx := context.Background()

	// Add instances
	for i := 0; i < 10; i++ {
		inst := &loadbalancer.Instance{
			ID:      string(rune('A' + i)),
			Address: "192.168.1.1",
			Port:    8080 + i,
		}
		lb.Add(ctx, inst)
	}

	factory := loadbalancer.WithClientFactory(func(inst *loadbalancer.Instance) (*MockClient, error) {
		return &MockClient{ID: inst.ID}, nil
	})

	// Concurrent operations
	const goroutines = 100
	const operations = 100

	var wg sync.WaitGroup
	errors := make(chan error, goroutines*operations*2)

	// Concurrent Select with keys
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < operations; j++ {
				key := fmt.Sprintf("key-%d", j)
				_, err := lb.Select(ctx, loadbalancer.WithKey(key))
				if err != nil {
					errors <- err
				}
			}
		}()
	}

	// Concurrent SelectClient with keys
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < operations; j++ {
				key := fmt.Sprintf("key-%d", j)
				_, err := lb.SelectClient(ctx,
					loadbalancer.WithKey(key),
					factory,
				)
				if err != nil {
					errors <- err
				}
			}
		}()
	}

	wg.Wait()
	close(errors)

	// Check for errors
	for err := range errors {
		t.Errorf("Concurrent operation failed: %v", err)
	}
}

func TestRing_Basic(t *testing.T) {
	ring := NewRing(50, nil)

	// Add instances
	ring.Add("A")
	ring.Add("B")
	ring.Add("C")

	// Test that keys map consistently
	key := "test-key"
	instance1 := ring.Get(key)
	instance2 := ring.Get(key)

	if instance1 != instance2 {
		t.Errorf("Inconsistent mapping: %s -> %s vs %s", key, instance1, instance2)
	}

	// Test different keys map to potentially different instances
	mapping := make(map[string]string)
	for i := 0; i < 100; i++ {
		key := fmt.Sprintf("key-%d", i)
		inst := ring.Get(key)
		mapping[key] = inst
	}

	// Count distribution
	counts := make(map[string]int)
	for _, inst := range mapping {
		counts[inst]++
	}

	// Each instance should get some keys
	for _, instID := range []string{"A", "B", "C"} {
		if counts[instID] == 0 {
			t.Errorf("Instance %s got no keys", instID)
		}
	}
}

func TestRing_Remove(t *testing.T) {
	ring := NewRing(50, nil)

	ring.Add("A")
	ring.Add("B")
	ring.Add("C")

	// Record mapping before removal
	key := "test-key"
	before := ring.Get(key)

	// Remove an instance
	ring.Remove("B")

	// Get mapping after removal
	after := ring.Get(key)

	// If before was B, after should be different
	// If before was not B, after should be the same
	if before == "B" {
		if after == "B" {
			t.Error("Key should not map to removed instance")
		}
	} else {
		if after != before {
			t.Errorf("Key remapped unnecessarily: %s -> %s", before, after)
		}
	}

	// Verify B is removed
	if ring.Contains("B") {
		t.Error("Instance B should be removed from ring")
	}
}

func TestRing_Empty(t *testing.T) {
	ring := NewRing(50, nil)

	// Empty ring should return empty string
	inst := ring.Get("test-key")
	if inst != "" {
		t.Errorf("Empty ring should return empty string, got %s", inst)
	}
}

func TestRing_VirtualNodes(t *testing.T) {
	// Test with different number of virtual nodes
	ring1 := NewRing(10, nil)
	ring2 := NewRing(100, nil)

	ring1.Add("A")
	ring2.Add("A")

	// Both should map keys, but distribution may differ
	key := "test-key"
	inst1 := ring1.Get(key)
	inst2 := ring2.Get(key)

	// Both should map to A (only one instance)
	if inst1 != "A" || inst2 != "A" {
		t.Errorf("Expected both to map to A, got %s and %s", inst1, inst2)
	}
}

func TestConsistentHashBalancer_EmptyBalancer(t *testing.T) {
	lb := New[*MockClient](100, nil)
	ctx := context.Background()

	// Select from empty balancer with key
	_, err := lb.Select(ctx, loadbalancer.WithKey("test"))
	if err != loadbalancer.ErrNoInstances {
		t.Errorf("Expected ErrNoInstances, got %v", err)
	}

	// Select from empty balancer without key
	_, err = lb.Select(ctx)
	if err != loadbalancer.ErrNoInstances {
		t.Errorf("Expected ErrNoInstances, got %v", err)
	}
}

// Benchmark tests
func BenchmarkConsistentHashBalancer_Select(b *testing.B) {
	lb := New[*MockClient](100, nil)
	ctx := context.Background()

	// Add instances
	for i := 0; i < 10; i++ {
		inst := &loadbalancer.Instance{
			ID:      strconv.Itoa(i),
			Address: "192.168.1.1",
			Port:    8080 + i,
		}
		lb.Add(ctx, inst)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := fmt.Sprintf("key-%d", i)
			lb.Select(ctx, loadbalancer.WithKey(key))
			i++
		}
	})
}
