package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/solo-kingdom/uniface/lab/app/dagsignal"
)

const defaultAddr = ":8087"

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}
	switch os.Args[1] {
	case "serve":
		serve()
	case "-h", "--help", "help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand %q\n", os.Args[1])
		usage()
		os.Exit(1)
	}
}

func serve() {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	addr := fs.String("addr", defaultAddr, "listen address")
	_ = fs.Parse(os.Args[2:])

	cfg, err := dagsignal.LoadConfig()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := dagsignal.Serve(ctx, *addr, cfg); err != nil {
		fmt.Fprintf(os.Stderr, "serve: %v\n", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Println(`lab-dag-signal - DAG Signal 异步编排 HTTP 服务验证 CLI

演示「HTTP 请求 → 实例停在 WAITING → 另一端点 signal 推进到终态」的异步编排范式。
通过统一 pkg/rpc/server 抽象启动（非直接手写 net/http），底层经
sa.Runtime.Memory().Engine() 走 StartInstance / DeliverSignal / DrainInstance。

用法:
  lab-dag-signal serve [-addr :8087]

端点:
  POST /start                启动实例并排空到 WAITING，返回 202 + {"entity_id","status":"WAITING"}
  POST /signal/{entityID}    投递 signal（?signal= 覆盖，默认 approval）推进到终态
  GET  /instances/{entityID} 查询实例状态
  GET  /api/status           域状态`)
}
