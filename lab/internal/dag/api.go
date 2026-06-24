package dag

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// RegisterAPI 注册 DAG HTTP 端点。
func RegisterAPI(r chi.Router, rt *Runtime) {
	r.Get("/api/status", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, rt.Status())
	})
	r.Get("/api/instances/{entityID}", func(w http.ResponseWriter, r *http.Request) {
		entityID := chi.URLParam(r, "entityID")
		inst, err := rt.GetInstance(r.Context(), entityID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		writeJSON(w, inst)
	})
	r.Get("/api/journal/{entityID}", func(w http.ResponseWriter, r *http.Request) {
		entityID := chi.URLParam(r, "entityID")
		entries, err := rt.Journal(r.Context(), entityID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, entries)
	})
	r.Get("/api/saga/{entityID}", func(w http.ResponseWriter, r *http.Request) {
		entityID := chi.URLParam(r, "entityID")
		state, err := rt.SagaState(r.Context(), entityID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, state)
	})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

// PanelFragment 返回 DAG 卡片 HTML。
func PanelFragment(rt *Runtime) string {
	st := rt.Status()
	return `<div class="card"><h2>DAG</h2><p class="online">在线</p>` +
		`<p>Store: ` + st.Impl + `</p><p><a href="/panel/dag">详情</a></p></div>`
}

// PanelDetail 返回 DAG 面板详情。
func PanelDetail(rt *Runtime) string {
	b, _ := json.MarshalIndent(rt.Status(), "", "  ")
	return `<pre>` + string(b) + `</pre>`
}
