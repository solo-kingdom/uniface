// Package invocation 提供 DAG 请求式调用抽象。
//
// 本包在底层 dag.Engine / dag.LineStore 之上封装「给定 graph + payload，
// 同步排空到终态或 WAITING 并返回结果」的常见业务调用模式，避免调用方
// 反复手写 StartInstance -> DrainInstance -> GetSnapshot 胶水。
//
// 子包：
//   - memory：标准内存 Runtime 装配辅助，封装 Registry、LineStore、Engine 与 Invoker 创建
//   - loader：声明式 YAML/JSON 图解析为 GraphSpec 与内联 ComputeUnitDef
//
// Invoker 核心输入输出使用 *anypb.Any 与 snapshot，不绑定具体业务类型；
// Codec 辅助提供 protobuf message 与 Any/snapshot payload 之间的双向转换。
package invocation
