// Package shard_test provides examples for using the shard manager.
//
// 基于 prompts/features/rpc/governance/load-balancer/shard/00-shard-manager.md 实现
package shard_test

import (
	"fmt"

	"github.com/solo-kingdom/uniface/pkg/rpc/governance/loadbalancer"
	"github.com/solo-kingdom/uniface/pkg/rpc/governance/loadbalancer/shard"
)

// Example_basicUsage demonstrates basic shard manager usage.
func Example_basicUsage() {
	// 1. Create shard manager with instances (fixed at creation time)
	manager := shard.NewShardManager([]*loadbalancer.Instance{
		{ID: "db-0", Address: "192.168.1.1", Port: 3306},
		{ID: "db-1", Address: "192.168.1.2", Port: 3306},
		{ID: "db-2", Address: "192.168.1.3", Port: 3306},
	})
	defer manager.Close()

	// 2. Route based on key (stable routing - same key always to same instance)
	instance, err := manager.Select("user-123")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Routed to: %s at %s:%d\n",
		instance.ID, instance.Address, instance.Port)
}

// Example_selectClient demonstrates client selection with factory.
func Example_selectClient() {
	// Create shard manager
	manager := shard.NewShardManager([]*loadbalancer.Instance{
		{ID: "inst-0", Address: "192.168.1.1", Port: 8080},
		{ID: "inst-1", Address: "192.168.1.2", Port: 8080},
	})
	defer manager.Close()

	// Define client factory
	factory := func(inst *loadbalancer.Instance) (interface{}, error) {
		// In real code, create actual client (gRPC, HTTP, database connection, etc.)
		return fmt.Sprintf("client-%s", inst.ID), nil
	}

	// Select client for a key
	client, err := manager.SelectClient("user-123", factory)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Got client: %v\n", client)
}

// Example_stability demonstrates routing stability.
func Example_stability() {
	manager := shard.NewShardManager([]*loadbalancer.Instance{
		{ID: "node-0", Address: "192.168.1.1", Port: 8080},
		{ID: "node-1", Address: "192.168.1.2", Port: 8080},
	})
	defer manager.Close()

	// Same key always routes to the same instance
	inst1, _ := manager.Select("user-123")
	inst2, _ := manager.Select("user-123")
	inst3, _ := manager.Select("user-123")

	fmt.Printf("Selection 1: %s\n", inst1.ID)
	fmt.Printf("Selection 2: %s\n", inst2.ID)
	fmt.Printf("Selection 3: %s\n", inst3.ID)

	if inst1.ID == inst2.ID && inst2.ID == inst3.ID {
		fmt.Println("✓ Stable routing confirmed!")
	}
}

// Example_databaseSharding demonstrates database sharding pattern.
func Example_databaseSharding() {
	// Create shard manager for database sharding
	// Each instance represents a database shard
	manager := shard.NewShardManager([]*loadbalancer.Instance{
		{ID: "db-shard-0", Address: "db0.example.com", Port: 5432},
		{ID: "db-shard-1", Address: "db1.example.com", Port: 5432},
		{ID: "db-shard-2", Address: "db2.example.com", Port: 5432},
	})
	defer manager.Close()

	// Route database queries based on user ID
	userID := "user-12345"
	instance, err := manager.Select(userID)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("User %s routed to database: %s\n", userID, instance.ID)
}
