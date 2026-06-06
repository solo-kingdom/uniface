package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	queuelab "github.com/solo-kingdom/uniface/lab/internal/queue"
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
	case "publish":
		run(cfg, func(h *queuelab.Handler) error {
			fs := flag.NewFlagSet("publish", flag.ExitOnError)
			topic := fs.String("topic", "demo", "topic")
			body := fs.String("body", `{"msg":"hi"}`, "json body")
			_ = fs.Parse(os.Args[2:])
			var payload map[string]any
			if err := json.Unmarshal([]byte(*body), &payload); err != nil {
				return err
			}
			return h.Publish(context.Background(), *topic, payload)
		})
	case "subscribe":
		runServeStyle(cfg, func(h *queuelab.Handler) error {
			fs := flag.NewFlagSet("subscribe", flag.ExitOnError)
			topic := fs.String("topic", "demo", "topic")
			_ = fs.Parse(os.Args[2:])
			ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
			defer cancel()
			if err := h.Subscribe(ctx, *topic); err != nil {
				return err
			}
			fmt.Printf("subscribed to %s\n", *topic)
			<-ctx.Done()
			return nil
		})
	case "bench":
		run(cfg, func(h *queuelab.Handler) error {
			start := time.Now()
			for i := 0; i < 100; i++ {
				if err := h.Publish(context.Background(), "bench", map[string]any{"i": i}); err != nil {
					return err
				}
			}
			fmt.Printf("published 100 messages in %s\n", time.Since(start))
			return nil
		})
	case "serve":
		serve(cfg)
	default:
		usage()
		os.Exit(1)
	}
}

func run(cfg *wiring.LabConfig, fn func(*queuelab.Handler) error) {
	h, cleanup := newHandler(cfg)
	defer cleanup()
	if err := fn(h); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runServeStyle(cfg *wiring.LabConfig, fn func(*queuelab.Handler) error) {
	h, cleanup := newHandler(cfg)
	defer cleanup()
	if err := fn(h); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newHandler(cfg *wiring.LabConfig) (*queuelab.Handler, func()) {
	q, impl, err := wiring.NewQueue(cfg.Queue)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	h := queuelab.NewHandler(q.Publisher, q.Subscriber, q.Closer, impl)
	return h, func() { _ = h.Close() }
}

func serve(cfg *wiring.LabConfig) {
	h, cleanup := newHandler(cfg)
	defer cleanup()
	srv := labweb.NewServer(":8084", func(r chi.Router) {
		queuelab.RegisterAPI(r, h)
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
	fmt.Println(`lab-queue - Queue 验证 CLI

用法:
  lab-queue publish --topic T --body JSON
  lab-queue subscribe --topic T
  lab-queue bench
  lab-queue serve`)
}
