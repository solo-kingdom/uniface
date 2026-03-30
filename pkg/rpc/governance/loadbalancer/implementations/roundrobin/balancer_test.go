// 基于 prompts/features/service/governance/load-balancer/01-load-balancer-iface.md 实现
package roundrobin

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

func TestRoundRobinBalancer_Add(t *testing.T) {
	lb := New[*MockClient]()
	ctx := context.Background()

	t.Run("add valid instance", func(t *testing.T) {
		inst := &loadbalancer.Instance{
			ID:      "test-1",
			Address: "192.168.1.1",
			Port:    8080,
		}

		err := lb.Add(ctx, inst)
		if err != nil {
			t.Fatalf("Add failed: %v", err)
		}
	})

	t.Run("add duplicate instance", func(t *testing.T) {
		inst := &loadbalancer.Instance{
			ID:      "test-1",
			Address: "192.168.1.1",
			Port:    8080,
		}

		err := lb.Add(ctx, inst)
		if err == nil {
			t.Fatal("Expected error for duplicate instance")
		}
	})

	t.Run("add nil instance", func(t *testing.T) {
		err := lb.Add(ctx, nil)
		if err == nil {
			t.Fatal("Expected error for nil instance")
		}
	})

	t.Run("add instance with empty ID", func(t *testing.T) {
		inst := &loadbalancer.Instance{
			Address: "192.168.1.1",
			Port:    8080,
		}

		err := lb.Add(ctx, inst)
		if err == nil {
			t.Fatal("Expected error for empty ID")
		}
	})

	t.Run("add instance with empty address", func(t *testing.T) {
		inst := &loadbalancer.Instance{
			ID:   "test-2",
			Port: 8080,
		}

		err := lb.Add(ctx, inst)
		if err == nil {
			t.Fatal("Expected error for empty address")
		}
	})

	t.Run("add instance with invalid port", func(t *testing.T) {
		inst := &loadbalancer.Instance{
			ID:      "test-3",
			Address: "192.168.1.1",
			Port:    -1,
		}

		err := lb.Add(ctx, inst)
		if err == nil {
			t.Fatal("Expected error for invalid port")
		}
	})
}

func TestRoundRobinBalancer_Select(t *testing.T) {
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

	t.Run("round-robin distribution", func(t *testing.T) {
		// Select 12 times (should cycle through 3 instances 4 times each)
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

		for i := 0; i < 5; i++ {
			inst, err := lb.Select(ctx, filter)
			if err != nil {
				t.Fatalf("Select with filter failed: %v", err)
			}
			if inst.ID != "A" {
				t.Errorf("Expected instance A, got %s", inst.ID)
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
}

func TestRoundRobinBalancer_SelectClient(t *testing.T) {
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

func TestRoundRobinBalancer_Remove(t *testing.T) {
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

func TestRoundRobinBalancer_Update(t *testing.T) {
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

func TestRoundRobinBalancer_Close(t *testing.T) {
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

func TestRoundRobinBalancer_Concurrency(t *testing.T) {
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
