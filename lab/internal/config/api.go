package config

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// RegisterAPI 注册 Config HTTP 端点。
func RegisterAPI(r chi.Router, h *Handler) {
	r.Get("/api/status", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, h.Status())
	})
	r.Get("/api/tree", func(w http.ResponseWriter, r *http.Request) {
		prefix := r.URL.Query().Get("prefix")
		keys, err := h.List(r.Context(), prefix)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]any{"prefix": prefix, "keys": keys})
	})
	r.Get("/api/watch/events", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, h.Events())
	})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

// PanelFragment 返回 Config 卡片 HTML。
func PanelFragment(h *Handler) string {
	st := h.Status()
	class, label := "online", "在线"
	if !st.Healthy {
		class, label = "offline", "离线"
	}
	return `<div class="card"><h2>Config</h2><p class="` + class + `">` + label + `</p>` +
		`<p>实现: ` + st.Impl + `</p><p><a href="/panel/config">详情</a></p></div>`
}

// PanelDetail 返回 Config 面板详情。
func PanelDetail(h *Handler) string {
	b, _ := json.MarshalIndent(h.Status(), "", "  ")
	return `<pre>` + string(b) + `</pre>`
}
