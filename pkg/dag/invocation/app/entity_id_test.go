package app_test

import (
	"sync"
	"testing"

	"github.com/solo-kingdom/uniface/pkg/dag/invocation/app"
)

// TestEntityIDGen_Monotonic 验证同一 gen 串行 Next 返回 "http-1" / "http-2" / "http-3"。
func TestEntityIDGen_Monotonic(t *testing.T) {
	rt := app.New()
	defer rt.Close()

	gen := rt.NewEntityIDGen("http")
	want := []string{"http-1", "http-2", "http-3"}
	for i, w := range want {
		got := gen.Next()
		if got != w {
			t.Fatalf("Next() #%d = %q, want %q", i+1, got, w)
		}
	}
}

// TestEntityIDGen_ConcurrentUniqueness 验证 1000 goroutine 并发 Next 返回 1000 个唯一 ID。
func TestEntityIDGen_ConcurrentUniqueness(t *testing.T) {
	rt := app.New()
	defer rt.Close()

	gen := rt.NewEntityIDGen("c")
	const N = 1000
	out := make(chan string, N)
	var wg sync.WaitGroup
	wg.Add(N)
	for i := 0; i < N; i++ {
		go func() {
			defer wg.Done()
			out <- gen.Next()
		}()
	}
	wg.Wait()
	close(out)

	seen := make(map[string]struct{}, N)
	for id := range out {
		if _, dup := seen[id]; dup {
			t.Fatalf("duplicate id: %q", id)
		}
		seen[id] = struct{}{}
	}
	if len(seen) != N {
		t.Fatalf("unique count = %d, want %d", len(seen), N)
	}
}

// TestEntityIDGen_DefaultPrefix 验证空 prefix 时使用 "dag"。
func TestEntityIDGen_DefaultPrefix(t *testing.T) {
	rt := app.New()
	defer rt.Close()

	gen := rt.NewEntityIDGen("")
	if got := gen.Next(); got != "dag-1" {
		t.Fatalf("Next() = %q, want dag-1", got)
	}
}

// TestEntityIDGen_MultipleGeneratorsIndependent 验证多次 NewEntityIDGen 返回独立计数器。
func TestEntityIDGen_MultipleGeneratorsIndependent(t *testing.T) {
	rt := app.New()
	defer rt.Close()

	genA := rt.NewEntityIDGen("a")
	genB := rt.NewEntityIDGen("b")

	if got := genA.Next(); got != "a-1" {
		t.Fatalf("genA.Next = %q, want a-1", got)
	}
	if got := genB.Next(); got != "b-1" {
		t.Fatalf("genB.Next = %q, want b-1", got)
	}
	// 互不干扰：genA 继续 a-2，genB 继续 b-2。
	if got := genA.Next(); got != "a-2" {
		t.Fatalf("genA.Next #2 = %q, want a-2", got)
	}
	if got := genB.Next(); got != "b-2" {
		t.Fatalf("genB.Next #2 = %q, want b-2", got)
	}
}
