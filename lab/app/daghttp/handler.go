// Package daghttp 把 HTTP 请求经 DAG echo 图排空到终态后返回，演示
// 「请求 = 实例、排空到终态、终态 payload 作为响应」的请求编排范式。
//
// 本包与 lab/internal/dag 完全隔离：自带 StringApp、units 与 fixtures；
// 通过统一 rpc.Server 抽象暴露，验证「同一 handler 可在不同传输间切换」。
// StringApp 内部基于公共 pkg/dag/invocation/app 轻量封装装配。
//
// 默认常量与 Config schema 见同包 config.go。
package daghttp

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/solo-kingdom/uniface/lab/internal/web/api"
	"github.com/solo-kingdom/uniface/pkg/dag/invocation/app"
	rpcserver "github.com/solo-kingdom/uniface/pkg/rpc/server"
	"github.com/solo-kingdom/uniface/pkg/rpc/server/dagbridge"
)

// Service 把 HTTP 请求经 DAG 排空到终态，并暴露 /api/status。
type Service struct {
	rt      *app.StringApp
	graphID string
	rec     *api.OpRecorder
	idGen   *app.EntityIDGen
}

// NewService 创建 daghttp 服务。graphID 为空时使用 "echo"。
func NewService(rt *app.StringApp, graphID string) *Service {
	if graphID == "" {
		graphID = defaultGraphID
	}
	return &Service{
		rt:      rt,
		graphID: graphID,
		rec:     api.NewOpRecorder(50),
		idGen:   rt.NewEntityIDGen("http"),
	}
}

// Register 在 rpc Server 上注册 /echo 与 /api/status 路由。
func (s *Service) Register(srv rpcserver.Server) error {
	if err := srv.Handle(rpcserver.Route{Method: http.MethodPost, Path: "/echo"}, s.Echo); err != nil {
		return err
	}
	return srv.Handle(rpcserver.Route{Method: http.MethodGet, Path: "/api/status"}, s.Status)
}

// Echo 是 POST /echo 处理器：
//  1. 读 Request.Body 作为 payload；
//  2. 生成唯一 entityID，经 StringApp.InvokeString 一次性 Start+Drain+Snapshot；
//  3. 终态 payload 作为响应体；终态映射由 dagbridge.ResponseForTerminalResult 统一翻译。
func (s *Service) Echo(ctx context.Context, req *rpcserver.Request) (*rpcserver.Response, error) {
	payload := string(req.Body)
	entityID := s.idGen.Next()

	res, err := s.rt.InvokeString(ctx, s.graphID, entityID, payload)
	if err != nil {
		s.rec.Record("echo", entityID, false, err)
		return &rpcserver.Response{
			StatusCode: http.StatusInternalServerError,
			Body:       []byte("dag invoke failed: " + err.Error()),
		}, nil
	}

	s.rec.RecordResult("echo", entityID, res)
	return dagbridge.ResponseForTerminalResult(res), nil
}

// Status 是 GET /api/status 处理器，返回域状态 JSON。
func (s *Service) Status(_ context.Context, _ *rpcserver.Request) (*rpcserver.Response, error) {
	b, _ := json.Marshal(s.StatusInfo())
	return &rpcserver.Response{
		StatusCode: http.StatusOK,
		Header:     map[string][]string{"Content-Type": {"application/json"}},
		Body:       b,
	}, nil
}

// StatusInfo 返回域状态（供直接调用）。
func (s *Service) StatusInfo() api.Status {
	return api.Status{
		Domain:    "daghttp",
		Impl:      "memory",
		Healthy:   true,
		RecentOps: s.rec.Snapshot(),
		Extra: map[string]any{
			"graph":         s.graphID,
			"loaded_graphs": s.rt.LoadedGraphs(),
		},
		CollectedAt: time.Now(),
	}
}
