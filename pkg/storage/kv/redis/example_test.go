package redis_test

import (
	"context"
	"fmt"
	"log"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"github.com/wii/uniface/pkg/storage/kv"
	"github.com/wii/uniface/pkg/storage/kv/redis"
)

// const redisAddress = "127.0.0.1:6379"
const redisAddress = "192.168.6.12:6379"

// ExampleNew demonstrates creating a new Redis storage instance with default settings.
func ExampleNew() {
	// Create a new Redis storage with default configuration (localhost:6379, DB 0).
	storage, err := redis.New()
	if err != nil {
		log.Fatal(err)
	}
	defer storage.Close()

	fmt.Println("Connected to Redis successfully")
	// Output:
	// Connected to Redis successfully
}

// ExampleNew demonstrates creating a new Redis storage instance with default settings.
func ExampleNewClient() {
	// Create a new Redis storage with default configuration (localhost:6379, DB 0).
	storage, err := redis.New(redis.WithAddr(redisAddress))
	if err != nil {
		log.Fatal(err)
	}
	defer storage.Close()

	fmt.Println("Connected to Redis successfully")
	// Output:
	// Connected to Redis successfully
}

// ExampleNew_withOptions demonstrates creating a Redis storage with custom options.
func ExampleNew_withOptions() {
	storage, err := redis.New(
		redis.WithAddr(redisAddress),
		redis.WithPassword(""),
		redis.WithDB(0),
		redis.WithPoolSize(20),
		redis.WithMinIdleConns(5),
		redis.WithMaxRetries(3),
		redis.WithDialTimeout(5*time.Second),
		redis.WithReadTimeout(3*time.Second),
		redis.WithWriteTimeout(3*time.Second),
		redis.WithKeyPrefix("myapp:"),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer storage.Close()

	fmt.Println("Connected with custom options")
	// Output:
	// Connected with custom options
}

// ExampleNewWithClient demonstrates creating a Redis storage with an existing redis client.
// This is useful when you want to share a Redis connection across multiple storage instances.
func ExampleNewWithClient() {
	// Create a shared Redis client
	client := goredis.NewClient(&goredis.Options{
		Addr: redisAddress,
	})

	// Create multiple storage instances sharing the same connection
	userStorage, err := redis.NewWithClient(client, redis.WithKeyPrefix("users:"))
	if err != nil {
		log.Fatal(err)
	}
	defer userStorage.Close()

	orderStorage, err := redis.NewWithClient(client, redis.WithKeyPrefix("orders:"))
	if err != nil {
		log.Fatal(err)
	}
	defer orderStorage.Close()

	fmt.Println("Created multiple storages with shared client")
	// Output:
	// Created multiple storages with shared client
}

// ExampleStorage_Set demonstrates storing a simple string value.
func ExampleStorage_Set() {
	storage, err := redis.New(redis.WithAddr(redisAddress), redis.WithKeyPrefix("example:"))
	if err != nil {
		log.Fatal(err)
	}
	defer storage.Close()

	ctx := context.Background()

	// Store a simple string value
	err = storage.Set(ctx, "greeting", "Hello, World!")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Value stored successfully")
	// Output:
	// Value stored successfully
}

// ExampleStorage_Set_withTTL demonstrates storing a value with an expiration time.
func ExampleStorage_Set_withTTL() {
	storage, err := redis.New(redis.WithKeyPrefix("example:"))
	if err != nil {
		log.Fatal(err)
	}
	defer storage.Close()

	ctx := context.Background()

	// Store a value that expires after 1 hour
	err = storage.Set(ctx, "session:abc123", "user_data", kv.WithTTL(1*time.Hour))
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Value stored with TTL")
	// Output:
	// Value stored with TTL
}

// ExampleStorage_Set_withNoOverwrite demonstrates storing a value only if the key does not already exist.
func ExampleStorage_Set_withNoOverwrite() {
	storage, err := redis.New(redis.WithKeyPrefix("example:"))
	if err != nil {
		log.Fatal(err)
	}
	defer storage.Close()

	ctx := context.Background()

	// Clean up first
	_ = storage.Delete(ctx, "unique_key")

	// First set succeeds
	err = storage.Set(ctx, "unique_key", "first_value", kv.WithNoOverwrite())
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("First set: success")

	// Second set fails because key already exists
	err = storage.Set(ctx, "unique_key", "second_value", kv.WithNoOverwrite())
	if err != nil {
		fmt.Println("Second set: key already exists")
	}

	// Clean up
	_ = storage.Delete(ctx, "unique_key")
	// Output:
	// First set: success
	// Second set: key already exists
}

// ExampleStorage_Set_withNamespace demonstrates storing values with namespace isolation.
func ExampleStorage_Set_withNamespace() {
	storage, err := redis.New(redis.WithKeyPrefix("example:"))
	if err != nil {
		log.Fatal(err)
	}
	defer storage.Close()

	ctx := context.Background()

	// Store user config in "config" namespace
	// The actual Redis key will be: "example:config:theme"
	err = storage.Set(ctx, "theme", "dark", kv.WithNamespace("config"))
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Value stored with namespace")
	// Output:
	// Value stored with namespace
}

// ExampleStorage_Get demonstrates retrieving a stored value.
func ExampleStorage_Get() {
	storage, err := redis.New(redis.WithKeyPrefix("example:"))
	if err != nil {
		log.Fatal(err)
	}
	defer storage.Close()

	ctx := context.Background()

	// Store a value first
	_ = storage.Set(ctx, "name", "Alice")

	// Retrieve the value
	var name string
	err = storage.Get(ctx, "name", &name)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Name:", name)

	// Clean up
	_ = storage.Delete(ctx, "name")
	// Output:
	// Name: Alice
}

// ExampleStorage_Get_struct demonstrates storing and retrieving a complex struct.
func ExampleStorage_Get_struct() {
	storage, err := redis.New(redis.WithKeyPrefix("example:"))
	if err != nil {
		log.Fatal(err)
	}
	defer storage.Close()

	ctx := context.Background()

	// Define a struct
	type User struct {
		Name  string `json:"name"`
		Email string `json:"email"`
		Age   int    `json:"age"`
	}

	// Store the struct
	user := User{Name: "Alice", Email: "alice@example.com", Age: 30}
	_ = storage.Set(ctx, "user:1", user)

	// Retrieve the struct
	var retrieved User
	err = storage.Get(ctx, "user:1", &retrieved)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("User: %s, Email: %s, Age: %d\n", retrieved.Name, retrieved.Email, retrieved.Age)

	// Clean up
	_ = storage.Delete(ctx, "user:1")
	// Output:
	// User: Alice, Email: alice@example.com, Age: 30
}

// ExampleStorage_Delete demonstrates deleting a key-value pair.
func ExampleStorage_Delete() {
	storage, err := redis.New(redis.WithKeyPrefix("example:"))
	if err != nil {
		log.Fatal(err)
	}
	defer storage.Close()

	ctx := context.Background()

	// Store and then delete
	_ = storage.Set(ctx, "temp_key", "temp_value")

	err = storage.Delete(ctx, "temp_key")
	if err != nil {
		log.Fatal(err)
	}

	// Verify it's gone
	exists, _ := storage.Exists(ctx, "temp_key")
	fmt.Println("Key exists after delete:", exists)
	// Output:
	// Key exists after delete: false
}

// ExampleStorage_Exists demonstrates checking if a key exists.
func ExampleStorage_Exists() {
	storage, err := redis.New(redis.WithKeyPrefix("example:"))
	if err != nil {
		log.Fatal(err)
	}
	defer storage.Close()

	ctx := context.Background()

	// Check a non-existent key
	exists, err := storage.Exists(ctx, "nonexistent_key_example_12345")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Nonexistent key exists:", exists)

	// Store a key and check again
	_ = storage.Set(ctx, "real_key", "real_value")

	exists, err = storage.Exists(ctx, "real_key")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Real key exists:", exists)

	// Clean up
	_ = storage.Delete(ctx, "real_key")
	// Output:
	// Nonexistent key exists: false
	// Real key exists: true
}

// ExampleStorage_BatchSet demonstrates storing multiple key-value pairs atomically.
func ExampleStorage_BatchSet() {
	storage, err := redis.New(redis.WithKeyPrefix("example:"))
	if err != nil {
		log.Fatal(err)
	}
	defer storage.Close()

	ctx := context.Background()

	// Store multiple key-value pairs at once
	items := map[string]interface{}{
		"lang:go":     "Go",
		"lang:python": "Python",
		"lang:rust":   "Rust",
	}

	err = storage.BatchSet(ctx, items)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Batch set completed")

	// Clean up
	_ = storage.BatchDelete(ctx, []string{"lang:go", "lang:python", "lang:rust"})
	// Output:
	// Batch set completed
}

// ExampleStorage_BatchGet demonstrates retrieving multiple values at once.
func ExampleStorage_BatchGet() {
	storage, err := redis.New(redis.WithKeyPrefix("example:"))
	if err != nil {
		log.Fatal(err)
	}
	defer storage.Close()

	ctx := context.Background()

	// Store some data first
	items := map[string]interface{}{
		"fruit:apple":  "red",
		"fruit:banana": "yellow",
		"fruit:grape":  "purple",
	}
	_ = storage.BatchSet(ctx, items)

	// Retrieve multiple keys at once
	keys := []string{"fruit:apple", "fruit:banana", "fruit:grape"}
	results, err := storage.BatchGet(ctx, keys)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Apple:", results["fruit:apple"])
	fmt.Println("Banana:", results["fruit:banana"])
	fmt.Println("Grape:", results["fruit:grape"])

	// Clean up
	_ = storage.BatchDelete(ctx, keys)
	// Output:
	// Apple: red
	// Banana: yellow
	// Grape: purple
}

// ExampleStorage_BatchDelete demonstrates deleting multiple keys at once.
func ExampleStorage_BatchDelete() {
	storage, err := redis.New(redis.WithKeyPrefix("example:"))
	if err != nil {
		log.Fatal(err)
	}
	defer storage.Close()

	ctx := context.Background()

	// Store some data
	items := map[string]interface{}{
		"del:key1": "value1",
		"del:key2": "value2",
		"del:key3": "value3",
	}
	_ = storage.BatchSet(ctx, items)

	// Delete all keys at once
	keys := []string{"del:key1", "del:key2", "del:key3"}
	err = storage.BatchDelete(ctx, keys)
	if err != nil {
		log.Fatal(err)
	}

	// Verify they are all gone
	results, _ := storage.BatchGet(ctx, keys)
	fmt.Println("Remaining keys:", len(results))
	// Output:
	// Remaining keys: 0
}

// ExampleStorage_Close demonstrates proper cleanup of the storage instance.
func ExampleStorage_Close() {
	storage, err := redis.New()
	if err != nil {
		log.Fatal(err)
	}

	// Use the storage...
	ctx := context.Background()
	_ = storage.Set(ctx, "key", "value")

	// Close when done - releases the Redis connection
	err = storage.Close()
	if err != nil {
		log.Fatal(err)
	}

	// After closing, operations will return errors
	err = storage.Set(ctx, "key", "value")
	if err != nil {
		fmt.Println("Error after close:", err)
	}
	// Output:
	// Error after close: kv set "key": storage closed
}

// ExampleStorage_Client demonstrates accessing the underlying Redis client
// for advanced operations not covered by the storage interface.
func ExampleStorage_Client() {
	storage, err := redis.New(redis.WithKeyPrefix("example:"))
	if err != nil {
		log.Fatal(err)
	}
	defer storage.Close()

	// Access the underlying Redis client for advanced operations
	client := storage.Client()

	ctx := context.Background()

	// Use native Redis commands directly
	err = client.Set(ctx, "raw:counter", 0, 0).Err()
	if err != nil {
		log.Fatal(err)
	}

	// Increment a counter using Redis INCR
	newVal, err := client.Incr(ctx, "raw:counter").Result()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Counter after INCR:", newVal)

	// Clean up
	_ = client.Del(ctx, "raw:counter")
	// Output:
	// Counter after INCR: 1
}
