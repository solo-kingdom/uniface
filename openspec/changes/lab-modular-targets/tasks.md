## 1. lab/Makefile 域注册表

- [x] 1.1 定义 `MODULES` 列表及每域 `MODULE_<name>_BIN`、`MODULE_<name>_PROFILES` 变量
- [x] 1.2 实现 `LAB_MODULES` 解析（`all` 展开、逗号/空格分隔多域）
- [x] 1.3 实现通用宏：`build-modules`、`up-module`、`down-module`（按注册表驱动）

## 2. lab/Makefile 拆分目标

- [x] 2.1 为每个域生成 `build-<module>`、`up-<module>`、`down-<module>` 目标
- [x] 2.2 重构聚合 `build`/`up`/`down` 调用通用宏，默认行为与变更前一致
- [x] 2.3 `up-<module>` 仅启动该域 compose profile（若有）及对应 serve 进程
- [x] 2.4 `down-<module>` 仅停止该域 pid 文件对应进程，不执行 `docker compose down`

## 3. 根 Makefile 转发

- [x] 3.1 为六域添加 `lab-build-<module>`、`lab-up-<module>`、`lab-down-<module>` 转发目标
- [x] 3.2 参数化目标透传 `LAB_MODULES`（`lab-build`、`lab-up`、`lab-down`）
- [x] 3.3 更新 `help` 输出，说明按域用法与 `LAB_MODULES` 示例

## 4. 文档与验证

- [x] 4.1 更新 `lab/README.md`：按域验证快速开始（以 `lab-up-dag` 为例）
- [x] 4.2 更新 `CLAUDE.md` 中 lab 构建命令说明
- [x] 4.3 手动验证：`make lab-up-dag` 仅启动 dag；`make lab-up` 仍全量启动；`make lab-down-dag` 不影响其他域
