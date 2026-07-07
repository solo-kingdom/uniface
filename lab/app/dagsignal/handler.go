// Package dagsignal 把 HTTP 请求经 DAG approval 图的 WAIT + signal 异步编排闭环
// 演示：POST /start 让实例停在 WAITING，POST /signal/{entityID} 投递信号推进到
// 终态，GET /instances/{entityID} 查询状态。
//
// 与 daghttp（同步 InvokeString 一次性排空）不同，本包演示异步生命周期：handler
// 不调 StringApp.InvokeString，而是经 sa.Runtime.Memory().Engine() 走底层 Engine API
// （StartInstance / DeliverSignal / DrainInstance / GetInstance）。参见
// pkg/dag/invocation/app/doc.go 关于「异步场景应使用底层 API」的说明。
//
// 终态→HTTP 映射由本包私有纯函数 responseForInstance 自治：WAITING→202、
// COMPLETED→200、失败终态→500、RUNNING→202。不复用 dagbridge.ResponseForTerminalResult
// （其同步语义把 WAITING 映射为 500，与异步应用冲突）。
package dagsignal

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	dagv1 "github.com/solo-kingdom/uniface/api/dag/v1"
	"github.com/solo-kingdom/uniface/lab/internal/web/api"
	"github.com/solo-kingdom/uniface/pkg/dag"
	"github.com/solo-kingdom/uniface/pkg/dag/invocation"
	"github.com/solo-kingdom/uniface/pkg/dag/invocation/app"
	rpcserver "github.com/solo-kingdom/uniface/pkg/rpc/server"
)

// defaultSignalName 是 POST /signal 未带 ?signal= query 时使用的默认信号名。
const defaultSignalName = "approval"

// Service 演示「HTTP 请求 → 实例停在 WAITING → signal 推进到终态」的异步编排。
//
// Service 持有底层 dag.Engine（经 sa.Runtime.Memory().Engine() 取得）与 typeKey
// （经 sa.TypeKey() 取得），不持有 *app.StringApp 句柄——装配完即只用 Engine。
type Service struct {
	engine  dag.Engine
	typeKey *dagv1.EntityTypeKey
	graphID string
	rec     *api.OpRecorder
	idGen   *app.EntityIDGen
	// loadedGraphs 由 NewService 快照，用于 StatusInfo 上报而无需反向依赖 StringApp。
	loadedGraphs map[string]string
}

// NewService 创建 dagsignal 服务。graphID 为空时使用 "approval"。
//
// 装配流程：经 rt.Runtime.Memory().Engine() 取底层 Engine，经 rt.TypeKey() 取
// EntityTypeKey，经 rt.NewEntityIDGen("signal") 取 entity ID 生成器（前缀 "signal"
// 与 daghttp 的 "http" 前缀平行）。
func NewService(rt *app.StringApp, graphID string) *Service {
	if graphID == "" {
		graphID = defaultGraphID
	}
	return &Service{
		engine:       rt.Runtime.Memory().Engine(),
		typeKey:      rt.TypeKey(),
		graphID:      graphID,
		rec:          api.NewOpRecorder(50),
		idGen:        rt.NewEntityIDGen("signal"),
		loadedGraphs: rt.LoadedGraphs(),
	}
}

// Register 在 rpc Server 上注册 /start、/signal/{entityID}、/instances/{entityID}
// 与 /api/status 路由。
//
// 路径参数 {entityID} 经 Go 1.22+ ServeMux 通配符匹配；handler 内从 req.Path
// 剥离前缀提取（rpc.Server 当前不在 Request 上透传 PathValue，故采用前缀剥离）。
func (s *Service) Register(srv rpcserver.Server) error {
	routes := []struct {
		route   rpcserver.Route
		handler rpcserver.Handler
	}{
		{rpcserver.Route{Method: http.MethodPost, Path: "/start"}, s.Start},
		{rpcserver.Route{Method: http.MethodPost, Path: "/signal/{entityID}"}, s.Signal},
		{rpcserver.Route{Method: http.MethodGet, Path: "/instances/{entityID}"}, s.Instances},
		{rpcserver.Route{Method: http.MethodGet, Path: "/api/status"}, s.Status},
	}
	for _, r := range routes {
		if err := srv.Handle(r.route, r.handler); err != nil {
			return err
		}
	}
	return nil
}

// Start 是 POST /start 处理器：
//  1. 读 Request.Body 作为 payload；
//  2. 生成唯一 entityID，StartInstance 推进入图节点，DrainInstance 推进到 WAITING；
//  3. GetInstance 取当前实例，经 responseForInstance 映射为响应。
//
// approval 图入口即 wait 节点，故首帧必为 WAITING → 202。
func (s *Service) Start(ctx context.Context, req *rpcserver.Request) (*rpcserver.Response, error) {
	entityID := s.idGen.Next()

	inst, err := s.startAndDrain(ctx, entityID, string(req.Body))
	if err != nil {
		s.rec.Record("start", entityID, false, err)
		return errorResponse(http.StatusInternalServerError, "start failed: "+err.Error(), entityID), nil
	}

	s.rec.Record("start", entityID, inst != nil && isWaiting(inst), nil)
	return responseForInstance(inst), nil
}

// startAndDrain 封装 StartInstance + DrainInstance 两步。
func (s *Service) startAndDrain(ctx context.Context, entityID, payload string) (*dagv1.EntityInstance, error) {
	ref := &dagv1.EntityRef{EntityId: entityID}
	startReq := &dagv1.StartInstanceRequest{
		Ref:            ref,
		TypeKey:        s.typeKey,
		GraphVersion:   &dagv1.GraphVersion{GraphId: s.graphID, Version: "v1"},
		GraphPinPolicy: dagv1.GraphPinPolicy_GRAPH_PIN_ON_START,
	}
	if payload != "" {
		initial, err := invocation.MarshalString(payload)
		if err != nil {
			return nil, err
		}
		startReq.InitialPayload = initial
	}
	if _, err := s.engine.StartInstance(ctx, startReq); err != nil {
		return nil, err
	}
	// 推进到 WAITING（入口即 wait 节点）或终态。
	if _, err := s.engine.DrainInstance(ctx, ref); err != nil {
		return nil, err
	}
	return s.engine.GetInstance(ctx, ref)
}

// Signal 是 POST /signal/{entityID} 处理器：
//  1. 从 req.Path 剥离 /signal/ 前缀取 entityID；
//  2. 从 req.Query 的 signal 取信号名（缺省 approval）；
//  3. DeliverSignal + DrainInstance + GetInstance；
//  4. signal 名不匹配 → 400；实例不存在 → 404；终态 → 200；仍 WAITING → 202。
func (s *Service) Signal(ctx context.Context, req *rpcserver.Request) (*rpcserver.Response, error) {
	entityID := pathParam(req.Path, "/signal/")
	if entityID == "" {
		return errorResponse(http.StatusBadRequest, "missing entityID in path", ""), nil
	}
	signalName := queryValue(req.Query, "signal", defaultSignalName)

	ref := &dagv1.EntityRef{EntityId: entityID}
	if err := s.engine.DeliverSignal(ctx, &dagv1.SignalDelivery{
		EntityId:   entityID,
		SignalName: signalName,
		DeliveryId: "signal-" + entityID,
	}); err != nil {
		if errors.Is(err, dag.ErrSignalMismatch) {
			s.rec.Record("signal", entityID+":"+signalName, false, err)
			return errorResponse(http.StatusBadRequest, "signal name mismatch: "+err.Error(), entityID), nil
		}
		if errors.Is(err, dag.ErrInstanceNotFound) {
			s.rec.Record("signal", entityID+":"+signalName, false, err)
			return errorResponse(http.StatusNotFound, "instance not found", entityID), nil
		}
		s.rec.Record("signal", entityID+":"+signalName, false, err)
		return errorResponse(http.StatusInternalServerError, "deliver signal failed: "+err.Error(), entityID), nil
	}

	// 推进到终态或下一 WAITING。
	if _, err := s.engine.DrainInstance(ctx, ref); err != nil && !errors.Is(err, dag.ErrInstanceNotFound) {
		s.rec.Record("signal", entityID+":"+signalName, false, err)
		return errorResponse(http.StatusInternalServerError, "drain failed: "+err.Error(), entityID), nil
	}
	inst, err := s.engine.GetInstance(ctx, ref)
	if err != nil {
		s.rec.Record("signal", entityID+":"+signalName, false, err)
		return errorResponse(http.StatusNotFound, "instance not found", entityID), nil
	}

	s.rec.Record("signal", entityID+":"+signalName, inst != nil && isTerminal(inst), nil)
	return responseForInstance(inst), nil
}

// Instances 是 GET /instances/{entityID} 处理器：透传 GetInstance 状态。
func (s *Service) Instances(ctx context.Context, req *rpcserver.Request) (*rpcserver.Response, error) {
	entityID := pathParam(req.Path, "/instances/")
	if entityID == "" {
		return errorResponse(http.StatusBadRequest, "missing entityID in path", ""), nil
	}
	inst, err := s.engine.GetInstance(ctx, &dagv1.EntityRef{EntityId: entityID})
	if err != nil || inst == nil {
		return errorResponse(http.StatusNotFound, "instance not found", entityID), nil
	}
	return responseForInstance(inst), nil
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
		Domain:    "dagsignal",
		Impl:      "memory",
		Healthy:   true,
		RecentOps: s.rec.Snapshot(),
		Extra: map[string]any{
			"graph":         s.graphID,
			"loaded_graphs": s.loadedGraphs,
		},
		CollectedAt: time.Now(),
	}
}

// responseForInstance 是包私有异步映射纯函数：把 *dagv1.EntityInstance.Status
// 映射为 *rpcserver.Response。
//
// | 实例状态 | HTTP | Body |
// |---|---|---|
// | WAITING | 202 | {"entity_id","status":"WAITING"} |
// | COMPLETED | 200 | {"entity_id","status":"COMPLETED"} |
// | FAILED / COMPENSATED / CANCELLED | 500 | 含 error 字段 |
// | RUNNING（signal 后未排空到终态/WAITING） | 202 | {"status":"RUNNING"} |
// | inst == nil | 500 | 含 error |
//
// SHALL NOT 调用 dagbridge.ResponseForTerminalResult（其同步 WAITING→500 语义与异步冲突）。
func responseForInstance(inst *dagv1.EntityInstance) *rpcserver.Response {
	if inst == nil {
		return errorResponse(http.StatusInternalServerError, "nil instance", "")
	}
	type body struct {
		EntityID string `json:"entity_id,omitempty"`
		Status   string `json:"status"`
		Error    string `json:"error,omitempty"`
	}
	statusName := inst.Status.String()
	resp := &rpcserver.Response{
		Header: map[string][]string{"Content-Type": {"application/json"}},
	}
	entityID := ""
	if inst.Ref != nil {
		entityID = inst.Ref.EntityId
	}
	switch inst.Status {
	case dagv1.InstanceStatus_INSTANCE_STATUS_WAITING,
		dagv1.InstanceStatus_INSTANCE_STATUS_RUNNING:
		resp.StatusCode = http.StatusAccepted
		resp.Body, _ = json.Marshal(body{EntityID: entityID, Status: statusName})
	case dagv1.InstanceStatus_INSTANCE_STATUS_COMPLETED:
		resp.StatusCode = http.StatusOK
		resp.Body, _ = json.Marshal(body{EntityID: entityID, Status: statusName})
	case dagv1.InstanceStatus_INSTANCE_STATUS_FAILED,
		dagv1.InstanceStatus_INSTANCE_STATUS_COMPENSATED,
		dagv1.InstanceStatus_INSTANCE_STATUS_CANCELLED:
		resp.StatusCode = http.StatusInternalServerError
		resp.Body, _ = json.Marshal(body{EntityID: entityID, Status: statusName, Error: "terminal failure: " + statusName})
	default:
		resp.StatusCode = http.StatusInternalServerError
		resp.Body, _ = json.Marshal(body{EntityID: entityID, Status: statusName, Error: "unexpected status"})
	}
	return resp
}

// errorResponse 构造 JSON 错误响应。
func errorResponse(code int, msg, entityID string) *rpcserver.Response {
	type body struct {
		EntityID string `json:"entity_id,omitempty"`
		Error    string `json:"error"`
	}
	b, _ := json.Marshal(body{EntityID: entityID, Error: msg})
	return &rpcserver.Response{
		StatusCode: code,
		Header:     map[string][]string{"Content-Type": {"application/json"}},
		Body:       b,
	}
}

// pathParam 从 path 剥离 prefix 取尾部参数（如 /signal/signal-1 → signal-1）。
func pathParam(path, prefix string) string {
	return strings.TrimSpace(strings.TrimPrefix(path, prefix))
}

// queryValue 从 query 取 key，缺省返回 fallback。
func queryValue(query map[string][]string, key, fallback string) string {
	if vs, ok := query[key]; ok && len(vs) > 0 && vs[0] != "" {
		return vs[0]
	}
	return fallback
}

// isWaiting 报告实例是否处于 WAITING。
func isWaiting(inst *dagv1.EntityInstance) bool {
	return inst != nil && inst.Status == dagv1.InstanceStatus_INSTANCE_STATUS_WAITING
}

// isTerminal 报告实例是否处于终态（COMPLETED/FAILED/COMPENSATED/CANCELLED）。
func isTerminal(inst *dagv1.EntityInstance) bool {
	if inst == nil {
		return false
	}
	switch inst.Status {
	case dagv1.InstanceStatus_INSTANCE_STATUS_COMPLETED,
		dagv1.InstanceStatus_INSTANCE_STATUS_FAILED,
		dagv1.InstanceStatus_INSTANCE_STATUS_COMPENSATED,
		dagv1.InstanceStatus_INSTANCE_STATUS_CANCELLED:
		return true
	}
	return false
}
