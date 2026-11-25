// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"slices"
	"testing"

	"github.com/heliopsy/tix/internal/docsmd"
)

// The key table in docs/web-ui.md transcribes shortcuts.go, the kind of list
// that rots, so it is read back and compared: every action is a row, every
// row is an action, and each scheme's cell names exactly the keys it binds.
func TestTheKeyTableIsDocumented(t *testing.T) {
	t.Parallel()
	md, err := docsmd.Read("docs/web-ui.md")
	if err != nil {
		t.Fatalf("reading docs/web-ui.md: %v", err)
	}
	rows, err := docsmd.Table(md, "Action", "Default", "Vim", "Emacs", "Off")
	if err != nil {
		t.Fatalf("reading the key table: %v", err)
	}
	schemes := []KeyScheme{KeySchemeDefault, KeySchemeVim, KeySchemeEmacs, KeySchemeOff}
	byLabel := map[string][]string{}
	for _, row := range rows {
		byLabel[row[0]] = row
	}
	for _, a := range shortcutActions {
		row, ok := byLabel[a.Label]
		if !ok {
			t.Errorf("the key table has no row for %q", a.Label)
			continue
		}
		delete(byLabel, a.Label)
		for i, s := range schemes {
			if i+1 >= len(row) {
				t.Errorf("the %q row has no %s cell", a.Label, s)
				continue
			}
			got := docsmd.Codes(row[i+1])
			want := KeyBindingsFor(s)[a.Name]
			if !slices.Equal(got, want) {
				t.Errorf("the key table says %s binds %q to %v, the code binds %v", s, a.Label, got, want)
			}
		}
	}
	for label := range byLabel {
		t.Errorf("the key table has a row for %q, which is not a shortcut action", label)
	}
}
