package daghttp

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	rpchttp "github.com/solo-kingdom/uniface/pkg/rpc/server/http"
)

// TestServe_RegisterRoutes 验证 Serve 装配流程：
// buildRuntime + NewService + Register 不报错。
// 端到端 HTTP 行为见 handler_test.go（沙箱不允许绑定端口，端到端在文档/CI 跑）。
func TestServe_RegisterRoutes(t *testing.T) {
	// 使用绝对路径指向本包自带的 fixture，绕开 cwd 依赖
	fixturesAbs, err := filepath.Abs(filepath.Join(".", "fixtures", "graphs"))
	if err != nil {
		t.Fatalf("Abs: %v", err)
	}
	cfg := &Config{Store: "memory", FixturesDir: fixturesAbs}
	rt, err := buildRuntime(cfg)
	if err != nil {
		t.Fatalf("buildRuntime: %v", err)
	}
	defer rt.Close()

	svc := NewService(rt, defaultGraphID)
	srv := rpchttp.NewHTTPServer("127.0.0.1:0")
	if err := svc.Register(srv); err != nil {
		t.Fatalf("Register: %v", err)
	}
}

// TestServe_RejectsUnsupportedStore 验证 buildRuntime 对不支持的 store 返回 error。
func TestServe_RejectsUnsupportedStore(t *testing.T) {
	cfg := &Config{Store: "redis", FixturesDir: DefaultFixturesDir}
	_, err := buildRuntime(cfg)
	if err == nil {
		t.Fatal("expected error for unsupported store, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported daghttp store") {
		t.Fatalf("error = %q, want unsupported daghttp store", err.Error())
	}
}

// TestServe_RequiresFixturesDir 验证 buildRuntime 在 FixturesDir 为空时返回 error。
func TestServe_RequiresFixturesDir(t *testing.T) {
	cfg := &Config{Store: "memory", FixturesDir: ""}
	_, err := buildRuntime(cfg)
	if err == nil {
		t.Fatal("expected error for empty FixturesDir, got nil")
	}
}

// TestLoadConfig_AppliesEnvOverrides 验证 LAB_DAG_STORE / LAB_DAG_FIXTURES_DIR 覆写。
func TestLoadConfig_AppliesEnvOverrides(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "configs", "default.yaml")
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfgPath, []byte("dag:\n  store: memory\n  fixtures_dir: ignored-by-env\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("LAB_CONFIG", cfgPath)
	t.Setenv("LAB_DAG_STORE", "memory")
	t.Setenv("LAB_DAG_FIXTURES_DIR", "app/daghttp/fixtures/graphs")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Store != "memory" {
		t.Fatalf("Store = %q, want memory", cfg.Store)
	}
	if cfg.FixturesDir != "app/daghttp/fixtures/graphs" {
		t.Fatalf("FixturesDir = %q, want app/daghttp/fixtures/graphs", cfg.FixturesDir)
	}
}

// TestLoadConfig_AppliesDefaults 验证 yaml 中无 fixtures_dir 时回退到 DefaultFixturesDir。
func TestLoadConfig_AppliesDefaults(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "configs", "default.yaml")
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfgPath, []byte("dag:\n  store: memory\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("LAB_CONFIG", cfgPath)
	t.Setenv("LAB_DAG_STORE", "")
	t.Setenv("LAB_DAG_FIXTURES_DIR", "")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.FixturesDir != DefaultFixturesDir {
		t.Fatalf("FixturesDir = %q, want default %q", cfg.FixturesDir, DefaultFixturesDir)
	}
}

// TestLoadConfig_FileNotFound 验证 LAB_CONFIG 不存在时返回 error。
func TestLoadConfig_FileNotFound(t *testing.T) {
	t.Setenv("LAB_CONFIG", "/nonexistent/path/default.yaml")
	t.Setenv("LAB_DAG_STORE", "")
	t.Setenv("LAB_DAG_FIXTURES_DIR", "")

	_, err := LoadConfig()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// 引入但不使用以避免编译告警
var _ = context.Background
var _ = http.MethodPost
