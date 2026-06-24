package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-chi/chi/v5"
	lblab "github.com/solo-kingdom/uniface/lab/internal/lb"
	labweb "github.com/solo-kingdom/uniface/lab/internal/web"
	"github.com/solo-kingdom/uniface/lab/internal/wiring"
	"github.com/solo-kingdom/uniface/pkg/rpc/governance/loadbalancer"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}
	cfg, err := wiring.LoadConfig()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	switch os.Args[1] {
	case "add":
		run(cfg, func(h *lblab.Handler) error {
			fs := flag.NewFlagSet("add", flag.ExitOnError)
			id := fs.String("id", "", "instance id")
			addr := fs.String("address", "127.0.0.1", "address")
			port := fs.Int("port", 8080, "port")
			weight := fs.Int("weight", 1, "weight")
			_ = fs.Parse(os.Args[2:])
			return h.Add(context.Background(), &loadbalancer.Instance{
				ID: *id, Address: *addr, Port: *port, Weight: *weight,
			})
		})
	case "remove":
		run(cfg, func(h *lblab.Handler) error {
			fs := flag.NewFlagSet("remove", flag.ExitOnError)
			id := fs.String("id", "", "instance id")
			_ = fs.Parse(os.Args[2:])
			return h.Remove(context.Background(), *id)
		})
	case "select":
		run(cfg, func(h *lblab.Handler) error {
			fs := flag.NewFlagSet("select", flag.ExitOnError)
			key := fs.String("key", "", "key")
			_ = fs.Parse(os.Args[2:])
			inst, err := h.Select(context.Background(), *key)
			if err != nil {
				return err
			}
			fmt.Printf("%s (%s:%d)\n", inst.ID, inst.Address, inst.Port)
			return nil
		})
	case "simulate":
		run(cfg, func(h *lblab.Handler) error {
			fs := flag.NewFlagSet("simulate", flag.ExitOnError)
			n := fs.Int("n", 1000, "iterations")
			prefix := fs.String("prefix", "sim", "key prefix")
			_ = fs.Parse(os.Args[2:])
			counts := h.Simulate(context.Background(), *n, *prefix)
			for id, c := range counts {
				fmt.Printf("%s: %d\n", id, c)
			}
			return nil
		})
	case "switch":
		fmt.Println("切换算法需修改配置 LB algo 并重启 lab-lb serve")
	case "serve":
		serve(cfg)
	default:
		usage()
		os.Exit(1)
	}
}

func run(cfg *wiring.LabConfig, fn func(*lblab.Handler) error) {
	bal, algo, err := wiring.NewBalancer(cfg.LB)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer bal.Close()
	h := lblab.NewHandler(bal, algo)
	if err := fn(h); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func serve(cfg *wiring.LabConfig) {
	bal, algo, err := wiring.NewBalancer(cfg.LB)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer bal.Close()
	h := lblab.NewHandler(bal, algo)
	srv := labweb.NewServer(":8083", func(r chi.Router) {
		lblab.RegisterAPI(r, h)
	})
	go onSignal()
	if err := srv.ListenAndServe(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	_ = algo
}

func onSignal() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
	os.Exit(0)
}

func usage() {
	fmt.Println(`lab-lb - Load Balancer 验证 CLI

用法:
  lab-lb add --id ID --address HOST --port PORT
  lab-lb remove --id ID
  lab-lb select --key KEY
  lab-lb simulate --n 1000
  lab-lb switch
  lab-lb serve`)
}
