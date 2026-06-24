package ui

import (
	"embed"
	"html/template"
	"io/fs"
	"net/http"
)

//go:embed templates/*.html
var templatesFS embed.FS

// Handler 提供嵌入式 HTML + htmx 页面。
type Handler struct {
	templates *template.Template
}

// NewHandler 创建 UI 处理器。
func NewHandler() (*Handler, error) {
	tmpl, err := template.ParseFS(templatesFS, "templates/*.html")
	if err != nil {
		return nil, err
	}
	return &Handler{templates: tmpl}, nil
}

// Register 注册 UI 路由。
func (h *Handler) Register(mux interface{ Get(string, http.HandlerFunc) }) {
	mux.Get("/", h.index)
	mux.Get("/panel/{domain}", h.panel)
}

func (h *Handler) index(w http.ResponseWriter, r *http.Request) {
	_ = h.templates.ExecuteTemplate(w, "index.html", map[string]any{
		"Title": "Uniface Lab Dashboard",
	})
}

func (h *Handler) panel(w http.ResponseWriter, r *http.Request) {
	domain := r.PathValue("domain")
	_ = h.templates.ExecuteTemplate(w, "panel.html", map[string]any{
		"Title":  domain + " panel",
		"Domain": domain,
	})
}

// StaticFS 返回模板文件系统（测试用）。
func StaticFS() fs.FS {
	sub, _ := fs.Sub(templatesFS, "templates")
	return sub
}
