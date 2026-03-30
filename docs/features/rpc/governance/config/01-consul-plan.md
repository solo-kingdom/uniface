# Config 代码迁移与 Consul 实现计划

## 需求概述
将 config 代码从 `pkg/storage/config` 移动到 `pkg/rpc/governance/config`，并以 sub go module 的方式支持 consul 实现。

## 背景
- 当前 config 代码位于 `pkg/storage/config`，包含接口定义、选项和错误类型
- 需要将配置管理能力整合到 RPC 治理模块中
- 需要支持 consul 作为配置中心实现

## 实施步骤

### 1. 移动核心代码到 pkg/rpc/governance/config
- [ ] 创建目录结构 `pkg/rpc/governance/config`
- [ ] 移动 interface.go 到新位置
- [ ] 移动 options.go 到新位置
- [ ] 移动 errors.go 到新位置
- [ ] 移动 interface_test.go 到新位置
- [ ] 更新所有文件中的 package 注释和导入路径

### 2. 调整包引用
- [ ] 更新 package 名称和注释，确保符合 governance 模块规范
- [ ] 添加中文文档注释（符合 AI.MD 要求）

### 3. 创建 Consul 实现的 Sub Go Module
- [ ] 创建目录 `pkg/rpc/governance/config/consul`
- [ ] 创建 consul/go.mod 文件
- [ ] 实现 consul.Storage 接口
- [ ] 创建 consul 实现的选项配置
- [ ] 编写 consul 实现的测试代码
- [ ] 编写 consul 实现的示例代码

### 4. 目录结构规划
```
pkg/rpc/governance/config/
├── interface.go          # Storage 接口定义
├── options.go            # 通用配置选项
├── errors.go             # 错误类型定义
├── interface_test.go     # 接口测试
└── consul/               # Consul 实现（sub module）
    ├── go.mod            # 子模块 go.mod
    ├── consul.go         # Consul 实现
    ├── options.go        # Consul 特定选项
    ├── consul_test.go    # Consul 测试
    └── example_test.go   # 使用示例
```

### 5. Consul 实现要点
- 使用 `github.com/hashicorp/consul/api` 客户端库
- 支持以下功能：
  - Read/Write/Delete 配置
  - Watch/WatchPrefix 监听配置变更
  - KV 存储操作
  - 健康检查
  - 会话管理（用于分布式锁）
- 实现缓存机制
- 实现重试机制

### 6. Go Module 配置
```go
// pkg/rpc/governance/config/consul/go.mod
module github.com/solo-kingdom/uniface/pkg/rpc/governance/config/consul

go 1.24

require (
    github.com/hashicorp/consul/api v1.29.0
    github.com/solo-kingdom/uniface v0.0.0
)

replace github.com/solo-kingdom/uniface => ../../../../../
```

## 验证清单
- [ ] 代码可编译
- [ ] 测试通过
- [ ] 文档完整（中文）
- [ ] 符合 AI_CODING_RULES.md 规范
- [ ] 引用了相关 prompt

## 注意事项
1. 保持向后兼容性，原路径 `pkg/storage/config` 可能需要保留别名
2. 所有文档使用中文
3. 遵循 Go 代码规范
4. 确保测试覆盖率 >80%

## 预期成果
1. config 代码成功迁移到 governance 模块
2. consul 实现作为独立子模块可用
3. 完整的测试和文档
4. 可作为其他配置中心实现的参考模板
