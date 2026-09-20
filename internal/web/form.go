package web

import (
	"mime"
	"strconv"
	"strings"

	"net/http"

	"github.com/thereisnotime/tix/internal/core"
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
