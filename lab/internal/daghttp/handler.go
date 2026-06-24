// Package daghttp 把 HTTP 请求经 DAG echo 图排空到终态后返回，演示
// 「请求 = 实例、排空到终态、终态 payload 作为响应」的请求编排范式。
//
// 本包与 lab/internal/dag 完全隔离：自带 Runtime、units 与 fixtures；
// 通过统一 rpc.Server 抽象暴露，验证「同一 handler 可在不同传输间切换」。
package daghttp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"

	dagv1 "github.com/solo-kingdom/uniface/api/dag/v1"
	"github.com/solo-kingdom/uniface/lab/internal/web/api"
	rpcserver "github.com/solo-kingdom/uniface/pkg/rpc/server"
	"google.golang.org/protobuf/types/known/wrapperspb"
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
//  2. 生成唯一 entityID，Start 一个 echo 图实例；
//  3. 排空到终态（COMPLETED/FAILED/COMPENSATED）或达上限；
//  4. 终态 payload（StringValue）作为响应体；
//     COMPLETED → 200，否则 → 500 并附失败原因。
func (s *Service) Echo(ctx context.Context, req *rpcserver.Request) (*rpcserver.Response, error) {
	payload := string(req.Body)
	entityID := s.nextEntityID()

	if _, err := s.rt.Start(ctx, s.graphID, entityID, payload); err != nil {
		s.rec.Record("echo", entityID, false, err)
		return &rpcserver.Response{
			StatusCode: http.StatusInternalServerError,
			Body:       []byte("dag start failed: " + err.Error()),
		}, nil
	}

	inst, err := s.rt.Drain(ctx, entityID)
	if err != nil {
		s.rec.Record("echo", entityID, false, err)
		return &rpcserver.Response{
			StatusCode: http.StatusInternalServerError,
			Body:       []byte("dag drain failed: " + err.Error()),
		}, nil
	}
	status := inst.Status
	body := s.snapshotPayload(ctx, entityID)
	completed := status == dagv1.InstanceStatus_INSTANCE_STATUS_COMPLETED
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

// snapshotPayload 读取实例终态 payload（StringValue），失败回退为原始字节。
func (s *Service) snapshotPayload(ctx context.Context, entityID string) string {
	snap, err := s.rt.Store().GetSnapshot(ctx, &dagv1.EntityRef{EntityId: entityID})
	if err != nil || snap == nil || snap.Payload == nil {
		return ""
	}
	var sv wrapperspb.StringValue
	if err := snap.Payload.UnmarshalTo(&sv); err != nil {
		return string(snap.Payload.Value)
	}
	return sv.GetValue()
}
