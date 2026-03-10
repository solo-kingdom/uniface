// Package consul 提供基于 Consul 的配置存储实现示例。
//
// 基于 specs/features/rpc/governance/config/01 consul.md 实现
package consul_test

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/wii/uniface/pkg/rpc/governance/config"
	"github.com/wii/uniface/pkg/rpc/governance/config/consul"
)

// ExampleNewStorage 演示如何创建 Consul 配置存储。
func ExampleNewStorage() {
	// 使用默认配置创建存储
	storage, err := consul.NewStorage()
	if err != nil {
		log.Fatalf("创建存储失败: %v", err)
	}
	defer storage.Close()

	fmt.Println("Consul 配置存储创建成功")
}

// ExampleNewStorage_withOptions 演示如何使用自定义选项创建 Consul 配置存储。
func ExampleNewStorage_withOptions() {
	// 使用自定义选项创建存储
	storage, err := consul.NewStorage(
		consul.WithAddress("192.168.1.100:8500"),
		consul.WithKeyPrefix("myapp/config/"),
		consul.WithToken("your-acl-token"),
	)
	if err != nil {
		log.Fatalf("创建存储失败: %v", err)
	}
	defer storage.Close()

	fmt.Println("Consul 配置存储创建成功（自定义配置）")
}

// ExampleStorage_writeRead 演示如何写入和读取配置。
func ExampleStorage_writeRead() {
	storage, err := consul.NewStorage(
		consul.WithKeyPrefix("example/"),
	)
	if err != nil {
		log.Fatalf("创建存储失败: %v", err)
	}
	defer storage.Close()

	ctx := context.Background()

	// 写入配置
	err = storage.Write(ctx, "database/host", "localhost")
	if err != nil {
		log.Fatalf("写入失败: %v", err)
	}

	// 读取配置
	var host string
	err = storage.Read(ctx, "database/host", &host)
	if err != nil {
		log.Fatalf("读取失败: %v", err)
	}

	fmt.Printf("数据库主机: %s\n", host)

	// 清理
	storage.Delete(ctx, "database/host")
}

// ExampleStorage_ReadWithCache 演示带缓存的配置读取。
func ExampleStorage_ReadWithCache() {
	storage, err := consul.NewStorage(
		consul.WithKeyPrefix("example/"),
	)
	if err != nil {
		log.Fatalf("创建存储失败: %v", err)
	}
	defer storage.Close()

	ctx := context.Background()
	key := "app/timeout"

	// 写入配置
	storage.Write(ctx, key, 30)

	// 第一次读取（从 Consul 读取并缓存）
	var timeout1 int
	err = storage.ReadWithCache(ctx, key, &timeout1,
		config.WithCacheTTL(5*time.Minute),
	)
	if err != nil {
		log.Fatalf("第一次读取失败: %v", err)
	}
	fmt.Printf("超时时间（第一次）: %d 秒\n", timeout1)

	// 第二次读取（从缓存读取，更快）
	var timeout2 int
	err = storage.ReadWithCache(ctx, key, &timeout2,
		config.WithCacheTTL(5*time.Minute),
	)
	if err != nil {
		log.Fatalf("第二次读取失败: %v", err)
	}
	fmt.Printf("超时时间（缓存）: %d 秒\n", timeout2)

	// 清理
	storage.Delete(ctx, key)
}

// ExampleStorage_Watch 演示如何监听配置变更。
func ExampleStorage_Watch() {
	storage, err := consul.NewStorage(
		consul.WithKeyPrefix("example/"),
	)
	if err != nil {
		log.Fatalf("创建存储失败: %v", err)
	}
	defer storage.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	key := "app/feature-flag"

	// 定义变更处理器
	handler := func(ctx context.Context, k string, value interface{}) error {
		if value == nil {
			fmt.Printf("配置 %s 已被删除\n", k)
		} else {
			fmt.Printf("配置 %s 已更新: %v\n", k, value)
		}
		return nil
	}

	// 注册监听器
	err = storage.Watch(ctx, key, handler)
	if err != nil {
		log.Fatalf("注册监听器失败: %v", err)
	}
	defer storage.Unwatch(key)

	// 写入配置触发变更通知
	storage.Write(ctx, key, true)
	time.Sleep(100 * time.Millisecond)

	// 更新配置
	storage.Write(ctx, key, false)
	time.Sleep(100 * time.Millisecond)

	// 清理
	storage.Delete(ctx, key)
}

// ExampleStorage_WatchPrefix 演示如何监听前缀下所有配置的变更。
func ExampleStorage_WatchPrefix() {
	storage, err := consul.NewStorage(
		consul.WithKeyPrefix("example/"),
	)
	if err != nil {
		log.Fatalf("创建存储失败: %v", err)
	}
	defer storage.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	prefix := "services/"

	// 定义变更处理器
	handler := func(ctx context.Context, key string, value interface{}) error {
		fmt.Printf("服务配置 %s 变更: %v\n", key, value)
		return nil
	}

	// 监听前缀
	err = storage.WatchPrefix(ctx, prefix, handler)
	if err != nil {
		log.Fatalf("注册前缀监听器失败: %v", err)
	}
	defer storage.UnwatchPrefix(prefix)

	// 写入多个配置
	storage.Write(ctx, prefix+"service1/port", 8080)
	storage.Write(ctx, prefix+"service2/port", 9090)
	time.Sleep(100 * time.Millisecond)

	// 清理
	storage.Delete(ctx, prefix+"service1/port")
	storage.Delete(ctx, prefix+"service2/port")
}

// ExampleStorage_List 演示如何列出所有配置键。
func ExampleStorage_List() {
	storage, err := consul.NewStorage(
		consul.WithKeyPrefix("example/"),
	)
	if err != nil {
		log.Fatalf("创建存储失败: %v", err)
	}
	defer storage.Close()

	ctx := context.Background()

	// 写入多个配置
	storage.Write(ctx, "db/host", "localhost")
	storage.Write(ctx, "db/port", 5432)
	storage.Write(ctx, "db/name", "mydb")

	// 列出所有数据库配置
	keys, err := storage.List(ctx, "db/")
	if err != nil {
		log.Fatalf("列出配置失败: %v", err)
	}

	fmt.Println("数据库配置:")
	for _, key := range keys {
		fmt.Printf("  - %s\n", key)
	}

	// 清理
	storage.Delete(ctx, "db/host")
	storage.Delete(ctx, "db/port")
	storage.Delete(ctx, "db/name")
}

// ExampleStorage_complexConfig 演示如何存储复杂配置结构。
func ExampleStorage_complexConfig() {
	storage, err := consul.NewStorage(
		consul.WithKeyPrefix("example/"),
	)
	if err != nil {
		log.Fatalf("创建存储失败: %v", err)
	}
	defer storage.Close()

	ctx := context.Background()

	// 定义复杂配置结构
	type DatabaseConfig struct {
		Host     string `json:"host"`
		Port     int    `json:"port"`
		Username string `json:"username"`
		Password string `json:"password"`
		Database string `json:"database"`
	}

	// 写入复杂配置
	dbConfig := DatabaseConfig{
		Host:     "localhost",
		Port:     5432,
		Username: "user",
		Password: "pass",
		Database: "mydb",
	}

	err = storage.Write(ctx, "database/config", dbConfig)
	if err != nil {
		log.Fatalf("写入配置失败: %v", err)
	}

	// 读取复杂配置
	var loadedConfig DatabaseConfig
	err = storage.Read(ctx, "database/config", &loadedConfig)
	if err != nil {
		log.Fatalf("读取配置失败: %v", err)
	}

	fmt.Printf("数据库配置: %+v\n", loadedConfig)

	// 清理
	storage.Delete(ctx, "database/config")
}

// ExampleStorage_errorHandling 演示错误处理。
func ExampleStorage_errorHandling() {
	storage, err := consul.NewStorage(
		consul.WithKeyPrefix("example/"),
	)
	if err != nil {
		log.Fatalf("创建存储失败: %v", err)
	}
	defer storage.Close()

	ctx := context.Background()

	// 尝试读取不存在的配置
	var value string
	err = storage.Read(ctx, "nonexistent/key", &value)
	if err != nil {
		if err == config.ErrConfigNotFound {
			fmt.Println("配置不存在，使用默认值")
		} else {
			log.Printf("读取配置失败: %v", err)
		}
	}
}
