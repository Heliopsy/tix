// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"reflect"
	"testing"

	"github.com/heliopsy/tix/internal/output"
)

// TestHandlersShareOneParsedTemplateSet pins the fix for the web suite needing
// 18 GB: every handler parsed its own copy of every template, and a parallel
// test suite holds a handler per test.
func TestHandlersShareOneParsedTemplateSet(t *testing.T) {
	t.Parallel()
	style, err := output.NewTimeStyle("", "utc")
	if err != nil {
		t.Fatal(err)
	}
	a, b := defaultTemplatesFor(style), defaultTemplatesFor(style)
	if reflect.ValueOf(a).Pointer() != reflect.ValueOf(b).Pointer() {
		t.Fatal("two handlers with one style parsed two template sets")
	}
	other, err := output.NewTimeStyle("", "Europe/Sofia")
	if err != nil {
		t.Fatal(err)
	}
	if reflect.ValueOf(defaultTemplatesFor(other)).Pointer() == reflect.ValueOf(a).Pointer() {
		t.Fatal("a handler in another zone was given the first zone's templates")
	}
}
