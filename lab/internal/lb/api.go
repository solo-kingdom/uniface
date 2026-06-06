package lb

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// RegisterAPI 注册 LB HTTP 端点。
func RegisterAPI(r chi.Router, h *Handler) {
	r.Get("/api/status", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, h.Status())
	})
	r.Get("/api/instances", func(w http.ResponseWriter, r *http.Request) {
		instances, err := h.GetAll(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, instances)
	})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

// PanelFragment 返回 LB 卡片 HTML。
func PanelFragment(h *Handler) string {
	st := h.Status()
	class, label := "online", "在线"
	if !st.Healthy {
		class, label = "offline", "离线"
	}
	return `<div class="card"><h2>LB</h2><p class="` + class + `">` + label + `</p>` +
		`<p>算法: ` + st.Impl + `</p><p><a href="/panel/lb">详情</a></p></div>`
}

// PanelDetail 返回 LB 面板详情。
func PanelDetail(h *Handler) string {
	b, _ := json.MarshalIndent(h.Status(), "", "  ")
	return `<pre>` + string(b) + `</pre>`
}
