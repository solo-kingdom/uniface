# Load Balancer

通用的负载均衡器接口和实现，用于 RPC 服务治理。

## 概述

本包提供了一个类型安全的、泛型的负载均衡器接口，支持多种负载均衡算法：

- **轮询 (Round Robin)**: 依次选择实例
- **随机 (Random)**: 随机选择实例
- **加权轮询 (Weighted Round Robin)**: 根据权重分配流量
- **一致性哈希 (Consistent Hash)**: 基于 Key 的稳定路由

## 核心特性

### 1. 泛型支持

使用 Go 泛型提供类型安全的 client 管理：

```go
// gRPC client
lb := roundrobin.New[*grpc.ClientConn]()

// HTTP client
lb := roundrobin.New[*http.Client]()

// 自定义 client
lb := roundrobin.New[MyCustomClient]()
```

### 2. Client 缓存

自动创建和缓存 client，相同实例复用相同的 client：

```go
// 第一次调用 - 创建 client
client1, err := lb.SelectClient(ctx, factory)

// 第二次调用 - 复用 client
client2, err := lb.SelectClient(ctx, factory)

// client1 == client2 (同一个实例)
```

### 3. Key-based 路由

支持基于 Key 的稳定路由（一致性哈希），适用于分片场景：

```go
// 相同的 userID 总是路由到同一个实例
client, err := lb.SelectClient(ctx,
    loadbalancer.WithKey(userID),
    loadbalancer.WithClientFactory(factory),
)
```

### 4. 自动资源管理

- 自动检测 `io.Closer` 接口并调用 `Close()`
- 实例移除时自动关闭关联的 client
- 负载均衡器关闭时自动清理所有资源

### 5. 线程安全

所有实现都是线程安全的，支持并发访问。

## 快速开始

### 安装

```bash
go get github.com/solo-kingdom/uniface/pkg/rpc/governance/loadbalancer
```

### 基础使用

```go
package main

import (
    "context"
    "fmt"
    
    "google.golang.org/grpc"
    "github.com/solo-kingdom/uniface/pkg/rpc/governance/loadbalancer"
    "github.com/solo-kingdom/uniface/pkg/rpc/governance/loadbalancer/implementations/roundrobin"
)

func main() {
    // 1. 创建负载均衡器
    lb := roundrobin.New[*grpc.ClientConn]()
    
    // 2. 添加实例
    ctx := context.Background()
    lb.Add(ctx, &loadbalancer.Instance{
        ID:      "server-1",
        Address: "192.168.1.1",
        Port:    9090,
        Weight:  100,
    })
    lb.Add(ctx, &loadbalancer.Instance{
        ID:      "server-2",
        Address: "192.168.1.2",
        Port:    9090,
        Weight:  100,
    })
    
    // 3. 设置 client 工厂
    factory := loadbalancer.WithClientFactory(func(inst *loadbalancer.Instance) (*grpc.ClientConn, error) {
        addr := fmt.Sprintf("%s:%d", inst.Address, inst.Port)
        return grpc.Dial(addr, grpc.WithInsecure())
    })
    
    // 4. 选择 client
    client, err := lb.SelectClient(ctx, factory)
    if err != nil {
        panic(err)
    }
    
    // 5. 使用 client
    // client 已经是 *grpc.ClientConn 类型
    _ = client
    
    // 6. 关闭（可选）
    defer lb.Close()
}
```

## 使用示例

### 示例 1: 用户服务分片

```go
type UserService struct {
    lb loadbalancer.Balancer[*grpc.ClientConn]
}

func (s *UserService) GetUser(ctx context.Context, userID string) (*User, error) {
    // 根据 userID 路由到固定的服务器
    client, err := s.lb.SelectClient(ctx,
        loadbalancer.WithKey(userID), // Key-based 路由
        loadbalancer.WithClientFactory(s.createGRPCClient),
    )
    if err != nil {
        return nil, err
    }
    
    // 调用 RPC
    resp, err := client.GetUser(ctx, &GetUserRequest{ID: userID})
    if err != nil {
        return nil, err
    }
    
    return resp.User, nil
}
```

### 示例 2: 实例过滤

```go
// 只选择特定区域的实例
filter := loadbalancer.WithFilter(func(inst *loadbalancer.Instance) bool {
    return inst.Metadata["region"] == "us-west"
})

client, err := lb.SelectClient(ctx, 
    filter,
    loadbalancer.WithClientFactory(factory),
)
```

### 示例 3: 动态更新实例

```go
// 添加新实例
lb.Add(ctx, &loadbalancer.Instance{
    ID:      "server-3",
    Address: "192.168.1.3",
    Port:    9090,
})

// 更新实例权重
lb.Update(ctx, &loadbalancer.Instance{
    ID:      "server-1",
    Address: "192.168.1.1",
    Port:    9090,
    Weight:  200, // 增加权重
})

// 移除实例
lb.Remove(ctx, "server-2")
```

### 示例 5: 使用随机负载均衡

```go
package main

import (
    "context"
    "fmt"
    
    "google.golang.org/grpc"
    "github.com/solo-kingdom/uniface/pkg/rpc/governance/loadbalancer"
    "github.com/solo-kingdom/uniface/pkg/rpc/governance/loadbalancer/implementations/random"
)

func main() {
    // 创建随机负载均衡器
    lb := random.New[*grpc.ClientConn]()
    
    // 添加实例
    ctx := context.Background()
    for i := 1; i <= 5; i++ {
        lb.Add(ctx, &loadbalancer.Instance{
            ID:      fmt.Sprintf("server-%d", i),
            Address: fmt.Sprintf("192.168.1.%d", i),
            Port:    9090,
        })
    }
    
    // 随机选择 client
    factory := loadbalancer.WithClientFactory(func(inst *loadbalancer.Instance) (*grpc.ClientConn, error) {
        addr := fmt.Sprintf("%s:%d", inst.Address, inst.Port)
        return grpc.Dial(addr, grpc.WithInsecure())
    })
    
    client, err := lb.SelectClient(ctx, factory)
    if err != nil {
        panic(err)
    }
    
    _ = client
}
```

### 示例 6: 确定性测试（使用种子）

```go
// 在测试中使用固定的种子，确保可重现
lb := random.NewWithSeed[*MockClient](12345)

// 相同的种子会产生相同的随机序列
// 这对测试非常有用
```

### 示例 7: 加权轮询（异构集群）

```go
package main

import (
    "context"
    "fmt"
    
    "google.golang.org/grpc"
    "github.com/solo-kingdom/uniface/pkg/rpc/governance/loadbalancer"
    "github.com/solo-kingdom/uniface/pkg/rpc/governance/loadbalancer/implementations/weighted"
)

func main() {
    // 创建加权轮询负载均衡器
    lb := weighted.New[*grpc.ClientConn]()
    
    ctx := context.Background()
    
    // 添加高性能服务器（权重 100）
    lb.Add(ctx, &loadbalancer.Instance{
        ID:      "high-perf-1",
        Address: "192.168.1.1",
        Port:    9090,
        Weight:  100, // 高权重，获得更多流量
    })
    
    // 添加低性能服务器（权重 30）
    lb.Add(ctx, &loadbalancer.Instance{
        ID:      "low-perf-1",
        Address: "192.168.1.2",
        Port:    9090,
        Weight:  30, // 低权重，获得较少流量
    })
    
    // 流量分配：约 77% 到 high-perf-1，23% 到 low-perf-1
}
```

### 示例 8: 一致性哈希（分片场景）

```go
package main

import (
    "context"
    "fmt"
    
    "google.golang.org/grpc"
    "github.com/solo-kingdom/uniface/pkg/rpc/governance/loadbalancer"
    "github.com/solo-kingdom/uniface/pkg/rpc/governance/loadbalancer/implementations/consistenthash"
)

type UserService struct {
    lb loadbalancer.Balancer[*grpc.ClientConn]
}

func main() {
    // 创建一致性哈希负载均衡器（100 个虚拟节点）
    lb := consistenthash.New[*grpc.ClientConn](100, nil)
    
    ctx := context.Background()
    
    // 添加分片实例
    for i := 1; i <= 5; i++ {
        lb.Add(ctx, &loadbalancer.Instance{
            ID:      fmt.Sprintf("shard-%d", i),
            Address: fmt.Sprintf("192.168.1.%d", i),
            Port:    9090,
        })
    }
    
    // Key-based 路由：相同 userID 总是路由到同一分片
    factory := loadbalancer.WithClientFactory(func(inst *loadbalancer.Instance) (*grpc.ClientConn, error) {
        addr := fmt.Sprintf("%s:%d", inst.Address, inst.Port)
        return grpc.Dial(addr, grpc.WithInsecure())
    })
    
    // 用户 A 的请求总是路由到同一分片
    clientA1, _ := lb.SelectClient(ctx, 
        loadbalancer.WithKey("user-A"),
        factory,
    )
    clientA2, _ := lb.SelectClient(ctx, 
        loadbalancer.WithKey("user-A"),
        factory,
    )
    // clientA1 == clientA2（同一分片）
    
    // 用户 B 的请求总是路由到同一分片
    clientB, _ := lb.SelectClient(ctx,
        loadbalancer.WithKey("user-B"),
        factory,
    )
    // clientB 可能与 clientA 不同（不同分片）
    
    _ = clientA1
    _ = clientA2
    _ = clientB
}
```

## 接口说明

### Balancer[T]

```go
type Balancer[T any] interface {
    // 实例选择
    Select(ctx context.Context, opts ...Option) (*Instance, error)
    SelectClient(ctx context.Context, opts ...Option) (T, error)
    
    // 实例管理
    Add(ctx context.Context, instance *Instance) error
    Remove(ctx context.Context, instanceID string) error
    Update(ctx context.Context, instance *Instance) error
    GetAll(ctx context.Context) ([]*Instance, error)
    
    // 生命周期
    Close() error
}
```

### Instance

```go
type Instance struct {
    ID       string            // 唯一标识（必须）
    Address  string            // IP 地址（必须）
    Port     int               // 端口号（必须）
    Weight   int               // 权重（默认 1）
    Metadata map[string]string // 元数据（可选）
}
```

### Options

```go
// 设置 Key（用于一致性哈希）
func WithKey(key string) Option

// 设置 client 工厂
func WithClientFactory[T any](factory func(*Instance) (T, error)) Option

// 设置实例过滤器
func WithFilter(filter func(*Instance) bool) Option
```

## 实现策略

### Round Robin (轮询)

依次选择实例，确保公平分配：

```
Request 1 -> Instance A
Request 2 -> Instance B
Request 3 -> Instance C
Request 4 -> Instance A
...
```

**适用场景**:
- 所有实例性能相近
- 需要均匀分配负载

### Random (随机)

随机选择实例，长期来看负载分布均匀：

```go
lb := random.New[*grpc.ClientConn]()

// 随机选择实例
client, err := lb.SelectClient(ctx, factory)
```

**特点**:
- 无状态，不需要维护计数器
- 简单高效，性能好
- 长期来看负载分布均匀
- 支持确定性测试（通过 `NewWithSeed`）

**适用场景**:
- 无状态服务
- 需要简单的负载均衡策略
- 测试环境（可重现随机序列）

### Weighted Round Robin (加权轮询)

根据权重分配流量，高性能服务器获得更多请求：

```go
lb := weighted.New[*grpc.ClientConn]()

// 高性能服务器 - 权重 100
lb.Add(ctx, &loadbalancer.Instance{
    ID:      "high-perf-1",
    Address: "192.168.1.1",
    Port:    9090,
    Weight:  100, // 高权重
})

// 低性能服务器 - 权重 30
lb.Add(ctx, &loadbalancer.Instance{
    ID:      "low-perf-1",
    Address: "192.168.1.2",
    Port:    9090,
    Weight:  30, // 低权重
})

// 流量分配：约 77% 到 high-perf-1，23% 到 low-perf-1
```

**特点**:
- 平滑加权轮询算法，避免流量突发
- 根据服务器能力动态分配负载
- 未指定权重或权重为 0 时，默认为 1

**适用场景**:
- 异构服务器集群
- 需要根据服务器能力分配流量

### Consistent Hash (一致性哈希)

基于 Key 的稳定路由，相同的 Key 总是路由到同一个实例：

```go
lb := consistenthash.New[*grpc.ClientConn](100, nil)

// 添加实例
lb.Add(ctx, &loadbalancer.Instance{
    ID:      "shard-1",
    Address: "192.168.1.1",
    Port:    9090,
})

// Key-based 路由（相同 userID 总是路由到同一实例）
client, err := lb.SelectClient(ctx,
    loadbalancer.WithKey(userID),
    loadbalancer.WithClientFactory(factory),
)

// 无 Key 时使用轮询
client, err := lb.SelectClient(ctx,
    loadbalancer.WithClientFactory(factory),
)
```

**特点**:
- 使用一致性哈希算法，确保相同 Key 路由到相同实例
- 支持虚拟节点，提高分布均匀性
- 实例增减时最小化影响范围
- 无 Key 时自动回退到轮询

**适用场景**:
- 有状态服务
- 数据分片
- 缓存亲和性
- 会话粘性

## 错误处理

```go
var (
    ErrNoInstances        // 没有可用实例
    ErrInstanceNotFound   // 实例不存在
    ErrInvalidInstance    // 无效实例
    ErrBalancerClosed     // 负载均衡器已关闭
    ErrDuplicateInstance  // 实例已存在
    ErrNoClientFactory    // 未提供 client 工厂
    ErrClientCreateFailed // client 创建失败
)
```

## 最佳实践

### 1. Client 工厂设计

```go
// ✅ 好的做法：工厂函数只负责创建 client
factory := loadbalancer.WithClientFactory(func(inst *loadbalancer.Instance) (*grpc.ClientConn, error) {
    addr := fmt.Sprintf("%s:%d", inst.Address, inst.Port)
    return grpc.Dial(addr,
        grpc.WithInsecure(),
        grpc.WithTimeout(5*time.Second),
    )
})

// ❌ 不好的做法：在工厂中执行业务逻辑
factory := loadbalancer.WithClientFactory(func(inst *loadbalancer.Instance) (*grpc.ClientConn, error) {
    // 不要在这里执行 RPC 调用
    conn, _ := grpc.Dial(...)
    client := pb.NewMyServiceClient(conn)
    client.HealthCheck(ctx, ...) // ❌ 错误
    return conn, nil
})
```

### 2. 资源清理

```go
// ✅ 确保调用 Close()
lb := roundrobin.New[*grpc.ClientConn]()
defer lb.Close()

// ✅ 或者让 client 实现 io.Closer
type MyClient struct {
    conn *grpc.ClientConn
}

func (c *MyClient) Close() error {
    return c.conn.Close()
}
```

### 3. 错误处理

```go
client, err := lb.SelectClient(ctx, factory)
if err != nil {
    switch {
    case errors.Is(err, loadbalancer.ErrNoInstances):
        // 没有可用实例，可能需要等待或降级
        return handleNoInstances()
    
    case errors.Is(err, loadbalancer.ErrBalancerClosed):
        // 负载均衡器已关闭
        return err
    
    default:
        // 其他错误
        return err
    }
}
```

### 4. 健康检查

```go
// 使用过滤器实现健康检查
filter := loadbalancer.WithFilter(func(inst *loadbalancer.Instance) bool {
    // 检查实例是否健康
    return inst.Metadata["status"] == "healthy"
})

client, err := lb.SelectClient(ctx, filter, factory)
```

## 性能考虑

1. **Client 缓存**: 避免重复创建 client，提高性能
2. **读多写少**: 使用读写锁优化读性能
3. **无锁计数**: Round-robin 使用原子计数器
4. **零分配**: Select 操作在热路径上尽量减少内存分配

## 扩展性

### 自定义负载均衡算法

```go
type MyBalancer[T any] struct {
    *base.BaseBalancer[T]
    // 自定义字段
}

func (b *MyBalancer[T]) Select(ctx context.Context, opts ...loadbalancer.Option) (*loadbalancer.Instance, error) {
    // 实现自定义选择逻辑
}
```

### 集成监控

```go
// 可以通过装饰器模式添加监控
type MonitoredBalancer[T any] struct {
    loadbalancer.Balancer[T]
    metrics *Metrics
}

func (m *MonitoredBalancer[T]) SelectClient(ctx context.Context, opts ...loadbalancer.Option) (T, error) {
    start := time.Now()
    client, err := m.Balancer.SelectClient(ctx, opts...)
    
    m.metrics.RecordSelect(time.Since(start), err)
    
    return client, err
}
```

## 相关文档

- [实施计划](../../../../docs/load-balancer-implementation-plan.md)
- [AI 代码生成规则](../../../../docs/AI_CODING_RULES.md)

## License

MIT
