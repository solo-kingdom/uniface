// 基于 prompts/features/service/governance/load-balancer/01-load-balancer-iface.md 实现
package random

import (
	"context"
	"io"
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

func TestRandomBalancer_NewWithSeed(t *testing.T) {
	// Test deterministic behavior with seed
	lb1 := NewWithSeed[*MockClient](12345)
	lb2 := NewWithSeed[*MockClient](12345)

	ctx := context.Background()

	// Add same instances to both
	for i := 0; i < 5; i++ {
		inst := &loadbalancer.Instance{
			ID:      string(rune('A' + i)),
			Address: "192.168.1.1",
			Port:    8080 + i,
		}
		lb1.Add(ctx, inst)
		lb2.Add(ctx, inst)
	}

	// Both should produce same sequence
	for i := 0; i < 10; i++ {
		inst1, err := lb1.Select(ctx)
		if err != nil {
			t.Fatalf("lb1.Select failed: %v", err)
		}

		inst2, err := lb2.Select(ctx)
		if err != nil {
			t.Fatalf("lb2.Select failed: %v", err)
		}

		if inst1.ID != inst2.ID {
			t.Errorf("Expected same selection with same seed, got %s vs %s", inst1.ID, inst2.ID)
		}
	}
}

func TestRandomBalancer_Select(t *testing.T) {
	lb := New[*MockClient]()
	ctx := context.Background()

	// Add 3 instances
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

	t.Run("random distribution", func(t *testing.T) {
		// Select many times and check distribution
		results := make(map[string]int)
		iterations := 3000 // Large enough for statistical significance

		for i := 0; i < iterations; i++ {
			inst, err := lb.Select(ctx)
			if err != nil {
				t.Fatalf("Select failed: %v", err)
			}
			results[inst.ID]++
		}

		// Each instance should be selected approximately 1/3 of the time
		// With 3000 iterations, we expect ~1000 per instance
		// Allow 20% variance (800-1200)
		expectedPerInstance := iterations / 3
		minExpected := int(float64(expectedPerInstance) * 0.8)
		maxExpected := int(float64(expectedPerInstance) * 1.2)

		for id, count := range results {
			if count < minExpected || count > maxExpected {
				t.Errorf("Instance %s selected %d times, expected %d-%d",
					id, count, minExpected, maxExpected)
			}
		}
	})

	t.Run("select with filter", func(t *testing.T) {
		// Filter to only select instances A and B
		filter := loadbalancer.WithFilter(func(inst *loadbalancer.Instance) bool {
			return inst.ID == "A" || inst.ID == "B"
		})

		results := make(map[string]int)
		for i := 0; i < 100; i++ {
			inst, err := lb.Select(ctx, filter)
			if err != nil {
				t.Fatalf("Select with filter failed: %v", err)
			}
			results[inst.ID]++
		}

		// Should only select A and B
		if results["C"] > 0 {
			t.Error("Should not select instance C")
		}

		// Both A and B should be selected
		if results["A"] == 0 || results["B"] == 0 {
			t.Error("Should select both A and B")
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

func TestRandomBalancer_SelectClient(t *testing.T) {
	lb := New[*MockClient]()
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

func TestRandomBalancer_Remove(t *testing.T) {
	lb := New[*MockClient]()
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

	// Try to remove non-existent instance
	err = lb.Remove(ctx, "non-existent")
	if err == nil {
		t.Fatal("Expected error for non-existent instance")
	}
}

func TestRandomBalancer_Update(t *testing.T) {
	lb := New[*MockClient]()
	ctx := context.Background()

	inst := &loadbalancer.Instance{
		ID:      "test-1",
		Address: "192.168.1.1",
		Port:    8080,
		Weight:  10,
	}
	lb.Add(ctx, inst)

	// Update instance
	updated := &loadbalancer.Instance{
		ID:      "test-1",
		Address: "192.168.1.1",
		Port:    8080,
		Weight:  20,
	}
	err := lb.Update(ctx, updated)
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	// Verify update
	instances, err := lb.GetAll(ctx)
	if err != nil {
		t.Fatalf("GetAll failed: %v", err)
	}

	if len(instances) != 1 {
		t.Fatalf("Expected 1 instance, got %d", len(instances))
	}

	if instances[0].Weight != 20 {
		t.Errorf("Expected weight 20, got %d", instances[0].Weight)
	}

	// Try to update non-existent instance
	nonExistent := &loadbalancer.Instance{
		ID:      "non-existent",
		Address: "192.168.1.1",
		Port:    8080,
	}
	err = lb.Update(ctx, nonExistent)
	if err == nil {
		t.Fatal("Expected error for non-existent instance")
	}
}

func TestRandomBalancer_Close(t *testing.T) {
	lb := New[*MockClient]()
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

	err = lb.Add(ctx, &loadbalancer.Instance{ID: "test", Address: "192.168.1.1", Port: 8080})
	if err != loadbalancer.ErrBalancerClosed {
		t.Errorf("Expected ErrBalancerClosed, got %v", err)
	}

	// Close again should not error
	err = lb.Close()
	if err != nil {
		t.Fatalf("Second close failed: %v", err)
	}
}

func TestRandomBalancer_Concurrency(t *testing.T) {
	lb := New[*MockClient]()
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

	// Concurrent Select operations
	const goroutines = 100
	const operations = 100

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

func TestRandomBalancer_SingleInstance(t *testing.T) {
	lb := New[*MockClient]()
	ctx := context.Background()

	inst := &loadbalancer.Instance{
		ID:      "single",
		Address: "192.168.1.1",
		Port:    8080,
	}
	lb.Add(ctx, inst)

	// With single instance, should always select the same one
	for i := 0; i < 100; i++ {
		selected, err := lb.Select(ctx)
		if err != nil {
			t.Fatalf("Select failed: %v", err)
		}
		if selected.ID != "single" {
			t.Errorf("Expected 'single', got %s", selected.ID)
		}
	}
}
