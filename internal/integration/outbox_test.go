package integration

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/service"
	"github.com/heliopsy/tix/internal/webhook"
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
// that body. Nothing here dispatches: the writer runs in server hook mode, so
// it queues the delivery in its own transaction and hands the queue over. The
// only process that can drain it is the running server, through the
// dispatcher `server.Assemble` wires into its workers. Unwiring that
// dispatcher leaves the delivery pending and fails this test.
func TestDirectDatabaseWriteProducesASignedWebhookDelivery(t *testing.T) {
	t.Parallel()
	h := newHarness(t, withDirectHooks(service.HookServer), withDispatchInterval(eventPoll))
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

	start := time.Now()
	task := h.createTaskDirectly("delivered to a webhook")
	event := h.latestEvent()
	if event.SubjectID != task.ID {
		t.Fatalf("newest outbox event names %q, want the task %q", event.SubjectID, task.ID)
	}
	got := receiver.await(t, deliveryWait)
	latency := time.Since(start)

	if got.eventType != string(core.EventTaskCreated) {
		t.Errorf("%s = %q, want %q", webhook.HeaderEvent, got.eventType, core.EventTaskCreated)
	}
	if got.delivery == "" {
		t.Errorf("%s was empty", webhook.HeaderDelivery)
	}
	if _, err := time.Parse(webhook.TimestampLayout, got.timestamp); err != nil {
		t.Errorf("%s %q is not a %s instant: %v", webhook.HeaderTimestamp, got.timestamp, webhook.TimestampLayout, err)
	}
	if !webhook.Verify(endpoint.Secret, got.timestamp, got.body, got.signature) {
		t.Errorf("%s %q does not verify as sha256 hmac over timestamp %q and the body",
			webhook.HeaderSignature, got.signature, got.timestamp)
	}
	if webhook.Verify(endpoint.Secret, got.timestamp, append(got.body, '!'), got.signature) {
		t.Error("the signature verified against a modified body")
	}
	// The timestamp is inside the signed material, so a body captured now
	// cannot be replayed under a later one: the receiver's check fails.
	replayed := webhook.FormatTimestamp(time.Now().Add(time.Hour))
	if webhook.Verify(endpoint.Secret, replayed, got.body, got.signature) {
		t.Error("the signature verified under a different timestamp, so a replay is undetectable")
	}
	if webhook.Verify(endpoint.Secret+"x", got.timestamp, got.body, got.signature) {
		t.Error("the signature verified under a secret the endpoint never issued")
	}

	var delivered core.Event
	if err := json.Unmarshal(got.body, &delivered); err != nil {
		t.Fatalf("decoding the delivered body: %v", err)
	}
	if delivered.Seq != event.Seq || delivered.SubjectID != task.ID {
		t.Errorf("delivered event %d/%q, want the committed %d/%q",
			delivered.Seq, delivered.SubjectID, event.Seq, task.ID)
	}
	if delivered.TenantID != h.tenantID || delivered.Type != core.EventTaskCreated {
		t.Errorf("delivered event %q/%q, want %q/%q",
			delivered.TenantID, delivered.Type, h.tenantID, core.EventTaskCreated)
	}
	t.Logf("direct-database write dispatched by the server's own dispatcher in %s", latency)
}
