package kv

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/solo-kingdom/uniface/lab/internal/conformance"
)

// RegisterAPI 注册 KV HTTP 端点。
func RegisterAPI(r chi.Router, h *Handler) {
	r.Get("/api/status", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, h.Status())
	})
	r.Get("/api/operations", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, h.rec.Snapshot())
	})
	r.Post("/api/conformance/run", func(w http.ResponseWriter, r *http.Request) {
		result := conformance.RunKV(r.Context(), h.store)
		cr := &ConformanceResult{
			Passed: result.Passed,
			Failed: result.Failed,
			Errors: result.Errors,
		}
		h.SetConformanceResult(cr)
		h.rec.Record("run-conformance", fmt.Sprintf("passed=%d failed=%d", cr.Passed, cr.Failed), cr.Failed == 0, nil)
		writeJSON(w, cr)
	})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

// PanelFragment 返回 htmx 面板片段。
func PanelFragment(h *Handler) string {
	st := h.Status()
	class := "online"
	label := "在线"
	if !st.Healthy {
		class = "offline"
		label = "离线"
	}
	return `<div class="card"><h2>KV</h2><p class="` + class + `">` + label + `</p>` +
		`<p>实现: ` + st.Impl + `</p>` +
		`<p><a href="/panel/kv">详情</a></p></div>`
}

// PanelDetail 返回 KV 面板详情 HTML。
func PanelDetail(h *Handler) string {
	st := h.Status()
	b, _ := json.MarshalIndent(st, "", "  ")
	return `<pre>` + string(b) + `</pre>`
}
