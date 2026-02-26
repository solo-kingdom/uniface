# AI 代码生成规则 - Uniface 项目

## 项目目标
创建统一的接口系统，整合多个 API/SDK 接口，提供类型安全和可扩展性。

---

## 1. 命名规范

### 包名
- 小写单词，无下划线和连字符
- 示例：`core`, `utils`, `parser`

### 函数/方法
- 导出：PascalCase（如 `GenerateCode`）
- 私有：camelCase（如 `generateCode`）
- 布尔值：Is/Has/Can 前缀（如 `IsValid`）

### 变量/常量
- 局部变量：camelCase
- 常量：PascalCase
- 错误变量：Err 前缀（如 `ErrInvalidInput`）

---

## 2. 代码风格

### 基本约定
- 遵循 Go 标准约定（Effective Go）
- 使用 `gofmt` 格式化
- 最大行长度：120 字符
- 使用 tab 缩进

### 文件组织
- 导入顺序：标准库 → 第三方库 → internal
- 每个文件一个公开结构体/接口
- 包文档在文件顶部

### 错误处理
```go
// 始终返回错误，不要 panic
// 使用 fmt.Errorf 包装错误上下文
if err != nil {
    return fmt.Errorf("操作失败: %w", err)
}
```

---

## 3. 目录结构说明

```
cmd/uniface/     # 程序入口
internal/core/   # 核心业务逻辑
internal/utils/  # 工具函数
pkg/             # 公共 API
prompts/         # AI 提示词（重要！）
test/            # 测试文件
docs/            # 文档
```

### 依赖方向
- 低层包不依赖高层包
- 使用依赖注入提高可测试性
- 优先使用接口而非具体类型

---

## 4. 测试要求

### 覆盖率
- 目标：>80%
- 所有导出函数必须有测试

---

## 5. 文档要求

### 包文档
```go
// Package core 提供 Uniface 项目的核心功能。
// 包含接口定义、注册管理和代码生成能力。
package core
```

### 函数文档
- 简短描述
- 参数说明
- 返回值说明
- 可能的错误（如适用）

---

## 6. Prompt 使用规则 ⭐ 重要

### 生成代码前必须：
1. **检查 `prompts/` 目录**
   - `prompts/architecture/` - 架构决策
   - `prompts/features/` - 功能实现
   - `prompts/tasks/` - 具体任务

2. **阅读相关 prompt 文件**

3. **在代码中引用使用的 prompt**
```go
// 基于 prompts/features/xxx.md 实现
```

---

## 7. AI 行为规则

### 生成前
- [ ] 读取本规则文档
- [ ] 检查 prompts/ 目录
- [ ] 了解现有代码结构

### 生成时
- [ ] 遵循所有约定
- [ ] 添加文档注释
- [ ] 编写测试
- [ ] 引用相关 prompt

### 生成后
- [ ] 验证代码格式
- [ ] 确保代码可编译

---

## 8. 快速检查清单

- [ ] 命名符合规范
- [ ] 导出项有文档注释
- [ ] 错误处理完整
- [ ] 测试覆盖充分
- [ ] 无安全问题
- [ ] 引用了相关 prompt
- [ ] 代码格式正确

---

## 9. 禁止事项

- ❌ 使用全局变量维护状态
- X 不要修改 prompts 目录下的文件
