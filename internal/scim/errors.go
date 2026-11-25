// SPDX-License-Identifier: AGPL-3.0-or-later

package scim

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/wire"
)

// Schema URNs from RFC 7643 and RFC 7644.
const (
	SchemaUser         = "urn:ietf:params:scim:schemas:core:2.0:User"
	SchemaGroup        = "urn:ietf:params:scim:schemas:core:2.0:Group"
	SchemaEnterprise   = "urn:ietf:params:scim:schemas:extension:enterprise:2.0:User"
	SchemaListResponse = "urn:ietf:params:scim:api:messages:2.0:ListResponse"
	SchemaPatchOp      = "urn:ietf:params:scim:api:messages:2.0:PatchOp"
	SchemaError        = "urn:ietf:params:scim:api:messages:2.0:Error"
	SchemaSPConfig     = "urn:ietf:params:scim:schemas:core:2.0:ServiceProviderConfig"
	SchemaResourceType = "urn:ietf:params:scim:schemas:core:2.0:ResourceType"
	SchemaSchema       = "urn:ietf:params:scim:schemas:core:2.0:Schema"
)

// SCIM error types a 400 or 409 names in scimType.
const (
	TypeInvalidFilter = "invalidFilter"
	TypeInvalidSyntax = "invalidSyntax"
	TypeInvalidPath   = "invalidPath"
	TypeInvalidValue  = "invalidValue"
	TypeUniqueness    = "uniqueness"
	TypeMutability    = "mutability"
)

// Error is the body of every SCIM error response.
type Error struct {
	Schemas  []string `json:"schemas"`
	Status   string   `json:"status"`
	SCIMType string   `json:"scimType,omitempty"`
	Detail   string   `json:"detail,omitempty"`
}

// protoError is a refusal the protocol layer itself decides, with its type.
type protoError struct {
	status   int
	scimType string
	detail   string
}

func (e *protoError) Error() string { return e.detail }

// badRequest refuses a request with a 400 of the given type.
func badRequest(scimType, detail string) error {
	return &protoError{status: http.StatusBadRequest, scimType: scimType, detail: detail}
}

// writeJSON answers with a SCIM document.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set(wire.HeaderContentType, wire.ContentSCIM)
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
}

// writeError answers with the SCIM error schema at the status the error
// calls for.
func writeError(w http.ResponseWriter, err error) {
	status, scimType, detail := classify(err)
	writeJSON(w, status, Error{Schemas: []string{SchemaError}, Status: strconv.Itoa(status),
		SCIMType: scimType, Detail: detail})
}

// classify maps an error to a status, a SCIM type and the detail shown.
func classify(err error) (int, string, string) {
	var pe *protoError
	if errors.As(err, &pe) {
		return pe.status, pe.scimType, pe.detail
	}
	var ce *core.Error
	if !errors.As(err, &ce) {
		return http.StatusInternalServerError, "", "internal error"
	}
	switch ce.Kind {
	case core.KindInvalid:
		return http.StatusBadRequest, TypeInvalidValue, ce.Message
	case core.KindNotFound:
		return http.StatusNotFound, "", ce.Message
	case core.KindConflict:
		return http.StatusConflict, TypeUniqueness, ce.Message
	case core.KindForbidden:
		return http.StatusForbidden, "", ce.Message
	case core.KindUnauthenticated:
		return http.StatusUnauthorized, "", ce.Message
	case core.KindPrecondition:
		return http.StatusPreconditionFailed, "", ce.Message
	}
	return http.StatusInternalServerError, "", "internal error"
}
