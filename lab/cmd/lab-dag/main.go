package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-chi/chi/v5"
	daglab "github.com/solo-kingdom/uniface/lab/internal/dag"
	labweb "github.com/solo-kingdom/uniface/lab/internal/web"
	"github.com/solo-kingdom/uniface/lab/internal/wiring"
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
	case "graph":
		if len(os.Args) < 3 || os.Args[2] != "load" {
			usage()
			os.Exit(1)
		}
		run(cfg, func(rt *daglab.Runtime) error {
			fs := flag.NewFlagSet("graph load", flag.ExitOnError)
			file := fs.String("file", "", "yaml file")
			name := fs.String("graph", "", "fixture graph id")
			_ = fs.Parse(os.Args[3:])
			if *file != "" {
				spec, err := rt.LoadGraphFile(*file)
				if err != nil {
					return err
				}
				fmt.Printf("loaded graph %s\n", spec.Version.GraphId)
				return nil
			}
			spec, err := rt.LoadFixture(*name)
			if err != nil {
				return err
			}
			fmt.Printf("loaded graph %s\n", spec.Version.GraphId)
			return nil
		})
	case "start":
		run(cfg, func(rt *daglab.Runtime) error {
			fs := flag.NewFlagSet("start", flag.ExitOnError)
			graph := fs.String("graph", "echo", "graph id")
			entityID := fs.String("entity-id", "inst-001", "entity id")
			payload := fs.String("payload", "hello", "payload")
			mockAddr := fs.String("mock-http", "", "start built-in mock HTTP server at this addr (e.g. 127.0.0.1:18099) for http_call fixture")
			_ = fs.Parse(os.Args[2:])
			if *mockAddr != "" {
				if err := rt.StartMockHTTPServer(*mockAddr); err != nil {
					return fmt.Errorf("start mock http: %w", err)
				}
			}
			if _, err := rt.LoadFixture(*graph); err != nil {
				return err
			}
			inst, err := rt.Start(context.Background(), *graph, *entityID, *payload)
			if err != nil {
				return err
			}
			for i := 0; i < 20; i++ {
				_ = rt.RunOnce(context.Background())
			}
			// 重新查询以反映 RunOnce 后的真实状态（inst 是 Start 时的快照）。
			final, _ := rt.GetInstance(context.Background(), *entityID)
			if final != nil {
				inst = final
			}
			fmt.Printf("status=%v node=%s\n", inst.Status, inst.CurrentNodeId)
			journal, _ := rt.Journal(context.Background(), *entityID)
			for _, e := range journal {
				fmt.Printf("  journal kind=%v node=%s reason=%q\n", e.Kind, e.NodeId, e.FailureReason)
			}
			return nil
		})
	case "status":
		run(cfg, func(rt *daglab.Runtime) error {
			fs := flag.NewFlagSet("status", flag.ExitOnError)
			entityID := fs.String("entity-id", "", "entity id")
			_ = fs.Parse(os.Args[2:])
			inst, err := rt.GetInstance(context.Background(), *entityID)
			if err != nil {
				return err
			}
			fmt.Printf("%+v\n", inst)
			return nil
		})
	case "signal":
		run(cfg, func(rt *daglab.Runtime) error {
			fs := flag.NewFlagSet("signal", flag.ExitOnError)
			entityID := fs.String("entity-id", "", "entity id")
			sig := fs.String("signal", "", "signal name")
			_ = fs.Parse(os.Args[2:])
			return rt.DeliverSignal(context.Background(), *entityID, *sig, "cli-delivery")
		})
	case "journal":
		run(cfg, func(rt *daglab.Runtime) error {
			fs := flag.NewFlagSet("journal", flag.ExitOnError)
			entityID := fs.String("entity-id", "", "entity id")
			_ = fs.Parse(os.Args[2:])
			entries, err := rt.Journal(context.Background(), *entityID)
			if err != nil {
				return err
			}
			for _, e := range entries {
				fmt.Printf("%+v\n", e)
			}
			return nil
		})
	case "run-once":
		run(cfg, func(rt *daglab.Runtime) error {
			return rt.RunOnce(context.Background())
		})
	case "serve":
		serve(cfg)
	default:
		usage()
		os.Exit(1)
	}
}

func run(cfg *wiring.LabConfig, fn func(*daglab.Runtime) error) {
	rt, _, err := wiring.NewDAG(cfg.DAG)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer rt.Close()
	if err := fn(rt); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func serve(cfg *wiring.LabConfig) {
	rt, _, err := wiring.NewDAG(cfg.DAG)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer rt.Close()
	for _, g := range []string{"echo", "saga_compensate", "fork_join", "approval_branch", "http_call"} {
		_, _ = rt.LoadFixture(g)
	}
	// 启动内置 mock HTTP 服务，作为 http_call fixture 的 HttpUnit 目标（url 直连 127.0.0.1:18099）。
	if err := rt.StartMockHTTPServer("127.0.0.1:18099"); err != nil {
		fmt.Fprintf(os.Stderr, "start mock http server: %v\n", err)
	}
	srv := labweb.NewServer(":8085", func(r chi.Router) {
		daglab.RegisterAPI(r, rt)
	})
	go onSignal()
	if err := srv.ListenAndServe(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func onSignal() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
	os.Exit(0)
}

func usage() {
	fmt.Println(`lab-dag - DAG 验证 CLI

用法:
  lab-dag graph load --file PATH
  lab-dag graph load --graph echo
  lab-dag start --graph echo --entity-id ID
  lab-dag start --graph http-call --entity-id ID --mock-http 127.0.0.1:18099
  lab-dag status --entity-id ID
  lab-dag signal --entity-id ID --signal NAME
  lab-dag journal --entity-id ID
  lab-dag run-once
  lab-dag serve`)
}
