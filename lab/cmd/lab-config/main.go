package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-chi/chi/v5"
	cfglab "github.com/solo-kingdom/uniface/lab/internal/config"
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
	case "put":
		run(cfg, func(h *cfglab.Handler) error {
			fs := flag.NewFlagSet("put", flag.ExitOnError)
			key := fs.String("key", "", "key")
			value := fs.String("value", "", "value")
			_ = fs.Parse(os.Args[2:])
			return h.Put(context.Background(), *key, *value)
		})
	case "get":
		run(cfg, func(h *cfglab.Handler) error {
			fs := flag.NewFlagSet("get", flag.ExitOnError)
			key := fs.String("key", "", "key")
			_ = fs.Parse(os.Args[2:])
			v, err := h.Get(context.Background(), *key)
			if err != nil {
				return err
			}
			fmt.Printf("%v\n", v)
			return nil
		})
	case "delete":
		run(cfg, func(h *cfglab.Handler) error {
			fs := flag.NewFlagSet("delete", flag.ExitOnError)
			key := fs.String("key", "", "key")
			_ = fs.Parse(os.Args[2:])
			return h.Delete(context.Background(), *key)
		})
	case "watch":
		run(cfg, func(h *cfglab.Handler) error {
			fs := flag.NewFlagSet("watch", flag.ExitOnError)
			prefix := fs.String("prefix", "", "prefix")
			_ = fs.Parse(os.Args[2:])
			ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
			defer cancel()
			return h.WatchPrefix(ctx, *prefix)
		})
	case "serve":
		serve(cfg)
	default:
		usage()
		os.Exit(1)
	}
}

func run(cfg *wiring.LabConfig, fn func(*cfglab.Handler) error) {
	store, impl, err := wiring.NewConfigStorage(cfg.Config)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer store.Close()
	h := cfglab.NewHandler(store, impl)
	if err := fn(h); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func serve(cfg *wiring.LabConfig) {
	store, impl, err := wiring.NewConfigStorage(cfg.Config)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer store.Close()
	h := cfglab.NewHandler(store, impl)
	srv := labweb.NewServer(":8082", func(r chi.Router) {
		cfglab.RegisterAPI(r, h)
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
	fmt.Println(`lab-config - Config 验证 CLI

用法:
  lab-config put --key K --value V
  lab-config get --key K
  lab-config delete --key K
  lab-config watch --prefix P
  lab-config serve`)
}
