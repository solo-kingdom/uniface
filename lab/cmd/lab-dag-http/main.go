package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/solo-kingdom/uniface/lab/app/daghttp"
)

const defaultAddr = ":8086"

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

	cfg, err := daghttp.LoadConfig()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := daghttp.Serve(ctx, *addr, cfg); err != nil {
		fmt.Fprintf(os.Stderr, "serve: %v\n", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Println(`lab-dag-http - DAG HTTP 服务验证 CLI

演示「HTTP 请求经 DAG echo 图排空到终态后返回」的请求编排范式。
通过统一 pkg/rpc/server 抽象启动（非直接手写 net/http）。

用法:
  lab-dag-http serve [-addr :8086]

端点:
  POST /echo        请求体经 hello→echo DAG 处理，返回 echo:hello, <body>
  GET  /api/status  域状态`)
}
