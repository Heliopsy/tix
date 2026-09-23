// SPDX-License-Identifier: AGPL-3.0-or-later

// Package httpapi serves the tix REST API and event stream.
package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/wire"
)

// WriteError renders err as the standard envelope with its mapped status. An
// internal failure is reported generically, so no SQL text, stack trace or
// credential can reach a client.
func WriteError(w http.ResponseWriter, err error) {
	kind := core.KindOf(err)
	body := wire.ErrorBody{Code: kind, Message: err.Error()}

	var domain *core.Error
	if errors.As(err, &domain) {
		body.Message = domain.Message
		body.Details = domain.Details
	}
	if kind == core.KindInternal {
		body = wire.ErrorBody{Code: core.KindInternal, Message: "internal error"}
	}

	w.Header().Set(wire.HeaderContentType, wire.ContentJSON)
	w.WriteHeader(kind.HTTPStatus())
	_ = json.NewEncoder(w).Encode(wire.ErrorEnvelope{Error: body})
}

// WriteJSON renders v with the given status.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set(wire.HeaderContentType, wire.ContentJSON)
	w.WriteHeader(status)
	if v != nil {
		_ = json.NewEncoder(w).Encode(v)
	}
}
