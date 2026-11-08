// SPDX-License-Identifier: AGPL-3.0-or-later

package httpapi

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/service"
	"github.com/heliopsy/tix/internal/wire"
)

// Idempotency reserves and settles the keys clients send to retry a write.
// The router finds it on its service when Config leaves it unset; without it
// the Idempotency-Key header is ignored.
type Idempotency interface {
	BeginIdempotent(ctx context.Context, in service.IdempotentRequest) (*service.IdempotentStart, error)
	FinishIdempotent(ctx context.Context, c *service.IdempotencyClaim, resp service.IdempotentResponse) error
}

// finishTimeout bounds keeping a response once the handler has answered. It
// runs detached from the request, so a client hanging up or the request
// deadline passing does not lose an answer that is already decided.
const finishTimeout = 5 * time.Second

// isMutating reports whether a method changes something.
func isMutating(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	}
	return false
}

// takesIdempotencyKey reports whether a request is one the Idempotency-Key
// header applies to: an authenticated write under the API prefix.
func takesIdempotencyKey(r *http.Request) bool {
	if !isMutating(r.Method) || !strings.HasPrefix(r.URL.Path, wire.APIPrefix) || IsPublicPath(r.URL.Path) {
		return false
	}
	_, ok := core.ActorFrom(r.Context())
	return ok
}

// idempotent answers a repeated write from the response its first attempt
// kept, refuses a key reused for another request or still running, and
// otherwise runs the request with the key reserved and keeps its answer.
func (rt *Router) idempotent(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		keys := r.Header.Values(wire.HeaderIdempotencyKey)
		if len(keys) == 0 || rt.cfg.Idempotency == nil || !takesIdempotencyKey(r) {
			next.ServeHTTP(w, r)
			return
		}
		if len(keys) > 1 {
			WriteError(w, core.Invalid("send one %s header", wire.HeaderIdempotencyKey))
			return
		}
		if err := service.ValidateIdempotencyKey(keys[0]); err != nil {
			WriteError(w, err)
			return
		}
		body, ok := readWholeBody(w, r)
		if !ok {
			return
		}
		start, err := rt.cfg.Idempotency.BeginIdempotent(r.Context(), service.IdempotentRequest{
			Key:         keys[0],
			Fingerprint: fingerprint(r, body),
			Lease:       2 * rt.cfg.RequestTimeout,
		})
		if err != nil {
			WriteError(w, err)
			return
		}
		switch start.Outcome {
		case service.IdempotentReplay:
			replay(w, start.Response)
		case service.IdempotentMismatch:
			writeStatusError(w, http.StatusUnprocessableEntity, core.KindInvalid,
				"idempotency key reused with a different request")
		case service.IdempotentInFlight:
			w.Header().Set(wire.HeaderRetryAfter, strconv.Itoa(retrySeconds(start.RetryAfter)))
			WriteError(w, core.Conflict("a request with this idempotency key is still running"))
		case service.IdempotentLost:
			WriteError(w, core.Conflict("the request with this idempotency key was applied but its response was not kept"))
		default:
			rt.runIdempotent(w, r, body, start.Claim, next)
		}
	})
}

// runIdempotent runs the request with its claim on the context, then keeps
// or releases the key according to what it answered.
func (rt *Router) runIdempotent(w http.ResponseWriter, r *http.Request, body []byte, claim *service.IdempotencyClaim, next http.Handler) {
	r.Body = io.NopCloser(bytes.NewReader(body))
	tee := &teeWriter{ResponseWriter: w}
	next.ServeHTTP(tee, r.WithContext(service.WithIdempotencyClaim(r.Context(), claim)))
	if !tee.written {
		tee.status = http.StatusOK
	}
	resp := service.IdempotentResponse{Status: tee.status, ContentType: tee.contentType, TooLarge: tee.overflow}
	if !tee.overflow {
		resp.Body = tee.buf.Bytes()
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), finishTimeout)
	defer cancel()
	if err := rt.cfg.Idempotency.FinishIdempotent(ctx, claim, resp); err != nil {
		rt.cfg.Logger.Error("keeping an idempotent response failed", "error", err.Error(),
			"request_id", RequestIDFrom(r.Context()))
	}
}

// readWholeBody reads the request body so it can be fingerprinted and then
// handed to the handler unchanged.
func readWholeBody(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	if r.Body == nil {
		return nil, true
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeStatusError(w, http.StatusRequestEntityTooLarge, core.KindInvalid, tooLargeMessage(r))
			return nil, false
		}
		WriteError(w, core.Invalid("reading the request body: %v", err))
		return nil, false
	}
	return body, true
}

// fingerprint identifies a request by its method, request URI and body.
func fingerprint(r *http.Request, body []byte) string {
	h := sha256.New()
	_, _ = io.WriteString(h, r.Method+"\n"+r.URL.RequestURI()+"\n")
	_, _ = h.Write(body)
	return hex.EncodeToString(h.Sum(nil))
}

// retrySeconds renders a wait as whole seconds, at least one.
func retrySeconds(d time.Duration) int {
	return max(1, int(math.Ceil(d.Seconds())))
}

// replay writes a kept response.
func replay(w http.ResponseWriter, resp *service.IdempotentResponse) {
	if resp.ContentType != "" {
		w.Header().Set(wire.HeaderContentType, resp.ContentType)
	}
	w.Header().Set(wire.HeaderIdempotentReplayed, "true")
	w.WriteHeader(resp.Status)
	_, _ = w.Write(resp.Body)
}

// teeWriter passes a response through and keeps a copy of it, up to the
// largest body that is kept for replay.
type teeWriter struct {
	http.ResponseWriter
	status      int
	contentType string
	written     bool
	overflow    bool
	buf         bytes.Buffer
}

func (t *teeWriter) WriteHeader(status int) {
	if t.written {
		return
	}
	t.written = true
	t.status = status
	t.contentType = t.Header().Get(wire.HeaderContentType)
	t.ResponseWriter.WriteHeader(status)
}

func (t *teeWriter) Write(b []byte) (int, error) {
	if !t.written {
		t.WriteHeader(http.StatusOK)
	}
	if !t.overflow {
		if t.buf.Len()+len(b) > service.MaxIdempotentBody {
			t.overflow = true
			t.buf = bytes.Buffer{}
		} else {
			t.buf.Write(b)
		}
	}
	return t.ResponseWriter.Write(b)
}

// Unwrap exposes the underlying writer to http.ResponseController.
func (t *teeWriter) Unwrap() http.ResponseWriter { return t.ResponseWriter }
