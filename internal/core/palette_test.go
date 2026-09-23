// SPDX-License-Identifier: AGPL-3.0-or-later

package core

import "testing"

func TestNormalizeProjectColor(t *testing.T) {
	tests := []struct {
		in    string
		want  ProjectColor
		valid bool
	}{
		{"", ColorNone, true},
		{"  ", ColorNone, true},
		{"blue", ColorBlue, true},
		{" BLUE ", ColorBlue, true},
		{"Violet", ColorViolet, true},
		{"#3b5bdb", ColorNone, false},
		{"chartreuse", ColorNone, false},
		{"rgb(1,2,3)", ColorNone, false},
	}
	for _, tc := range tests {
		got, err := NormalizeProjectColor(tc.in)
		if tc.valid && err != nil {
			t.Errorf("NormalizeProjectColor(%q) = %v, want %q", tc.in, err, tc.want)
			continue
		}
		if !tc.valid {
			if !IsKind(err, KindInvalid) {
				t.Errorf("NormalizeProjectColor(%q) = %v, want invalid", tc.in, err)
			}
			continue
		}
		if got != tc.want {
			t.Errorf("NormalizeProjectColor(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestNormalizeProjectIcon(t *testing.T) {
	tests := []struct {
		in    string
		want  string
		valid bool
	}{
		{"", "", true},
		{"   ", "", true},
		{" \U0001F680 ", "\U0001F680", true},
		{"⚙️", "⚙️", true},
		{"IN", "IN", true},
		{"infra", "", false},
		{"a\u0007", "", false},
		{"a b", "", false},
	}
	for _, tc := range tests {
		got, err := NormalizeProjectIcon(tc.in)
		if !tc.valid {
			if !IsKind(err, KindInvalid) {
				t.Errorf("NormalizeProjectIcon(%q) = %v, want invalid", tc.in, err)
			}
			continue
		}
		if err != nil {
			t.Errorf("NormalizeProjectIcon(%q) = %v, want %q", tc.in, err, tc.want)
			continue
		}
		if got != tc.want {
			t.Errorf("NormalizeProjectIcon(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestProjectColorsAreDistinctAndValid(t *testing.T) {
	seen := map[ProjectColor]bool{}
	for _, c := range ProjectColors() {
		if c == ColorNone {
			t.Error("the palette offers the empty colour as a choice")
		}
		if !c.Valid() {
			t.Errorf("palette member %q does not validate", c)
		}
		if seen[c] {
			t.Errorf("palette lists %q twice", c)
		}
		seen[c] = true
	}
	if len(seen) == 0 {
		t.Fatal("the palette is empty")
	}
	ProjectColors()[0] = "mutated"
	if ProjectColors()[0] == "mutated" {
		t.Error("ProjectColors returns the package's own slice")
	}
}
