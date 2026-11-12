// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import "testing"

func TestSharedLineJoinsAShareTheWayTheAppDoes(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, title, text, link, want string
	}{
		{"nothing", "", "", "", ""},
		{"text only", "", "buy milk", "", "buy milk"},
		{"title only", "Notes", "", "", "Notes"},
		{"title and text", "Notes", "read them", "", "Notes read them"},
		{"text repeating the title", "Notes", "Notes to read", "", "Notes to read"},
		{"url only", "", "", "https://example.test", "https://example.test"},
		{"url after the title", "Notes", "", "https://example.test", "Notes https://example.test"},
		{"url already in the text", "", "see https://example.test now", "https://example.test", "see https://example.test now"},
		{"lines and runs of space", " a\n\tb ", "c\r\nd", "", "a b c d"},
	} {
		if got := sharedLine(tc.title, tc.text, tc.link); got != tc.want {
			t.Errorf("%s: sharedLine(%q, %q, %q) = %q, want %q", tc.name, tc.title, tc.text, tc.link, got, tc.want)
		}
	}
}
