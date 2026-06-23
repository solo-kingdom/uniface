package balanceradapter

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/solo-kingdom/uniface/pkg/rpc/governance/loadbalancer"
	"github.com/solo-kingdom/uniface/pkg/rpc/governance/loadbalancer/implementations/roundrobin"
)

func TestAdapterResolveClientReturnsBaseURL(t *testing.T) {
	lb := roundrobin.New[*http.Client]()
	if err := lb.Add(context.Background(), &loadbalancer.Instance{
		ID:      "order-1",
		Address: "10.0.1.5",
		Port:    8080,
	}); err != nil {
		t.Fatal(err)
	}

	adapter := New(lb)
	client, baseURL, err := adapter.ResolveClient(context.Background(), "order-service")
	if err != nil {
		t.Fatalf("ResolveClient failed: %v", err)
	}
	if client == nil {
		t.Fatal("expected non-nil http.Client")
	}
	if baseURL != "http://10.0.1.5:8080" {
		t.Fatalf("expected base url http://10.0.1.5:8080, got %q", baseURL)
	}
}

func TestAdapterResolveClientCachesByInstanceID(t *testing.T) {
	lb := roundrobin.New[*http.Client]()
	_ = lb.Add(context.Background(), &loadbalancer.Instance{ID: "a", Address: "1.1.1.1", Port: 80})

	called := 0
	adapter := New(lb, WithClientFactory(func(_ *loadbalancer.Instance) (*http.Client, error) {
		called++
		return &http.Client{}, nil
	}))

	// Same instance ID should reuse the cached client.
	if _, _, err := adapter.ResolveClient(context.Background(), "svc"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := adapter.ResolveClient(context.Background(), "svc"); err != nil {
		t.Fatal(err)
	}
	if called != 1 {
		t.Fatalf("expected factory called once (cached), got %d", called)
	}
}

func TestAdapterResolveClientNoInstances(t *testing.T) {
	lb := roundrobin.New[*http.Client]()
	adapter := New(lb)
	_, _, err := adapter.ResolveClient(context.Background(), "order-service")
	if err == nil {
		t.Fatal("expected error when balancer has no instances")
	}
	if !errors.Is(err, loadbalancer.ErrNoInstances) {
		t.Fatalf("expected ErrNoInstances, got %v", err)
	}
}
