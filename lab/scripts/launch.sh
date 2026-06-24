#!/bin/sh
# launch.sh — 后台启动一个进程并记录其 PID。
#
# 用法: launch.sh <pidfile> <logfile> <cmd> [args...]
#
# 以 nohup 后台运行 <cmd>（stdout/stderr 重定向到 <logfile>），
# 并把后台进程 PID 写入 <pidfile>。
#
# 设计目的：把 shell 的 $! 捕获放在本脚本内（Make 不展开脚本内容），
# 使 lab/Makefile 的「单域 up-<m>」（经 $(eval)，双重展开）与
# 「聚合 up 的 $(foreach)」（单重展开）两条路径写入一致的 PID，
# 修复此前聚合 down 因 pid 文件为 "<shellpid>!" 而无法 kill 的既有缺陷。
#
# 环境变量（如 LAB_CONFIG）由调用方在 launch.sh 前设置，
# 会随子进程继承传递给被启动的程序。

pidfile=$1
logfile=$2
shift 2

mkdir -p "$(dirname "$pidfile")"
nohup "$@" > "$logfile" 2>&1 &
echo $! > "$pidfile"
