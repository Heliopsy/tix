package server_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/server"
	"github.com/heliopsy/tix/internal/service"
	"github.com/heliopsy/tix/internal/webhook"
)

// hookPost is one delivery as the receiving end saw it.
type hookPost struct {
	eventType string
	timestamp string
	signature string
	body      []byte
}

// The serve process is what delivers a webhook for a write it never saw. A
// client queues the delivery in its own commit and leaves it there; only the
// server's dispatcher worker turns it into a signed request.
func TestAssembledServerDeliversQueuedWebhooks(t *testing.T) {
	posts := make(chan hookPost, 4)
	receiver := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		posts <- hookPost{
			eventType: r.Header.Get(webhook.HeaderEvent),
			timestamp: r.Header.Get(webhook.HeaderTimestamp),
			signature: r.Header.Get(webhook.HeaderSignature),
			body:      body,
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer receiver.Close()

	a := assemble(t, func(o *server.Options) { o.DispatchInterval = 10 * time.Millisecond })
	if err := a.srv.Listen(); err != nil {
		t.Fatalf("listening: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- a.srv.Serve(ctx) }()
	defer func() {
		cancel()
		if err := <-done; err != nil {
			t.Fatalf("serving: %v", err)
		}
	}()

	writer := service.New(a.store, service.WithHooks(service.HookServer))
	writeCtx := a.conn.Context(context.Background())
	endpoint, err := writer.PutWebhook(writeCtx, core.WebhookInput{
		URL: receiver.URL, EventTypes: []string{"task.*"}, Active: true,
	})
	if err != nil {
		t.Fatalf("registering the endpoint: %v", err)
	}
	task, err := writer.CreateTask(writeCtx, core.CreateTaskInput{Title: "delivered by the server"})
	if err != nil {
		t.Fatalf("creating the task: %v", err)
	}

	select {
	case got := <-posts:
		if got.eventType != string(core.EventTaskCreated) {
			t.Errorf("%s = %q, want %q", webhook.HeaderEvent, got.eventType, core.EventTaskCreated)
		}
		if !webhook.Verify(endpoint.Secret, got.timestamp, got.body, got.signature) {
			t.Errorf("%s %q does not verify over timestamp %q and the body",
				webhook.HeaderSignature, got.signature, got.timestamp)
		}
		var delivered core.Event
		if err := json.Unmarshal(got.body, &delivered); err != nil {
			t.Fatalf("decoding the delivered body: %v", err)
		}
		if delivered.SubjectID != task.ID {
			t.Errorf("delivered event names %q, want the task %q", delivered.SubjectID, task.ID)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("the server never delivered the queued webhook")
	}
}

func TestAssembleHonoursADisabledDispatcher(t *testing.T) {
	a := assemble(t, func(o *server.Options) { o.DisableDispatch = true })
	if err := a.srv.Listen(); err != nil {
		t.Fatalf("listening: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- a.srv.Serve(ctx) }()
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("serving: %v", err)
	}
}
