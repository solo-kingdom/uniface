package units

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	dagv1 "github.com/solo-kingdom/uniface/api/dag/v1"
	"github.com/solo-kingdom/uniface/pkg/dag/entity"
	"github.com/solo-kingdom/uniface/pkg/dag/testpb"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/anypb"
)

const orderTypeURL = "type.googleapis.com/dag.testpb.Order"

func orderSnapshot(order *testpb.Order) *dagv1.EntitySnapshot {
	a, _ := anypb.New(order)
	return &dagv1.EntitySnapshot{
		Ref:      &dagv1.EntityRef{EntityId: "e1"},
		TypeKey:  &dagv1.EntityTypeKey{EntityType: "order.Order", PayloadSchemaVersion: "v1"},
		Sequence: 1,
		Payload:  a,
	}
}

// ---- body 构造（Level 0 / Level 1 / 字段缺失）----

func TestBuildBody_Level0WholePayload(t *testing.T) {
	u := NewHttpUnit(&dagv1.HttpUnit{}, nil)
	snap := orderSnapshot(&testpb.Order{OrderId: "o1", Amount: 100, Status: "new", Approved: true})
	body, err := u.buildBody(snap)
	if err != nil {
		t.Fatalf("buildBody: %v", err)
	}
	// protojson 默认使用 camelCase。
	if !bytes.Contains(body, []byte(`"orderId":"o1"`)) {
		t.Fatalf("expected orderId in body, got %s", body)
	}
	if !bytes.Contains(body, []byte(`"amount":100`)) {
		t.Fatalf("expected amount 100 in body, got %s", body)
	}
}

func TestBuildBody_Level1FieldPathMessage(t *testing.T) {
	wrapper := &testpb.Wrapper{Order: &testpb.Order{OrderId: "o1", Amount: 50}, Metadata: "meta"}
	a, _ := anypb.New(wrapper)
	snap := &dagv1.EntitySnapshot{Ref: &dagv1.EntityRef{EntityId: "e1"}, Sequence: 1, Payload: a}
	u := NewHttpUnit(&dagv1.HttpUnit{RequestBody: &dagv1.BodyTemplate{FieldPath: "Order"}}, nil)
	body, err := u.buildBody(snap)
	if err != nil {
		t.Fatalf("buildBody: %v", err)
	}
	if !bytes.Contains(body, []byte(`"orderId":"o1"`)) {
		t.Fatalf("expected nested order in body, got %s", body)
	}
	if bytes.Contains(body, []byte(`"metadata"`)) {
		t.Fatalf("Level 1 should not include sibling fields, got %s", body)
	}
}

func TestBuildBody_Level1FieldMissing(t *testing.T) {
	u := NewHttpUnit(&dagv1.HttpUnit{RequestBody: &dagv1.BodyTemplate{FieldPath: "missing"}}, nil)
	_, err := u.buildBody(orderSnapshot(&testpb.Order{OrderId: "o1"}))
	if err == nil {
		t.Fatal("expected error for missing field path")
	}
}

func TestBuildBody_NilPayload(t *testing.T) {
	u := NewHttpUnit(&dagv1.HttpUnit{}, nil)
	body, err := u.buildBody(&dagv1.EntitySnapshot{Ref: &dagv1.EntityRef{EntityId: "e1"}})
	if err != nil {
		t.Fatalf("buildBody: %v", err)
	}
	if body != nil {
		t.Fatalf("expected nil body for nil payload, got %s", body)
	}
}

// ---- 状态码分类 ----

func TestClassify_2xxRoutesToResponseMapping(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"orderId":"o1","amount":100,"status":"charged","approved":true}`))
	}))
	defer srv.Close()
	u := NewHttpUnit(&dagv1.HttpUnit{Url: srv.URL, Path: "/charge"}, nil)
	mut, err := u.Execute(context.Background(), orderSnapshot(&testpb.Order{OrderId: "o1"}))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	upd, ok := mut.GetIntent().(*dagv1.EntityMutation_Update)
	if !ok {
		t.Fatalf("expected update mutation, got %T", mut.GetIntent())
	}
	if upd.Update.Payload.GetTypeUrl() != orderTypeURL {
		t.Fatalf("expected payload type %q, got %q", orderTypeURL, upd.Update.Payload.GetTypeUrl())
	}
}

func TestClassify_503ReturnsRetryableError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	u := NewHttpUnit(&dagv1.HttpUnit{Url: srv.URL}, nil)
	_, err := u.Execute(context.Background(), orderSnapshot(&testpb.Order{}))
	if err == nil || !IsRetryableError(err) {
		t.Fatalf("expected retryable error, got %v", err)
	}
}

func TestClassify_404ProducesFailMutation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	u := NewHttpUnit(&dagv1.HttpUnit{Url: srv.URL}, nil)
	mut, err := u.Execute(context.Background(), orderSnapshot(&testpb.Order{}))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	fail, ok := mut.GetIntent().(*dagv1.EntityMutation_Fail)
	if !ok {
		t.Fatalf("expected fail mutation, got %T", mut.GetIntent())
	}
	if !strings.Contains(fail.Fail.GetReason(), "404") {
		t.Fatalf("expected reason contains 404, got %q", fail.Fail.GetReason())
	}
	if fail.Fail.GetTriggerCompensation() {
		t.Fatal("fail should not trigger compensation by default")
	}
}

func TestClassify_500Unclassified5xxFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	u := NewHttpUnit(&dagv1.HttpUnit{Url: srv.URL}, nil)
	mut, err := u.Execute(context.Background(), orderSnapshot(&testpb.Order{}))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if _, ok := mut.GetIntent().(*dagv1.EntityMutation_Fail); !ok {
		t.Fatalf("unclassified 5xx should fail, got %T", mut.GetIntent())
	}
}

func TestClassify_CustomRetryCodes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(429)
	}))
	defer srv.Close()
	u := NewHttpUnit(&dagv1.HttpUnit{
		Url: srv.URL,
		RetryOn: &dagv1.RetryClassification{RetryStatusCodes: []int32{429}},
	}, nil)
	_, err := u.Execute(context.Background(), orderSnapshot(&testpb.Order{}))
	if err == nil || !IsRetryableError(err) {
		t.Fatalf("429 in custom retry codes should be retryable, got %v", err)
	}
}

// ---- response 映射（AUTO / MUTATION / 反序列化失败）----

func TestMapResponse_ModeMutation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"complete":2}`)) // TERMINAL_OUTCOME_FAILURE = 2
	}))
	defer srv.Close()
	u := NewHttpUnit(&dagv1.HttpUnit{
		Url: srv.URL,
		Response: &dagv1.ResponseMapping{Mode: dagv1.ResponseMapping_MODE_MUTATION},
	}, nil)
	mut, err := u.Execute(context.Background(), orderSnapshot(&testpb.Order{}))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if _, ok := mut.GetIntent().(*dagv1.EntityMutation_Complete); !ok {
		t.Fatalf("MODE_MUTATION with complete intent expected, got %T", mut.GetIntent())
	}
}

func TestMapResponse_DecodeFailureProducesFail(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{not valid json`))
	}))
	defer srv.Close()
	u := NewHttpUnit(&dagv1.HttpUnit{Url: srv.URL}, nil)
	mut, err := u.Execute(context.Background(), orderSnapshot(&testpb.Order{}))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	fail, ok := mut.GetIntent().(*dagv1.EntityMutation_Fail)
	if !ok {
		t.Fatalf("expected fail on decode error, got %T", mut.GetIntent())
	}
	if !strings.Contains(fail.Fail.GetReason(), "response decode failed") {
		t.Fatalf("expected decode failed reason, got %q", fail.Fail.GetReason())
	}
}

func TestMapResponse_PayloadFieldProjection(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"order":{"orderId":"p1","amount":5},"metadata":"x"}`))
	}))
	defer srv.Close()
	u := NewHttpUnit(&dagv1.HttpUnit{
		Url: srv.URL,
		Response: &dagv1.ResponseMapping{
			PayloadTypeUrl: "type.googleapis.com/dag.testpb.Wrapper",
			PayloadField:   "Order",
		},
	}, nil)
	mut, err := u.Execute(context.Background(), orderSnapshot(&testpb.Order{}))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	upd, ok := mut.GetIntent().(*dagv1.EntityMutation_Update)
	if !ok {
		t.Fatalf("expected update, got %T", mut.GetIntent())
	}
	if upd.Update.Payload.GetTypeUrl() != orderTypeURL {
		t.Fatalf("projected payload should be Order, got %q", upd.Update.Payload.GetTypeUrl())
	}
}

func TestMapResponse_OnSuccessComplete(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"orderId":"o1"}`))
	}))
	defer srv.Close()
	u := NewHttpUnit(&dagv1.HttpUnit{
		Url: srv.URL,
		Response: &dagv1.ResponseMapping{OnSuccess: dagv1.TerminalOutcome_TERMINAL_OUTCOME_SUCCESS},
	}, nil)
	mut, err := u.Execute(context.Background(), orderSnapshot(&testpb.Order{}))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	comp, ok := mut.GetIntent().(*dagv1.EntityMutation_Complete)
	if !ok || comp.Complete != dagv1.TerminalOutcome_TERMINAL_OUTCOME_SUCCESS {
		t.Fatalf("expected complete success, got %T = %v", mut.GetIntent(), mut.GetIntent())
	}
}

// ---- resolver 集成 ----

type fakeResolver struct {
	client  *http.Client
	baseURL string
	err     error
}

func (f *fakeResolver) ResolveClient(ctx context.Context, service string) (*http.Client, string, error) {
	return f.client, f.baseURL, f.err
}

func TestExecute_ServiceViaResolver(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/charge" {
			t.Errorf("expected path /charge, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = io.Copy(w, r.Body) // 回显 body 用于断言
	}))
	defer srv.Close()

	resolver := &fakeResolver{client: srv.Client(), baseURL: srv.URL}
	u := NewHttpUnit(&dagv1.HttpUnit{Service: "order-service", Path: "/charge", Method: "POST"}, resolver)
	mut, err := u.Execute(context.Background(), orderSnapshot(&testpb.Order{OrderId: "o1"}))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if _, ok := mut.GetIntent().(*dagv1.EntityMutation_Update); !ok {
		t.Fatalf("expected update, got %T", mut.GetIntent())
	}
}

func TestExecute_ServiceWithoutResolverErrors(t *testing.T) {
	u := NewHttpUnit(&dagv1.HttpUnit{Service: "order-service"}, nil)
	_, err := u.Execute(context.Background(), orderSnapshot(&testpb.Order{}))
	if err == nil {
		t.Fatal("expected error when service set but no resolver")
	}
	// 配置错误，不应归类为可重试。
	if IsRetryableError(err) {
		t.Fatal("missing resolver is a config error, not retryable")
	}
}

func TestExecute_ResolverErrorIsRetryable(t *testing.T) {
	resolver := &fakeResolver{err: errors.New("no instance available")}
	u := NewHttpUnit(&dagv1.HttpUnit{Service: "order-service"}, resolver)
	_, err := u.Execute(context.Background(), orderSnapshot(&testpb.Order{}))
	if err == nil || !IsRetryableError(err) {
		t.Fatalf("resolver error should be retryable, got %v", err)
	}
}

func TestExecute_BodyLevel0SentToServer(t *testing.T) {
	var received []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"orderId":"o1","amount":100,"status":"ok","approved":true}`))
	}))
	defer srv.Close()
	u := NewHttpUnit(&dagv1.HttpUnit{Url: srv.URL}, nil)
	_, err := u.Execute(context.Background(), orderSnapshot(&testpb.Order{OrderId: "o1", Amount: 100}))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !bytes.Contains(received, []byte(`"orderId":"o1"`)) {
		t.Fatalf("server should receive payload body, got %s", received)
	}
}

func TestExecute_HeadersApplied(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"orderId":"o1","amount":1,"status":"x","approved":true}`))
	}))
	defer srv.Close()
	u := NewHttpUnit(&dagv1.HttpUnit{
		Url:     srv.URL,
		Headers: map[string]string{"Authorization": "Bearer t"},
	}, nil)
	_, err := u.Execute(context.Background(), orderSnapshot(&testpb.Order{}))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if gotAuth != "Bearer t" {
		t.Fatalf("expected Authorization header applied, got %q", gotAuth)
	}
}

func TestJoinURL(t *testing.T) {
	cases := []struct{ base, path, want string }{
		{"http://h:8080", "/charge", "http://h:8080/charge"},
		{"http://h:8080/", "/charge", "http://h:8080/charge"},
		{"http://h:8080", "charge", "http://h:8080/charge"},
		{"http://h:8080/api", "/charge", "http://h:8080/api/charge"},
		{"http://h:8080/api/", "/charge", "http://h:8080/api/charge"},
		{"http://h:8080", "", "http://h:8080"},
	}
	for _, c := range cases {
		if got := joinURL(c.base, c.path); got != c.want {
			t.Errorf("joinURL(%q,%q)=%q want %q", c.base, c.path, got, c.want)
		}
	}
}

// 编译期断言 entity.NewSnapshot 可用（避免未使用 import）。
var _ = entity.NewSnapshot

// 确保 protojson 在测试中被引用（用于额外断言时）。
var _ = protojson.MarshalOptions{}
