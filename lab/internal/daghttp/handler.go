// Package daghttp 把 HTTP 请求经 DAG echo 图排空到终态后返回，演示
// 「请求 = 实例、排空到终态、终态 payload 作为响应」的请求编排范式。
//
// 本包与 lab/internal/dag 完全隔离：自带 Runtime、units 与 fixtures；
// 通过统一 rpc.Server 抽象暴露，验证「同一 handler 可在不同传输间切换」。
// Runtime 内部基于公共 pkg/dag/invocation/app 轻量封装装配。
package daghttp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/solo-kingdom/uniface/lab/internal/web/api"
	rpcserver "github.com/solo-kingdom/uniface/pkg/rpc/server"
)

const (
	// defaultGraphID 是 echo DAG 的默认图 ID。
	defaultGraphID = "echo"
)

// Service 把 HTTP 请求经 DAG 排空到终态，并暴露 /api/status。
type Service struct {
	rt      *Runtime
	graphID string
	rec     *api.OpRecorder

	idCounter atomic.Uint64
}

// NewService 创建 daghttp 服务。graphID 为空时使用 "echo"。
func NewService(rt *Runtime, graphID string) *Service {
	if graphID == "" {
		graphID = defaultGraphID
	}
	return &Service{
		rt:      rt,
		graphID: graphID,
		rec:     api.NewOpRecorder(50),
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
//  2. 生成唯一 entityID，经 app.InvokeString 一次性 Start+Drain+Snapshot；
//  3. 终态 payload 作为响应体；COMPLETED → 200，否则 → 500 并附失败原因。
func (s *Service) Echo(ctx context.Context, req *rpcserver.Request) (*rpcserver.Response, error) {
	payload := string(req.Body)
	entityID := s.nextEntityID()

	res, err := s.rt.Invoke(ctx, s.graphID, entityID, payload)
	if err != nil {
		s.rec.Record("echo", entityID, false, err)
		return &rpcserver.Response{
			StatusCode: http.StatusInternalServerError,
			Body:       []byte("dag invoke failed: " + err.Error()),
		}, nil
	}

	body := res.Value
	status := res.Instance.Status
	completed := res.IsCompleted()
	s.rec.Record("echo", entityID, completed, nil)

	if completed {
		return &rpcserver.Response{StatusCode: http.StatusOK, Body: []byte(body)}, nil
	}
	reason := fmt.Sprintf("dag terminal status %s: %s", status, body)
	return &rpcserver.Response{
		StatusCode: http.StatusInternalServerError,
		Body:       []byte(reason),
	}, nil
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

// nextEntityID 生成全局唯一 entityID（http-<unixnano>-<counter>）。
func (s *Service) nextEntityID() string {
	n := s.idCounter.Add(1)
	return fmt.Sprintf("http-%d-%d", time.Now().UnixNano(), n)
}
