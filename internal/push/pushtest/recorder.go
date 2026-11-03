// Package pushtest provides a recording push.Sender for tests.
package pushtest

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"homebase/internal/push"
)

// Sent is one Push the Recorder was asked to send.
type Sent struct {
	Subscription push.Subscription
	Payload      push.Payload
}

// Recorder records every Push instead of sending it. Endpoints marked with
// Gone are answered as gone (like a 404/410 from a push service). It is safe
// for concurrent use.
type Recorder struct {
	mu   sync.Mutex
	sent []Sent
	gone map[string]bool
}

func NewRecorder() *Recorder {
	return &Recorder{gone: map[string]bool{}}
}

// Gone makes the Recorder report these endpoints as no longer existing.
func (r *Recorder) Gone(endpoints ...string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, e := range endpoints {
		r.gone[e] = true
	}
}

func (r *Recorder) Send(_ context.Context, sub push.Subscription, payload []byte) (push.Result, error) {
	var p push.Payload
	if err := json.Unmarshal(payload, &p); err != nil {
		return push.Failed, fmt.Errorf("payload is not a push.Payload: %w", err)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.gone[sub.Endpoint] {
		return push.Gone, nil
	}
	r.sent = append(r.sent, Sent{Subscription: sub, Payload: p})
	return push.Delivered, nil
}

// Sent returns every Push accepted so far, in order.
func (r *Recorder) Sent() []Sent {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]Sent(nil), r.sent...)
}
