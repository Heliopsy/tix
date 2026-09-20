package integration

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/thereisnotime/tix/internal/core"
	"github.com/thereisnotime/tix/internal/webhook"
)

// A command line writing straight to the database knows nothing about any
// running server. The event still reaches a connected WebSocket client,
// because the server is a reader of a persisted outbox rather than the
// producer of an in-memory one. Replacing the outbox with a process-local bus
// fails here.
func TestDirectDatabaseWriteReachesAConnectedWebSocketClient(t *testing.T) {
	t.Parallel()
	h := newHarness(t)

	client := h.dial()
	client.subscribe("live", nil)

	start := time.Now()
	task := h.createTaskDirectly("written by a direct database client")
	event := client.awaitEvent(task.ID, eventWait)
	latency := time.Since(start)

	if event.Type != core.EventTaskCreated {
		t.Errorf("event type = %q, want %q", event.Type, core.EventTaskCreated)
	}
	if event.TenantID != h.tenantID {
		t.Errorf("event tenant = %q, want %q", event.TenantID, h.tenantID)
	}
	if event.Seq <= 0 {
		t.Error("the delivered event carries no sequence, so it was never persisted")
	}
	if persisted := h.latestEvent(); persisted.Seq != event.Seq || persisted.ID != event.ID {
		t.Errorf("delivered event %q at seq %d does not match the outbox row %q at seq %d",
			event.ID, event.Seq, persisted.ID, persisted.Seq)
	}
	t.Logf("direct-database write observed over the websocket in %s", latency)
}

// Every mutation writes its domain rows, its audit entry and its outbox event
// in one transaction, so the row the subscriber is told about is always
// readable by the time the subscriber hears of it.
func TestDeliveredEventMatchesCommittedRows(t *testing.T) {
	t.Parallel()
	h := newHarness(t)

	client := h.dial()
	client.subscribe("live", nil)

	task := h.createTaskDirectly("committed with its event")
	event := client.awaitEvent(task.ID, eventWait)

	got, err := h.direct.GetTask(h.adminCtx, core.TaskRef{ID: event.SubjectID})
	if err != nil {
		t.Fatalf("the event named task %q, which cannot be read: %v", event.SubjectID, err)
	}
	if got.ID != task.ID {
		t.Errorf("event subject = %q, want %q", got.ID, task.ID)
	}

	audits, _, err := h.direct.ListAudit(h.adminCtx, core.AuditFilter{SubjectID: task.ID})
	if err != nil {
		t.Fatalf("listing audit entries: %v", err)
	}
	if len(audits) == 0 {
		t.Errorf("no audit entry names task %q, so the event committed without its audit row", task.ID)
	}
}

// A subscriber that drops off misses nothing: it resumes from its last
// sequence and the durable log replays the gap. An in-memory bus could not
// do this, because the events happened while nobody was listening.
func TestSinceSeqResumesAcrossADisconnect(t *testing.T) {
	t.Parallel()
	h := newHarness(t)

	first := h.dial()
	first.subscribe("live", nil)
	seen := h.createTaskDirectly("seen before the disconnect")
	event := first.awaitEvent(seen.ID, eventWait)
	cursor := event.Seq
	first.close()

	missed := h.createTaskDirectly("written while nobody was connected")

	resumed := h.dial()
	resumed.subscribe("resumed", &cursor)

	replayed := resumed.awaitEvent(missed.ID, eventWait)
	if replayed.Seq <= cursor {
		t.Errorf("replayed event seq = %d, want greater than the cursor %d", replayed.Seq, cursor)
	}
	if replayed.Type != core.EventTaskCreated {
		t.Errorf("replayed event type = %q, want %q", replayed.Type, core.EventTaskCreated)
	}

	live := h.createTaskDirectly("written after the resume")
	if after := resumed.awaitEvent(live.ID, eventWait); after.Seq <= replayed.Seq {
		t.Errorf("live event seq = %d, want greater than the replayed %d", after.Seq, replayed.Seq)
	}
}

// A cursor below the oldest retained event is refused rather than silently
// skipped, which is only answerable because the log is on disk.
func TestSubscribingBelowTheRetainedFloorIsRefused(t *testing.T) {
	t.Parallel()
	h := newHarness(t)

	h.createTaskDirectly("something to retain")
	client := h.dial()
	negative := int64(-1)

	raw, err := json.Marshal(map[string]any{"type": "subscribe", "id": "bad", "since_seq": negative})
	if err != nil {
		t.Fatalf("encoding subscribe: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), dialWait)
	defer cancel()
	if err := client.conn.Write(ctx, websocket.MessageText, raw); err != nil {
		t.Fatalf("sending subscribe: %v", err)
	}
	if m := client.read(dialWait); m.Type != "error" {
		t.Errorf("a negative cursor was answered with %q, want an error", m.Type)
	}
}

// The outbox row a direct-database write commits is what a webhook delivery
// carries, and the signature the receiver verifies covers the timestamp and
// that body. The fan-out and the attempt are both the shipped path: the write
// queues the delivery in its own transaction and, in the default inline hook
// mode, drains it after that transaction commits. The test only watches.
func TestDirectDatabaseWriteProducesASignedWebhookDelivery(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	receiver := newHookRecorder(t)

	endpoint, err := h.direct.PutWebhook(h.adminCtx, core.WebhookInput{
		URL: receiver.server.URL, EventTypes: []string{"task.*"}, Active: true,
	})
	if err != nil {
		t.Fatalf("registering the webhook endpoint: %v", err)
	}
	if endpoint.Secret == "" {
		t.Fatal("registering an endpoint without a secret returned no generated secret")
	}

	task := h.createTaskDirectly("delivered to a webhook")
	event := h.latestEvent()
	if event.SubjectID != task.ID {
		t.Fatalf("newest outbox event names %q, want the task %q", event.SubjectID, task.ID)
	}
	got := receiver.await(t, deliveryWait)
	if got.eventType != string(core.EventTaskCreated) {
		t.Errorf("%s = %q, want %q", webhook.HeaderEvent, got.eventType, core.EventTaskCreated)
	}
	if got.delivery == "" {
		t.Errorf("%s was empty", webhook.HeaderDelivery)
	}
	if !webhook.Verify(endpoint.Secret, got.timestamp, got.body, got.signature) {
		t.Errorf("%s %q does not verify as sha256 hmac over timestamp %q and the body",
			webhook.HeaderSignature, got.signature, got.timestamp)
	}
	if webhook.Verify(endpoint.Secret, got.timestamp, append(got.body, '!'), got.signature) {
		t.Error("the signature verified against a modified body")
	}

	var delivered core.Event
	if err := json.Unmarshal(got.body, &delivered); err != nil {
		t.Fatalf("decoding the delivered body: %v", err)
	}
	if delivered.Seq != event.Seq || delivered.SubjectID != task.ID {
		t.Errorf("delivered event %d/%q, want the committed %d/%q",
			delivered.Seq, delivered.SubjectID, event.Seq, task.ID)
	}
}
