// SPDX-License-Identifier: AGPL-3.0-or-later

package core

import (
	"strconv"
	"strings"
)

// TaskRef addresses a task by stable identifier or by human ref like "infra-42".
type TaskRef struct {
	ID         string
	ProjectKey string
	Seq        int64
}

// IsID reports whether the ref addresses a task by stable identifier.
func (r TaskRef) IsID() bool { return r.ID != "" }

// String renders the ref in the form it was parsed from.
func (r TaskRef) String() string {
	if r.IsID() {
		return r.ID
	}
	if r.ProjectKey == "" {
		return ""
	}
	return r.ProjectKey + "-" + strconv.FormatInt(r.Seq, 10)
}

// Valid reports whether the ref addresses anything.
func (r TaskRef) Valid() bool {
	return r.IsID() || (r.ProjectKey != "" && r.Seq > 0)
}

// ParseTaskRef parses a task reference.
func ParseTaskRef(s string) (TaskRef, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return TaskRef{}, Invalid("empty task reference")
	}

	if i := strings.LastIndex(s, "-"); i > 0 && i < len(s)-1 {
		key, num := s[:i], s[i+1:]
		if seq, err := strconv.ParseInt(num, 10, 64); err == nil {
			if seq <= 0 {
				return TaskRef{}, Invalid("task reference %q has a non-positive number", s)
			}
			if !validProjectKey(key) {
				return TaskRef{}, Invalid("task reference %q has an invalid project key", s)
			}
			return TaskRef{ProjectKey: strings.ToLower(key), Seq: seq}, nil
		}
	}

	if !validID(s) {
		return TaskRef{}, Invalid("task reference %q is not a valid identifier or project reference", s)
	}
	return TaskRef{ID: s}, nil
}

// MustParseTaskRef parses a reference, panicking on failure.
func MustParseTaskRef(s string) TaskRef {
	r, err := ParseTaskRef(s)
	if err != nil {
		panic(err)
	}
	return r
}

// validProjectKey reports whether s is a usable project key.
func validProjectKey(s string) bool {
	if s == "" || len(s) > 64 {
		return false
	}
	for i, c := range s {
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z':
		case c >= '0' && c <= '9', c == '-', c == '_':
			if i == 0 {
				return false
			}
		default:
			return false
		}
	}
	if last := s[len(s)-1]; last == '-' || last == '_' {
		return false
	}
	return true
}

// ValidateProjectKey returns an error describing why s is not a usable project key.
func ValidateProjectKey(s string) error { return validateKey("project key", s) }

// ValidateWorkflowKey returns an error describing why s is not a usable workflow
// key. A workflow key is held to the same shape a project key is because it is
// spent the same way: every surface puts it in a URL path segment, and a key
// carrying a slash, a query or a fragment produced a location that was no longer
// the route it was built from.
func ValidateWorkflowKey(s string) error { return validateKey("workflow key", s) }

// validateKey is the one shape a key addressable from a URL holds, named by
// whichever kind of key is being rejected.
func validateKey(what, s string) error {
	if !validProjectKey(s) {
		return Invalid("%s %q must start with a letter, end with a letter or digit, and contain only letters, digits, hyphens and underscores", what, s)
	}
	return nil
}

// validID reports whether s looks like a stable identifier.
func validID(s string) bool {
	if len(s) < 8 || len(s) > 64 {
		return false
	}
	for _, c := range s {
		switch {
		case c >= '0' && c <= '9':
		case c >= 'A' && c <= 'Z':
		case c >= 'a' && c <= 'z':
		default:
			return false
		}
	}
	return true
}
