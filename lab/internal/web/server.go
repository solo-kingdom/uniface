package web

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// Server 封装 chi HTTP 服务。
type Server struct {
	addr   string
	router chi.Router
	server *http.Server
}

// NewServer 创建 HTTP 服务框架。
func NewServer(addr string, register func(r chi.Router)) *Server {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	if register != nil {
		register(r)
	}

	return &Server{
		addr:   addr,
		router: r,
		server: &http.Server{Addr: addr, Handler: r},
	}
}

// Router 返回 chi 路由器。
func (s *Server) Router() chi.Router {
	return s.router
}

// ListenAndServe 启动 HTTP 服务。
func (s *Server) ListenAndServe() error {
	fmt.Printf("lab server listening on %s\n", s.addr)
	return s.server.ListenAndServe()
}

// Shutdown 优雅关闭服务。
func (s *Server) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}
