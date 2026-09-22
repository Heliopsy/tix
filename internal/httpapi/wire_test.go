package httpapi

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/wire"
)

func TestWriteErrorMapsEveryKindToItsStatus(t *testing.T) {
	tests := []struct {
		err    error
		status int
		code   core.Kind
	}{
		{core.Invalid("bad field"), 400, core.KindInvalid},
		{core.Unauthenticated("no credential"), 401, core.KindUnauthenticated},
		{core.Forbidden("missing scope"), 403, core.KindForbidden},
		{core.NotFound("no task"), 404, core.KindNotFound},
		{core.NoTaskAvailable("queue empty"), 404, core.KindNoTaskAvailable},
		{core.Conflict("claimed"), 409, core.KindConflict},
		{core.LeaseExpired("stale token"), 409, core.KindLeaseExpired},
		{core.Precondition("illegal transition"), 422, core.KindPrecondition},
	}
	for _, tt := range tests {
		t.Run(string(tt.code), func(t *testing.T) {
			rec := httptest.NewRecorder()
			WriteError(rec, tt.err)

			if rec.Code != tt.status {
				t.Errorf("status = %d, want %d", rec.Code, tt.status)
			}
			var got wire.ErrorEnvelope
			if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
				t.Fatalf("decoding envelope: %v", err)
			}
			if got.Error.Code != tt.code {
				t.Errorf("code = %q, want %q", got.Error.Code, tt.code)
			}
			if got.Error.Message == "" {
				t.Error("envelope carries no message")
			}
		})
	}
}

// An internal failure must not leak SQL text, a stack trace, or anything a
// wrapped error happened to carry.
func TestWriteErrorHidesInternalDetail(t *testing.T) {
	leak := core.Internal("select * from tokens where secret = 'hunter2': connection refused")

	rec := httptest.NewRecorder()
	WriteError(rec, leak)

	if rec.Code != 500 {
		t.Errorf("status = %d, want 500", rec.Code)
	}
	body := rec.Body.String()
	for _, forbidden := range []string{"select", "hunter2", "connection refused"} {
		if strings.Contains(strings.ToLower(body), forbidden) {
			t.Errorf("internal error leaked %q:\n%s", forbidden, body)
		}
	}
}

func TestWriteErrorCarriesDetails(t *testing.T) {
	err := core.Invalid("validation failed").WithDetail("field", "title")

	rec := httptest.NewRecorder()
	WriteError(rec, err)

	var got wire.ErrorEnvelope
	if decErr := json.NewDecoder(rec.Body).Decode(&got); decErr != nil {
		t.Fatalf("decoding: %v", decErr)
	}
	if got.Error.Details["field"] != "title" {
		t.Errorf("details = %v, want the offending field", got.Error.Details)
	}
}

// The same service error from two routes must produce the identical code and
// status, which is what lets a client treat them uniformly.
func TestWriteErrorIsStableAcrossCallSites(t *testing.T) {
	first := httptest.NewRecorder()
	second := httptest.NewRecorder()

	WriteError(first, core.Conflict("claimed by another worker"))
	WriteError(second, core.Conflict("claimed by another worker"))

	if first.Code != second.Code || first.Body.String() != second.Body.String() {
		t.Errorf("the same error rendered differently:\n%s\nvs\n%s", first.Body, second.Body)
	}
}

func TestWriteJSON(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteJSON(rec, 201, map[string]string{"id": "t1"})

	if rec.Code != 201 {
		t.Errorf("status = %d, want 201", rec.Code)
	}
	if ct := rec.Header().Get(wire.HeaderContentType); ct != wire.ContentJSON {
		t.Errorf("content type = %q, want %q", ct, wire.ContentJSON)
	}
}

func TestWriteJSONNoBody(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteJSON(rec, 204, nil)

	if rec.Code != 204 {
		t.Errorf("status = %d, want 204", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("204 should have no body, got %q", rec.Body.String())
	}
}
