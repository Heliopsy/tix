// SPDX-License-Identifier: AGPL-3.0-or-later

package tasklink

import "testing"

// The same cases are in internal/web/jstest/share.test.mjs, so the browser
// and the command line never disagree about what a shared link says.
func TestForms(t *testing.T) {
	tests := []struct {
		name, ref, title, base, url, text, markdown string
	}{
		{"plain", "INFRA-12", "Fix the build", "https://tix.example.com", "https://tix.example.com/tasks/INFRA-12",
			"INFRA-12 Fix the build", "[INFRA-12 Fix the build](https://tix.example.com/tasks/INFRA-12)"},
		{"trailing slash", "infra-12", "x", "https://tix.example.com/", "https://tix.example.com/tasks/infra-12",
			"infra-12 x", "[infra-12 x](https://tix.example.com/tasks/infra-12)"},
		{"brackets", "INFRA-3", `see [docs] \ here`, "http://h:8080", "http://h:8080/tasks/INFRA-3",
			`INFRA-3 see [docs] \ here`, `[INFRA-3 see \[docs\] \\ here](http://h:8080/tasks/INFRA-3)`},
		{"lines", "INFRA-4", "one\n  two\tthree ", "https://h", "https://h/tasks/INFRA-4",
			"INFRA-4 one two three", "[INFRA-4 one two three](https://h/tasks/INFRA-4)"},
		{"no title", "INFRA-5", "  ", "https://h", "https://h/tasks/INFRA-5", "INFRA-5", "[INFRA-5](https://h/tasks/INFRA-5)"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			u := URL(tc.base, tc.ref)
			if u != tc.url {
				t.Errorf("URL = %q, want %q", u, tc.url)
			}
			if got := Text(tc.ref, tc.title); got != tc.text {
				t.Errorf("Text = %q, want %q", got, tc.text)
			}
			if got := Markdown(tc.ref, tc.title, u); got != tc.markdown {
				t.Errorf("Markdown = %q, want %q", got, tc.markdown)
			}
		})
	}
}

func TestPathEscapesWhatIsNotAReference(t *testing.T) {
	if got := Path("a/b c"); got != "/tasks/a%2Fb%20c" {
		t.Errorf("Path = %q", got)
	}
}
