// SPDX-License-Identifier: AGPL-3.0-or-later

package markdown_test

import (
	"errors"
	"testing"

	"github.com/heliopsy/tix/internal/markdown"
)

func TestChecklistCounts(t *testing.T) {
	t.Parallel()
	cases := []struct {
		src         string
		done, total int
	}{
		{"", 0, 0},
		{"no list here [x]", 0, 0},
		{"- [ ] a\n- [x] b\n- [X] c\n", 2, 3},
		{"* [ ] a\n+ [x] b\n1. [ ] c\n", 1, 3},
		{"- [ ] a\n  - [x] nested\n", 1, 2},
		{"> - [x] quoted\n", 1, 1},
		{"```\n- [ ] in code\n```\n", 0, 0},
		{"- plain\n- [ ]\n", 0, 1},
	}
	for _, c := range cases {
		done, total := markdown.Checklist(c.src)
		if done != c.done || total != c.total {
			t.Errorf("Checklist(%q) = %d/%d, want %d/%d", c.src, done, total, c.done, c.total)
		}
	}
}

func TestSetItemChangesOnlyThatMarker(t *testing.T) {
	t.Parallel()
	src := "intro [ ] not an item\n\n- [ ] one\n- [x] two\n  - [ ] three\n\n```\n- [ ] code\n```\n1. [ ] four\n"
	cases := []struct {
		n    int
		done bool
		want string
	}{
		{1, true, "intro [ ] not an item\n\n- [x] one\n- [x] two\n  - [ ] three\n\n```\n- [ ] code\n```\n1. [ ] four\n"},
		{2, false, "intro [ ] not an item\n\n- [ ] one\n- [ ] two\n  - [ ] three\n\n```\n- [ ] code\n```\n1. [ ] four\n"},
		{3, true, "intro [ ] not an item\n\n- [ ] one\n- [x] two\n  - [x] three\n\n```\n- [ ] code\n```\n1. [ ] four\n"},
		{4, true, "intro [ ] not an item\n\n- [ ] one\n- [x] two\n  - [ ] three\n\n```\n- [ ] code\n```\n1. [x] four\n"},
		{2, true, src},
	}
	for _, c := range cases {
		got, err := markdown.SetItem(src, c.n, c.done)
		if err != nil {
			t.Fatalf("SetItem(%d, %v): %v", c.n, c.done, err)
		}
		if got != c.want {
			t.Errorf("SetItem(%d, %v) =\n%q\nwant\n%q", c.n, c.done, got, c.want)
		}
	}
}

func TestSetItemRefusesAnItemThatIsNotThere(t *testing.T) {
	t.Parallel()
	for _, n := range []int{0, -1, 2} {
		if _, err := markdown.SetItem("- [ ] only\n", n, true); !errors.Is(err, markdown.ErrNoItem) {
			t.Errorf("SetItem(%d) error = %v, want ErrNoItem", n, err)
		}
	}
}

func TestSetItemKeepsLooseListsAndCRLF(t *testing.T) {
	t.Parallel()
	src := "- [ ] one\r\n\r\n- [ ] two\r\n"
	got, err := markdown.SetItem(src, 2, true)
	if err != nil || got != "- [ ] one\r\n\r\n- [x] two\r\n" {
		t.Fatalf("SetItem on a loose CRLF list = %q, %v", got, err)
	}
}
