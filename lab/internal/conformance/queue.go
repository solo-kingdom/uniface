package conformance

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/solo-kingdom/uniface/pkg/messaging/queue"
)

// RunQueue 运行基础发布/订阅一致性用例（可选）。
func RunQueue(ctx context.Context, pub queue.Publisher[map[string]any], sub queue.Subscriber[map[string]any]) Result {
	if ctx == nil {
		ctx = context.Background()
	}
	result := Result{}
	topic := fmt.Sprintf("lab.conformance.%d", time.Now().UnixNano())

	var wg sync.WaitGroup
	wg.Add(1)
	received := make(chan map[string]any, 1)

	subCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	_, err := sub.Subscribe(subCtx, topic, func(_ context.Context, msg *queue.Message[map[string]any]) error {
		received <- msg.Value
		wg.Done()
		return nil
	})
	if err != nil {
		result.Failed++
		result.Errors = append(result.Errors, "subscribe: "+err.Error())
		return result
	}

	time.Sleep(200 * time.Millisecond)
	body := map[string]any{"hello": "lab"}
	if err := pub.Publish(ctx, topic, &queue.Message[map[string]any]{Topic: topic, Value: body}); err != nil {
		result.Failed++
		result.Errors = append(result.Errors, "publish: "+err.Error())
		return result
	}

	select {
	case got := <-received:
		if got["hello"] != "lab" {
			result.Failed++
			result.Errors = append(result.Errors, "payload mismatch")
		} else {
			result.Passed++
		}
	case <-time.After(5 * time.Second):
		result.Failed++
		result.Errors = append(result.Errors, "timeout waiting for message")
	}
	wg.Wait()
	return result
}
