// Package app 在 invocation/memory.Runtime、Loader 与 Codec 之上提供请求式 DAG
// 应用封装，面向「输入 payload、执行 graph、返回终态结果」的常见同步调用场景。
//
// 本包组合现有底层组件，不替代 dag.Engine、invocation.Invoker 或 memory.Runtime。
// 复杂生命周期、异步 Signal/Saga 等场景应继续使用底层 API。
package app
