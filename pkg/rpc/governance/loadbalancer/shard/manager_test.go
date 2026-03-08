// Package shard provides simple sharding management based on load balancers.
// This file contains tests for the ShardManager implementation.
//
// 基于 prompts/features/rpc/governance/load-balancer/shard/00-shard-manager.md 实现
package shard

import (
	"testing"

	"github.com/wii/uniface/pkg/rpc/governance/loadbalancer"
)

func TestShardManager_Select(t *testing.T) {
	instances := []*loadbalancer.Instance{
		{ID: "inst-0", Address: "192.168.1.1", Port: 8080},
		{ID: "inst-1", Address: "192.168.1.2", Port: 8080},
		{ID: "inst-2", Address: "192.168.1.3", Port: 8080},
	}

	manager := NewShardManager(instances)
	defer manager.Close()

	t.Run("stable routing - same key always routes to same instance", func(t *testing.T) {
		key := "user-123"

		// Select multiple times with the same key
		inst1, err := manager.Select(key)
		if err != nil {
			t.Fatalf("Select failed: %v", err)
		}

		inst2, err := manager.Select(key)
		if err != nil {
			t.Fatalf("Select failed: %v", err)
		}

		inst3, err := manager.Select(key)
		if err != nil {
			t.Fatalf("Select failed: %v", err)
		}

		// All selections should return the same instance
		if inst1.ID != inst2.ID || inst2.ID != inst3.ID {
			t.Errorf("Same key routed to different instances: %s, %s, %s",
				inst1.ID, inst2.ID, inst3.ID)
		}
	})

	t.Run("different keys distribute across instances", func(t *testing.T) {
		distribution := make(map[string]int)

		// Use different keys
		for i := 0; i < 100; i++ {
			key := string(rune('a' + i%26))
			inst, err := manager.Select(key)
			if err != nil {
				t.Fatalf("Select failed for key %s: %v", key, err)
			}
			distribution[inst.ID]++
		}

		// All instances should receive some traffic
		for _, inst := range instances {
			if distribution[inst.ID] == 0 {
				t.Errorf("Instance %s received no traffic", inst.ID)
			}
		}
	})

	t.Run("empty key returns error", func(t *testing.T) {
		_, err := manager.Select("")
		if err != ErrInvalidKey {
			t.Errorf("Expected ErrInvalidKey, got %v", err)
		}
	})
}

func TestShardManager_SelectClient(t *testing.T) {
	instances := []*loadbalancer.Instance{
		{ID: "inst-0", Address: "192.168.1.1", Port: 8080},
		{ID: "inst-1", Address: "192.168.1.2", Port: 8080},
	}

	manager := NewShardManager(instances)
	defer manager.Close()

	t.Run("create and cache client", func(t *testing.T) {
		factory := func(inst *loadbalancer.Instance) (interface{}, error) {
			return "client-" + inst.ID, nil
		}

		key := "user-123"

		// First call creates the client
		client1, err := manager.SelectClient(key, factory)
		if err != nil {
			t.Fatalf("SelectClient failed: %v", err)
		}

		// Second call returns the cached client
		client2, err := manager.SelectClient(key, factory)
		if err != nil {
			t.Fatalf("SelectClient failed: %v", err)
		}

		// Should be the same client
		if client1 != client2 {
			t.Error("Expected same client to be returned")
		}
	})

	t.Run("empty key returns error", func(t *testing.T) {
		factory := func(inst *loadbalancer.Instance) (interface{}, error) {
			return "client", nil
		}

		_, err := manager.SelectClient("", factory)
		if err != ErrInvalidKey {
			t.Errorf("Expected ErrInvalidKey, got %v", err)
		}
	})

	t.Run("nil factory returns error", func(t *testing.T) {
		_, err := manager.SelectClient("user-123", nil)
		if err != ErrNoFactory {
			t.Errorf("Expected ErrNoFactory, got %v", err)
		}
	})
}

func TestShardManager_Close(t *testing.T) {
	instances := []*loadbalancer.Instance{
		{ID: "inst-0", Address: "192.168.1.1", Port: 8080},
	}

	manager := NewShardManager(instances)

	err := manager.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Operations after close should fail
	_, err = manager.Select("user-123")
	if err == nil {
		t.Error("Expected error after close, got nil")
	}
}

func TestShardManager_EmptyInstances(t *testing.T) {
	// Create manager with no instances
	manager := NewShardManager([]*loadbalancer.Instance{})
	defer manager.Close()

	// Should return error when selecting
	_, err := manager.Select("user-123")
	if err == nil {
		t.Error("Expected error with no instances, got nil")
	}
}
