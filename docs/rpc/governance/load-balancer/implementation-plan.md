# 负载均衡器实施计划

> 本文档记录负载均衡器的完整设计和实施计划

## 一、需求来源

基于 `prompts/features/service/governance/load-balancer/01-load-balancer-iface.md` 实现。

## 二、核心设计决策

| 决策点 | 选择 | 说明 |
|--------|------|------|
| **路径** | `pkg/rpc/governance/loadbalancer/` | RPC 服务治理 |
| **Client 类型** | 泛型 `T` | 类型安全 |
| **Client 关闭** | 自动检测 `io.Closer` | 智能且灵活 |
| **Key 路由** | 有 Key → 一致性哈希，无 Key → 默认 | 符合直觉 |
| **空 Key** | 等同于未指定 | 简化逻辑 |
| **Instance 状态** | 不包含 | 简化设计 |
| **实施策略** | 渐进式 | 降低风险 |

### 关键特性

1. **路径调整**: `service` → `rpc`
2. **SelectClient 接口**: 通过传入可选的创建 client 方法，生成并管理 client
3. **Key-based 选择**: 支持指定 key，保证分片稳定性（相同 key 访问相同的 client）

## 三、目录结构

```
pkg/rpc/governance/loadbalancer/
├── interface.go              # 核心接口和 Instance 定义
├── options.go                # 选项模式
├── errors.go                 # 错误定义
├── README.md                 # 包文档
│
├── base/                     # 基础实现（共享逻辑）
│   └── balancer.go          # 通用逻辑（client 缓存、实例管理）
│
└── implementations/          # 具体算法实现
    ├── roundrobin/           # 轮询算法
    │   ├── balancer.go
    │   └── balancer_test.go
    ├── weighted/             # 加权轮询算法
    │   ├── balancer.go
    │   └── balancer_test.go
    ├── random/               # 随机算法
    │   ├── balancer.go
    │   └── balancer_test.go
    └── consistenthash/       # 一致性哈希算法
        ├── balancer.go
        ├── balancer_test.go
        └── ring.go           # 哈希环实现
```

## 四、接口设计

### 1. Instance 结构体

```go
type Instance struct {
    ID       string            // 实例唯一标识（必须）
    Address  string            // IP 地址（必须）
    Port     int               // 端口号（必须）
    Weight   int               // 权重，默认为 1
    Metadata map[string]string // 元数据（可选）
}
```

### 2. Balancer[T] 接口

```go
type Balancer[T any] interface {
    // ========== 实例选择 ==========
    
    // Select 从可用实例中选择一个实例
    // 如果 opts 中指定了 Key，使用一致性哈希算法
    // 如果没有 Key 或 Key 为空，使用默认策略
    Select(ctx context.Context, opts ...Option) (*Instance, error)
    
    // SelectClient 选择并返回一个 client
    // 如果 client 已缓存，直接返回
    // 如果 client 未缓存，调用 ClientFactory 创建并缓存
    // 相同的实例总是返回相同的 client
    SelectClient(ctx context.Context, opts ...Option) (T, error)
    
    // ========== 实例管理 ==========
    
    // Add 添加一个实例到负载均衡器
    Add(ctx context.Context, instance *Instance) error
    
    // Remove 从负载均衡器中移除实例
    // 同时会关闭并移除关联的 client（如果存在）
    Remove(ctx context.Context, instanceID string) error
    
    // Update 更新实例信息
    Update(ctx context.Context, instance *Instance) error
    
    // GetAll 获取所有实例的副本
    GetAll(ctx context.Context) ([]*Instance, error)
    
    // ========== 生命周期 ==========
    
    // Close 关闭负载均衡器并释放资源
    // 会关闭所有缓存的 client
    Close() error
}
```

### 3. Options 设计

```go
type Options struct {
    // Key 用于一致性哈希选择（可选）
    Key string
    
    // ClientFactory 用于创建 client 的工厂函数（可选）
    ClientFactory func(*Instance) (interface{}, error)
    
    // Filter 实例过滤器（可选）
    Filter func(*Instance) bool
}

// 选项函数
func WithKey(key string) Option
func WithClientFactory[T any](factory func(*Instance) (T, error)) Option
func WithFilter(filter func(*Instance) bool) Option
```

### 4. 错误定义

```go
var (
    ErrNoInstances       = errors.New("no instances available")
    ErrInstanceNotFound  = errors.New("instance not found")
    ErrInvalidInstance   = errors.New("invalid instance")
    ErrBalancerClosed    = errors.New("balancer closed")
    ErrDuplicateInstance = errors.New("duplicate instance")
    ErrNoClientFactory   = errors.New("client factory not provided")
    ErrClientCreateFailed = errors.New("failed to create client")
)
```

## 五、实施阶段

### 阶段 1: 核心框架（优先）✅

**目标**: 建立基础架构和第一个可用实现

**文件清单**:
- [x] `pkg/rpc/governance/loadbalancer/interface.go` - 核心接口
- [x] `pkg/rpc/governance/loadbalancer/options.go` - 选项模式
- [x] `pkg/rpc/governance/loadbalancer/errors.go` - 错误定义
- [x] `pkg/rpc/governance/loadbalancer/README.md` - 包文档
- [x] `pkg/rpc/governance/loadbalancer/base/balancer.go` - 基础实现
- [x] `pkg/rpc/governance/loadbalancer/implementations/roundrobin/balancer.go` - 轮询算法
- [x] `pkg/rpc/governance/loadbalancer/implementations/roundrobin/balancer_test.go` - 单元测试

**验证点**:
- [x] 代码可编译 (`go build ./...`)
- [x] 测试通过 (`go test ./pkg/rpc/governance/loadbalancer/...`)
- [x] 遵循 `docs/AI_CODING_RULES.md` 规范
- [x] 所有文件顶部包含 prompt 引用

### 阶段 2: 基础算法 ✅

**目标**: 实现简单的负载均衡算法

**文件清单**:
- [x] `pkg/rpc/governance/loadbalancer/implementations/random/balancer.go` - 随机算法
- [x] `pkg/rpc/governance/loadbalancer/implementations/random/balancer_test.go` - 单元测试

**验证点**:
- [x] 代码可编译
- [x] 测试通过 (覆盖率 92.6%)
- [x] 并发安全
- [x] 支持确定性测试 (NewWithSeed)

### 阶段 3: 高级算法 ✅

**目标**: 实现复杂的负载均衡算法

#### Weighted Round Robin (加权轮询)

**文件清单**:
- [x] `pkg/rpc/governance/loadbalancer/implementations/weighted/balancer.go` - 加权轮询算法
- [x] `pkg/rpc/governance/loadbalancer/implementations/weighted/balancer_test.go` - 单元测试

**验证点**:
- [x] 代码可编译
- [x] 测试通过 (覆盖率 88.3%)
- [x] 平滑加权轮询算法
- [x] 支持动态权重更新

#### Consistent Hash (一致性哈希)

**文件清单**:
- [x] `pkg/rpc/governance/loadbalancer/implementations/consistenthash/ring.go` - 哈希环实现
- [x] `pkg/rpc/governance/loadbalancer/implementations/consistenthash/balancer.go` - 一致性哈希算法
- [x] `pkg/rpc/governance/loadbalancer/implementations/consistenthash/balancer_test.go` - 单元测试

**验证点**:
- [x] 代码可编译
- [x] 测试通过 (覆盖率 90.0%)
- [x] Key-based 稳定路由
- [x] 虚拟节点支持
- [x] 实例增减时最小化影响

### 阶段 4: 测试与文档（后续）

**目标**: 完善测试和优化性能

**任务清单**:
- [ ] 基准测试（benchmark）
- [ ] 性能优化
- [ ] 更多使用示例
- [ ] 压力测试

## 六、关键实现要点

### 1. 泛型使用

```go
// ✅ 正确
type Balancer[T any] interface {
    SelectClient(ctx context.Context, opts ...Option) (T, error)
}
```

### 2. 线程安全

- 使用 `sync.RWMutex` 保护共享数据
- 读操作使用 `RLock/RUnlock`
- 写操作使用 `Lock/Unlock`

### 3. Client 缓存（双检锁）

```go
// 1. 读锁检查
b.mu.RLock()
if client, ok := b.clients[id]; ok {
    b.mu.RUnlock()
    return client, nil
}
b.mu.RUnlock()

// 2. 写锁创建
b.mu.Lock()
defer b.mu.Unlock()

// 3. 再次检查（双检锁）
if client, ok := b.clients[id]; ok {
    return client, nil
}
```

### 4. io.Closer 检测

```go
// 使用 any() 进行类型断言
if closer, ok := any(client).(io.Closer); ok {
    closer.Close()
}
```

### 5. Key-based 选择逻辑

```go
func (b *Balancer[T]) Select(ctx context.Context, opts ...Option) (*Instance, error) {
    options := MergeOptions(opts...)
    
    // 有 Key -> 使用一致性哈希
    if options.Key != "" {
        return b.consistentHash.SelectByKey(ctx, options.Key, options)
    }
    
    // 无 Key -> 使用默认策略
    return b.strategy.Select(ctx, options)
}
```

## 七、使用示例

### 示例 1: 基础使用（gRPC）

```go
package main

import (
    "context"
    "google.golang.org/grpc"
    "github.com/solo-kingdom/uniface/pkg/rpc/governance/loadbalancer"
    "github.com/solo-kingdom/uniface/pkg/rpc/governance/loadbalancer/implementations/roundrobin"
)

func main() {
    // 创建轮询负载均衡器（泛型：*grpc.ClientConn）
    lb := roundrobin.New[*grpc.ClientConn]()
    
    // 添加实例
    lb.Add(context.Background(), &loadbalancer.Instance{
        ID:      "grpc-server-1",
        Address: "192.168.1.1",
        Port:    9090,
        Weight:  100,
    })
    
    // 设置 client 工厂
    factory := loadbalancer.WithClientFactory(func(inst *loadbalancer.Instance) (*grpc.ClientConn, error) {
        addr := fmt.Sprintf("%s:%d", inst.Address, inst.Port)
        return grpc.Dial(addr, grpc.WithInsecure())
    })
    
    // 选择 client（自动创建并缓存）
    client, err := lb.SelectClient(context.Background(), factory)
    if err != nil {
        panic(err)
    }
    
    // 使用 client
    _ = client
}
```

### 示例 2: Key-based 选择（分片场景）

```go
// 用户服务分片
func (s *UserService) GetUser(ctx context.Context, userID string) (*User, error) {
    // 根据 userID 选择稳定的 client（相同 userID 总是访问同一台服务器）
    client, err := s.lb.SelectClient(ctx, 
        loadbalancer.WithKey(userID), // Key-based 路由
        loadbalancer.WithClientFactory(s.createGRPCClient),
    )
    if err != nil {
        return nil, err
    }
    
    // 调用 RPC
    return client.GetUser(ctx, &GetUserRequest{ID: userID})
}
```

### 示例 3: 加权轮询

```go
// 异构服务器集群
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

// 流量分配：大约 77% 到 high-perf-1，23% 到 low-perf-1
```

## 八、测试策略

### 单元测试覆盖

1. **实例管理**
   - Add 正常实例
   - Add 重复实例
   - Add 无效实例
   - Remove 存在的实例
   - Remove 不存在的实例
   - Update 实例

2. **选择逻辑**
   - Select 轮询选择
   - Select 空实例列表
   - SelectClient 创建 client
   - SelectClient 复用 client

3. **生命周期**
   - Close 关闭所有 client
   - 关闭后操作返回错误

4. **并发安全**
   - 并发 Add/Remove
   - 并发 Select
   - 并发 SelectClient

## 九、依赖关系

```
interface.go
    ↓
options.go
    ↓
errors.go
    ↓
base/balancer.go (依赖 interface, options, errors)
    ↓
implementations/roundrobin/balancer.go (依赖 base, interface, options)
```

## 十、注意事项

1. **Prompt 引用**: 每个文件顶部必须包含 prompt 引用
2. **代码风格**: 遵循 `docs/AI_CODING_RULES.md` 规范
3. **测试覆盖**: 单元测试覆盖率 > 80%
4. **线程安全**: 所有实现必须是线程安全的
5. **资源管理**: 正确关闭 client 和释放资源

## 十一、后续扩展

### 服务发现集成

```go
// 未来可以添加服务发现接口
type ServiceDiscovery interface {
    Watch(ctx context.Context, serviceName string) ([]*Instance, error)
    Register(ctx context.Context, instance *Instance) error
    Deregister(ctx context.Context, instanceID string) error
}
```

### 健康检查

```go
// 未来可以添加健康检查
type HealthChecker interface {
    Check(ctx context.Context, instance *Instance) (bool, error)
}
```

### 指标监控

```go
// 未来可以添加指标接口
type Metrics interface {
    RecordSelect(instanceID string, duration time.Duration)
    RecordError(instanceID string, err error)
}
```

---

**文档版本**: v1.0  
**创建日期**: 2026-03-08  
**最后更新**: 2026-03-08
