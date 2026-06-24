package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-chi/chi/v5"
	"github.com/solo-kingdom/uniface/lab/internal/conformance"
	kvhandler "github.com/solo-kingdom/uniface/lab/internal/kv"
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
	case "set":
		runKV(cfg, func(h *kvhandler.Handler) error {
			fs := flag.NewFlagSet("set", flag.ExitOnError)
			key := fs.String("key", "", "key")
			value := fs.String("value", "", "value")
			_ = fs.Parse(os.Args[2:])
			return h.Set(context.Background(), *key, *value)
		})
	case "get":
		runKV(cfg, func(h *kvhandler.Handler) error {
			fs := flag.NewFlagSet("get", flag.ExitOnError)
			key := fs.String("key", "", "key")
			_ = fs.Parse(os.Args[2:])
			v, err := h.Get(context.Background(), *key)
			if err != nil {
				return err
			}
			fmt.Println(kvhandler.FormatValue(v))
			return nil
		})
	case "delete":
		runKV(cfg, func(h *kvhandler.Handler) error {
			fs := flag.NewFlagSet("delete", flag.ExitOnError)
			key := fs.String("key", "", "key")
			_ = fs.Parse(os.Args[2:])
			return h.Delete(context.Background(), *key)
		})
	case "list":
		runKV(cfg, func(h *kvhandler.Handler) error {
			keys, err := h.List(context.Background())
			if err != nil {
				return err
			}
			for _, k := range keys {
				fmt.Println(k)
			}
			return nil
		})
	case "exists":
		runKV(cfg, func(h *kvhandler.Handler) error {
			fs := flag.NewFlagSet("exists", flag.ExitOnError)
			key := fs.String("key", "", "key")
			_ = fs.Parse(os.Args[2:])
			ok, err := h.Exists(context.Background(), *key)
			if err != nil {
				return err
			}
			fmt.Println(ok)
			return nil
		})
	case "run-conformance":
		runKV(cfg, func(h *kvhandler.Handler) error {
			result := conformance.RunKV(context.Background(), h.Store())
			fmt.Printf("passed=%d failed=%d\n", result.Passed, result.Failed)
			for _, e := range result.Errors {
				fmt.Println(e)
			}
			if result.Failed > 0 {
				return fmt.Errorf("conformance failed")
			}
			return nil
		})
	case "serve":
		serveKV(cfg)
	default:
		usage()
		os.Exit(1)
	}
}

func runKV(cfg *wiring.LabConfig, fn func(*kvhandler.Handler) error) {
	store, impl, err := wiring.NewKV(cfg.KV)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer store.Close()
	h := kvhandler.NewHandler(store, impl)
	if err := fn(h); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func serveKV(cfg *wiring.LabConfig) {
	store, impl, err := wiring.NewKV(cfg.KV)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer store.Close()
	h := kvhandler.NewHandler(store, impl)
	srv := labweb.NewServer(":8081", func(r chi.Router) {
		kvhandler.RegisterAPI(r, h)
	})
	go onSignal()
	if err := srv.ListenAndServe(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	_ = impl
}

func onSignal() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
	os.Exit(0)
}

func usage() {
	fmt.Println(`lab-kv - KV 验证 CLI

用法:
  lab-kv set --key K --value V
  lab-kv get --key K
  lab-kv delete --key K
  lab-kv list
  lab-kv exists --key K
  lab-kv run-conformance
  lab-kv serve`)
}
