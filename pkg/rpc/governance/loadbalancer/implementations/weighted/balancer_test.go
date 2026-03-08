// 基于 prompts/features/service/governance/load-balancer/01-load-balancer-iface.md 实现
package weighted

import (
	"context"
	"io"
	"sync"
	"testing"

	"github.com/wii/uniface/pkg/rpc/governance/loadbalancer"
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

func TestWeightedBalancer_Select(t *testing.T) {
	lb := New[*MockClient]()
	ctx := context.Background()

	// Add instances with different weights
	instances := []*loadbalancer.Instance{
		{ID: "A", Address: "192.168.1.1", Port: 8080, Weight: 5},
		{ID: "B", Address: "192.168.1.2", Port: 8080, Weight: 1},
		{ID: "C", Address: "192.168.1.3", Port: 8080, Weight: 1},
	}

	for _, inst := range instances {
		if err := lb.Add(ctx, inst); err != nil {
			t.Fatalf("Add failed: %v", err)
		}
	}

	t.Run("weighted distribution", func(t *testing.T) {
		// Total weight = 5 + 1 + 1 = 7
		// Expected distribution: A:5/7, B:1/7, C:1/7
		results := make(map[string]int)
		iterations := 700 // 100 * total weight for easy calculation

		for i := 0; i < iterations; i++ {
			inst, err := lb.Select(ctx)
			if err != nil {
				t.Fatalf("Select failed: %v", err)
			}
			results[inst.ID]++
		}

		// A should be selected ~500 times (5/7 * 700)
		// B should be selected ~100 times (1/7 * 700)
		// C should be selected ~100 times (1/7 * 700)
		// Allow 10% variance
		expectedA := 500
		expectedBC := 100

		if results["A"] < expectedA-50 || results["A"] > expectedA+50 {
			t.Errorf("Instance A selected %d times, expected %d±50", results["A"], expectedA)
		}
		if results["B"] < expectedBC-10 || results["B"] > expectedBC+10 {
			t.Errorf("Instance B selected %d times, expected %d±10", results["B"], expectedBC)
		}
		if results["C"] < expectedBC-10 || results["C"] > expectedBC+10 {
			t.Errorf("Instance C selected %d times, expected %d±10", results["C"], expectedBC)
		}
	})

	t.Run("smooth distribution", func(t *testing.T) {
		// Test that distribution is smoother than simple weighted round-robin
		// With weights 5, 1, 1, smooth distribution should intersperse B among A's
		// Not all A's followed by all B's

		lb2 := New[*MockClient]()
		lb2.Add(ctx, &loadbalancer.Instance{ID: "A", Address: "192.168.1.1", Port: 8080, Weight: 5})
		lb2.Add(ctx, &loadbalancer.Instance{ID: "B", Address: "192.168.1.2", Port: 8080, Weight: 1})

		// Track selection sequence
		sequence := make([]string, 0, 12)
		for i := 0; i < 12; i++ {
			inst, _ := lb2.Select(ctx)
			sequence = append(sequence, inst.ID)
		}

		// Count how many times B appears (should appear roughly 2 times in 12 selections)
		countB := 0
		for _, id := range sequence {
			if id == "B" {
				countB++
			}
		}

		// B should appear at least once (weight ratio 5:1 means B gets 1/6 of traffic)
		if countB < 1 {
			t.Errorf("Distribution not smooth: B appears %d times in sequence %v", countB, sequence)
		}

		// Log the sequence for debugging
		t.Logf("Selection sequence: %v (B count: %d)", sequence, countB)
	})

	t.Run("select with filter", func(t *testing.T) {
		// Filter to only select instance B
		filter := loadbalancer.WithFilter(func(inst *loadbalancer.Instance) bool {
			return inst.ID == "B"
		})

		for i := 0; i < 10; i++ {
			inst, err := lb.Select(ctx, filter)
			if err != nil {
				t.Fatalf("Select with filter failed: %v", err)
			}
			if inst.ID != "B" {
				t.Errorf("Expected instance B, got %s", inst.ID)
			}
		}
	})

	t.Run("select with filter that excludes all", func(t *testing.T) {
		// Filter that excludes all instances
		filter := loadbalancer.WithFilter(func(inst *loadbalancer.Instance) bool {
			return false
		})

		_, err := lb.Select(ctx, filter)
		if err != loadbalancer.ErrNoInstances {
			t.Errorf("Expected ErrNoInstances, got %v", err)
		}
	})

	t.Run("select from empty balancer", func(t *testing.T) {
		emptyLb := New[*MockClient]()
		_, err := emptyLb.Select(ctx)
		if err != loadbalancer.ErrNoInstances {
			t.Errorf("Expected ErrNoInstances, got %v", err)
		}
	})
}

func TestWeightedBalancer_DefaultWeight(t *testing.T) {
	lb := New[*MockClient]()
	ctx := context.Background()

	// Add instance without specifying weight (should default to 1)
	inst := &loadbalancer.Instance{
		ID:      "test",
		Address: "192.168.1.1",
		Port:    8080,
		// Weight not specified
	}
	lb.Add(ctx, inst)

	// Add another instance with explicit weight
	inst2 := &loadbalancer.Instance{
		ID:      "test2",
		Address: "192.168.1.2",
		Port:    8080,
		Weight:  1,
	}
	lb.Add(ctx, inst2)

	// Both should have equal weight (1)
	results := make(map[string]int)
	for i := 0; i < 100; i++ {
		selected, err := lb.Select(ctx)
		if err != nil {
			t.Fatalf("Select failed: %v", err)
		}
		results[selected.ID]++
	}

	// Both should be selected approximately 50 times each
	if results["test"] < 40 || results["test"] > 60 {
		t.Errorf("Instance 'test' selected %d times, expected ~50", results["test"])
	}
	if results["test2"] < 40 || results["test2"] > 60 {
		t.Errorf("Instance 'test2' selected %d times, expected ~50", results["test2"])
	}
}

func TestWeightedBalancer_ZeroWeight(t *testing.T) {
	lb := New[*MockClient]()
	ctx := context.Background()

	// Add instance with zero weight (should be treated as 1)
	inst := &loadbalancer.Instance{
		ID:      "zero",
		Address: "192.168.1.1",
		Port:    8080,
		Weight:  0,
	}
	lb.Add(ctx, inst)

	// Should still be selectable
	selected, err := lb.Select(ctx)
	if err != nil {
		t.Fatalf("Select failed: %v", err)
	}
	if selected.ID != "zero" {
		t.Errorf("Expected 'zero', got %s", selected.ID)
	}
}

func TestWeightedBalancer_SelectClient(t *testing.T) {
	lb := New[*MockClient]()
	ctx := context.Background()

	inst := &loadbalancer.Instance{
		ID:      "test-1",
		Address: "192.168.1.1",
		Port:    8080,
		Weight:  5,
	}
	lb.Add(ctx, inst)

	factory := loadbalancer.WithClientFactory(func(inst *loadbalancer.Instance) (*MockClient, error) {
		return &MockClient{ID: inst.ID}, nil
	})

	t.Run("create and cache client", func(t *testing.T) {
		// First call - create client
		client1, err := lb.SelectClient(ctx, factory)
		if err != nil {
			t.Fatalf("SelectClient failed: %v", err)
		}

		if client1.ID != "test-1" {
			t.Errorf("Expected client ID 'test-1', got '%s'", client1.ID)
		}

		// Second call - reuse client
		client2, err := lb.SelectClient(ctx, factory)
		if err != nil {
			t.Fatalf("SelectClient failed: %v", err)
		}

		// Verify same client instance
		if client1 != client2 {
			t.Error("Expected same client instance")
		}
	})

	t.Run("select without factory", func(t *testing.T) {
		lb2 := New[*MockClient]()
		lb2.Add(ctx, inst)

		_, err := lb2.SelectClient(ctx)
		if err != loadbalancer.ErrNoClientFactory {
			t.Errorf("Expected ErrNoClientFactory, got %v", err)
		}
	})
}

func TestWeightedBalancer_Update(t *testing.T) {
	lb := New[*MockClient]()
	ctx := context.Background()

	inst := &loadbalancer.Instance{
		ID:      "test-1",
		Address: "192.168.1.1",
		Port:    8080,
		Weight:  5,
	}
	lb.Add(ctx, inst)

	// Update weight
	updated := &loadbalancer.Instance{
		ID:      "test-1",
		Address: "192.168.1.1",
		Port:    8080,
		Weight:  10,
	}
	err := lb.Update(ctx, updated)
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	// Add another instance for comparison
	inst2 := &loadbalancer.Instance{
		ID:      "test-2",
		Address: "192.168.1.2",
		Port:    8080,
		Weight:  1,
	}
	lb.Add(ctx, inst2)

	// Verify weight update affects distribution
	results := make(map[string]int)
	for i := 0; i < 110; i++ { // Total weight = 10 + 1 = 11
		selected, err := lb.Select(ctx)
		if err != nil {
			t.Fatalf("Select failed: %v", err)
		}
		results[selected.ID]++
	}

	// test-1 should be selected ~100 times (10/11 * 110)
	// test-2 should be selected ~10 times (1/11 * 110)
	if results["test-1"] < 90 || results["test-1"] > 110 {
		t.Errorf("Instance test-1 selected %d times, expected ~100", results["test-1"])
	}
	if results["test-2"] < 5 || results["test-2"] > 15 {
		t.Errorf("Instance test-2 selected %d times, expected ~10", results["test-2"])
	}
}

func TestWeightedBalancer_Remove(t *testing.T) {
	lb := New[*MockClient]()
	ctx := context.Background()

	inst := &loadbalancer.Instance{
		ID:      "test-1",
		Address: "192.168.1.1",
		Port:    8080,
		Weight:  5,
	}
	lb.Add(ctx, inst)

	factory := loadbalancer.WithClientFactory(func(inst *loadbalancer.Instance) (*MockClient, error) {
		return &MockClient{ID: inst.ID}, nil
	})

	// Create client
	client, err := lb.SelectClient(ctx, factory)
	if err != nil {
		t.Fatalf("SelectClient failed: %v", err)
	}

	// Remove instance
	err = lb.Remove(ctx, "test-1")
	if err != nil {
		t.Fatalf("Remove failed: %v", err)
	}

	// Verify client was closed
	if !client.Closed {
		t.Error("Expected client to be closed")
	}

	// Verify instance is removed
	_, err = lb.Select(ctx)
	if err != loadbalancer.ErrNoInstances {
		t.Errorf("Expected ErrNoInstances, got %v", err)
	}
}

func TestWeightedBalancer_Close(t *testing.T) {
	lb := New[*MockClient]()
	ctx := context.Background()

	// Add instances
	for i := 1; i <= 3; i++ {
		inst := &loadbalancer.Instance{
			ID:      string(rune('A' + i - 1)),
			Address: "192.168.1.1",
			Port:    8080 + i,
			Weight:  i,
		}
		lb.Add(ctx, inst)
	}

	factory := loadbalancer.WithClientFactory(func(inst *loadbalancer.Instance) (*MockClient, error) {
		return &MockClient{ID: inst.ID}, nil
	})

	// Create clients for all instances
	clients := make([]*MockClient, 3)
	for i := 0; i < 3; i++ {
		client, err := lb.SelectClient(ctx, factory)
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
	_, err = lb.Select(ctx)
	if err != loadbalancer.ErrBalancerClosed {
		t.Errorf("Expected ErrBalancerClosed, got %v", err)
	}
}

func TestWeightedBalancer_Concurrency(t *testing.T) {
	lb := New[*MockClient]()
	ctx := context.Background()

	// Add instances with different weights
	for i := 0; i < 5; i++ {
		inst := &loadbalancer.Instance{
			ID:      string(rune('A' + i)),
			Address: "192.168.1.1",
			Port:    8080 + i,
			Weight:  i + 1, // Weights: 1, 2, 3, 4, 5
		}
		lb.Add(ctx, inst)
	}

	factory := loadbalancer.WithClientFactory(func(inst *loadbalancer.Instance) (*MockClient, error) {
		return &MockClient{ID: inst.ID}, nil
	})

	// Concurrent Select operations
	const goroutines = 50
	const operations = 50

	var wg sync.WaitGroup
	errors := make(chan error, goroutines*operations)

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < operations; j++ {
				_, err := lb.Select(ctx)
				if err != nil {
					errors <- err
				}
			}
		}()
	}

	// Concurrent SelectClient operations
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < operations; j++ {
				_, err := lb.SelectClient(ctx, factory)
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
