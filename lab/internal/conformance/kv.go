package conformance

import (
	"context"
	"fmt"
	"time"

	"github.com/solo-kingdom/uniface/pkg/storage/kv"
)

// Result 一致性测试结果。
type Result struct {
	Passed int      `json:"passed"`
	Failed int      `json:"errors_count"`
	Errors []string `json:"errors,omitempty"`
}

// RunKV 对 KV 实现运行一致性用例。
func RunKV(ctx context.Context, store kv.Storage) Result {
	if ctx == nil {
		ctx = context.Background()
	}
	prefix := fmt.Sprintf("lab-conformance-%d", time.Now().UnixNano())
	result := Result{}

	run := func(name string, fn func() error) {
		if err := fn(); err != nil {
			result.Failed++
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", name, err))
			return
		}
		result.Passed++
	}

	key := prefix + "/k1"
	run("set/get", func() error {
		if err := store.Set(ctx, key, "v1"); err != nil {
			return err
		}
		var out string
		if err := store.Get(ctx, key, &out); err != nil {
			return err
		}
		if out != "v1" {
			return fmt.Errorf("expected v1, got %q", out)
		}
		return nil
	})

	run("exists", func() error {
		ok, err := store.Exists(ctx, key)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("key should exist")
		}
		return nil
	})

	run("batch", func() error {
		items := map[string]interface{}{
			prefix + "/b1": "x",
			prefix + "/b2": "y",
		}
		if err := store.BatchSet(ctx, items); err != nil {
			return err
		}
		got, err := store.BatchGet(ctx, []string{prefix + "/b1", prefix + "/b2"})
		if err != nil {
			return err
		}
		if len(got) != 2 {
			return fmt.Errorf("batch get len=%d", len(got))
		}
		return nil
	})

	run("delete", func() error {
		if err := store.Delete(ctx, key); err != nil {
			return err
		}
		ok, err := store.Exists(ctx, key)
		if err != nil {
			return err
		}
		if ok {
			return fmt.Errorf("key should be deleted")
		}
		return nil
	})

	run("batch_delete", func() error {
		return store.BatchDelete(ctx, []string{prefix + "/b1", prefix + "/b2"})
	})

	run("list", func() error {
		_, err := store.List(ctx)
		return err
	})

	return result
}
