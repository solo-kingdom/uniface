package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	labweb "github.com/solo-kingdom/uniface/lab/internal/web"
	"github.com/solo-kingdom/uniface/lab/internal/web/api"
	"github.com/solo-kingdom/uniface/lab/internal/web/ui"
	"github.com/solo-kingdom/uniface/lab/internal/wiring"
)

func main() {
	cfg, err := wiring.LoadConfig()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	uiHandler, err := ui.NewHandler()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	client := &http.Client{Timeout: 3 * time.Second}
	srv := labweb.NewServer(":3000", func(r chi.Router) {
		uiHandler.Register(r)
		r.Get("/api/dashboard", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(renderDashboard(cfg, client)))
		})
		r.Get("/api/panel/{domain}", func(w http.ResponseWriter, r *http.Request) {
			domain := chi.URLParam(r, "domain")
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(renderPanel(cfg, client, domain)))
		})
		r.Get("/api/status/all", func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(w, fetchAll(cfg, client))
		})
	})

	go onSignal()
	if err := srv.ListenAndServe(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func renderDashboard(cfg *wiring.LabConfig, client *http.Client) string {
	out := ""
	domains := []struct {
		name string
		url  string
	}{
		{"kv", cfg.Services.KV},
		{"config", cfg.Services.Config},
		{"lb", cfg.Services.LB},
		{"queue", cfg.Services.Queue},
		{"dag", cfg.Services.DAG},
	}
	for _, d := range domains {
		st, err := fetchStatus(client, d.url+"/api/status")
		if err != nil {
			out += offlineCard(d.name)
			continue
		}
		out += cardFromStatus(st, d.name)
	}
	return out
}

func renderPanel(cfg *wiring.LabConfig, client *http.Client, domain string) string {
	urls := map[string]string{
		"kv": cfg.Services.KV, "config": cfg.Services.Config, "lb": cfg.Services.LB,
		"queue": cfg.Services.Queue, "dag": cfg.Services.DAG,
	}
	base, ok := urls[domain]
	if !ok {
		return `<pre>unknown domain</pre>`
	}
	st, err := fetchStatus(client, base+"/api/status")
	if err != nil {
		return `<pre>offline: ` + err.Error() + `</pre>`
	}
	b, _ := json.MarshalIndent(st, "", "  ")
	return `<pre>` + string(b) + `</pre>`
}

func fetchAll(cfg *wiring.LabConfig, client *http.Client) map[string]any {
	result := map[string]any{}
	for name, url := range map[string]string{
		"kv": cfg.Services.KV, "config": cfg.Services.Config, "lb": cfg.Services.LB,
		"queue": cfg.Services.Queue, "dag": cfg.Services.DAG,
	} {
		st, err := fetchStatus(client, url+"/api/status")
		if err != nil {
			result[name] = map[string]any{"healthy": false, "error": err.Error()}
			continue
		}
		result[name] = st
	}
	return result
}

func fetchStatus(client *http.Client, url string) (api.Status, error) {
	resp, err := client.Get(url)
	if err != nil {
		return api.Status{}, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return api.Status{}, err
	}
	var st api.Status
	if err := json.Unmarshal(body, &st); err != nil {
		return api.Status{}, err
	}
	return st, nil
}

func offlineCard(name string) string {
	return `<div class="card"><h2>` + name + `</h2><p class="offline">离线</p></div>`
}

func cardFromStatus(st api.Status, name string) string {
	class, label := "online", "在线"
	if !st.Healthy {
		class, label = "offline", "离线"
	}
	return `<div class="card"><h2>` + name + `</h2><p class="` + class + `">` + label +
		`</p><p>` + st.Impl + `</p><p><a href="/panel/` + name + `">详情</a></p></div>`
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func onSignal() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
	os.Exit(0)
}
