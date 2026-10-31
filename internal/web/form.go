// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"html/template"
	"mime"
	"strconv"
	"strings"

	"net/http"

	"github.com/heliopsy/tix/internal/core"
)

// maxUploadBytes bounds the part of a multipart submission kept in memory.
const maxUploadBytes = 1 << 20

// ensureForm parses a submission, whether it is url encoded or multipart.
//
// The body is bounded here as well as by the server's limit middleware, so the
// property holds even if this handler is mounted somewhere that lacks it.
func ensureForm(r *http.Request) error {
	r.Body = http.MaxBytesReader(nil, r.Body, maxUploadBytes)
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err == nil && media == "multipart/form-data" {
		if err := r.ParseMultipartForm(maxUploadBytes); err != nil { // #nosec G120 -- bounded above and by the limit
			return core.Invalid("this submission could not be read: %v", err)
		}
		return nil
	}
	if err := r.ParseForm(); err != nil { // #nosec G120 -- body bounded by MaxBytesReader above
		return core.Invalid("this submission could not be read")
	}
	return nil
}

// field returns one trimmed form value.
func field(r *http.Request, name string) string {
	return strings.TrimSpace(r.PostFormValue(name))
}

// prose returns one trimmed multi-line form value with its line endings
// normalised to \n.
//
// A browser submits a textarea with CRLF line breaks, which the HTML form
// specification requires, so every save through the browser rewrote the line
// endings of text the CLI and the API had stored with \n. Nothing displayed
// differently and nothing failed, which is what made it worth fixing: a no-op
// save silently changed the bytes, an export round-tripped to a different
// document, and a diff between two stores showed every multi-line field as
// modified.
//
// Use this for anything a person types more than one line into. field is
// still right for a name, a key or a select.
func prose(r *http.Request, name string) string {
	v := r.PostFormValue(name)
	v = strings.ReplaceAll(v, "\r\n", "\n")
	v = strings.ReplaceAll(v, "\r", "\n")
	return strings.TrimSpace(v)
}

// sent returns one trimmed form value and whether the form carried the key at
// all. An absent key and an empty one are different requests: a form that
// names a field is asking for the value it submitted, empty included, while a
// form that never mentions it is asking for no change.
func sent(r *http.Request, name string) (string, bool) {
	_ = r.PostFormValue(name)
	values, ok := r.PostForm[name]
	if !ok || len(values) == 0 {
		return "", false
	}
	return strings.TrimSpace(values[0]), true
}

// checked reports whether a checkbox was ticked.
func checked(r *http.Request, name string) bool {
	switch strings.ToLower(field(r, name)) {
	case "1", "true", "on", "yes":
		return true
	default:
		return false
	}
}

// fieldList splits a comma separated form value.
func fieldList(r *http.Request, name string) []string {
	return splitList(field(r, name))
}

// splitList splits a comma separated value, discarding blanks.
func splitList(raw string) []string {
	var out []string
	for _, part := range strings.Split(raw, ",") {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

// lines splits a textarea into its non-empty lines.
func lines(raw string) []string {
	var out []string
	for _, line := range strings.Split(raw, "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

// pairs reads "key=value" lines into a map.
func pairs(raw string) map[string]any {
	out := map[string]any{}
	for _, line := range lines(raw) {
		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		out[strings.TrimSpace(key)] = strings.TrimSpace(value)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// priorityField reads an optional priority form value.
func priorityField(r *http.Request, name string) (core.Priority, error) {
	raw := field(r, name)
	if raw == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, core.Invalid("priority %q must be a number from 1 to 5", raw)
	}
	p := core.Priority(n)
	if !p.Valid() {
		return 0, core.Invalid("priority %d is out of range", n)
	}
	return p, nil
}

// intField reads an optional whole number form value.
func intField(r *http.Request, name string) (int, error) {
	raw := field(r, name)
	if raw == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, core.Invalid("%s %q must be a whole number", name, raw)
	}
	return n, nil
}

// fieldErrors names, control by control, why a submission was refused, so a
// screen can re-render itself with each message beside the field it is about
// instead of replacing the page with an error screen that loses everything the
// reader typed. A refusal naming no field is recorded under the empty key and
// belongs above the form.
//
// The attribution comes from the refusal itself (core.FieldOf), not from
// matching message text here, so the rule and the message stay in one place:
// internal/service and internal/core.
type fieldErrors map[string]string

// On returns one control's refusal as the "field-error" partial renders it,
// and nil when that control was not what the refusal was about. The empty name
// is the submission as a whole.
func (e fieldErrors) On(name string) *fieldError {
	message, ok := e[name]
	if !ok {
		return nil
	}
	return &fieldError{Field: name, Message: message}
}

// Attrs marks one control as refused and points it at its message, and is
// empty when the control was not what the refusal was about. The field names
// are the handlers' own constants, never anything a reader typed.
func (e fieldErrors) Attrs(name string) template.HTMLAttr {
	f := e.On(name)
	if f == nil {
		return ""
	}
	return template.HTMLAttr(` aria-invalid="true" aria-describedby="` + template.HTMLEscapeString(f.ID()) + `"`) // #nosec G203 -- built from a constant field name, escaped
}

// fieldError is one refusal as a template renders it.
type fieldError struct {
	Field   string
	Message string
}

// ID is what the control's aria-describedby points at.
func (f fieldError) ID() string { return f.Field + "-error" }

// refusal records err against the field it names.
func refusal(err error) fieldErrors {
	message := safeMessage(err, core.KindOf(err))
	if core.KindOf(err) == core.KindNotFound && !strings.HasPrefix(message, "no ") {
		message = "no such " + message
	}
	return fieldErrors{core.FieldOf(err): message}
}

// correctable reports whether a refusal is one the reader can act on by
// changing what they submitted. Those are re-rendered in place; anything else
// -- a lost session, a scope they do not hold, a store that failed -- is not
// about the form and goes to the error screen, which is the only thing that
// can explain it.
func correctable(err error) bool {
	switch core.KindOf(err) {
	case core.KindInvalid, core.KindConflict, core.KindPrecondition:
		return true
	default:
		return false
	}
}
