// SPDX-License-Identifier: AGPL-3.0-or-later

package docsmd_test

import (
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/docsmd"
)

const page = `# Title

## The two views

| View | Opened by |
| --- | --- |
| board | ` + "`enter`" + ` |
| help | ` + "`?`" + ` |

Prose after the table mentioning ` + "`z`" + `.

### Deeper

| View | Offered when |
| --- | --- |
| board | always |

## Another section

| View | Opened by |
| --- | --- |
| decoy | nothing |
`

func TestSectionStopsAtTheNextHeadingOfTheSameLevel(t *testing.T) {
	t.Parallel()
	got, err := docsmd.Section(page, "## The two views")
	if err != nil {
		t.Fatalf("Section: %v", err)
	}
	if !strings.Contains(got, "| board |") {
		t.Error("the section lost its own table")
	}
	if !strings.Contains(got, "### Deeper") {
		t.Error("the section stopped at a deeper heading")
	}
	if strings.Contains(got, "decoy") {
		t.Error("the section ran into the next section")
	}
}

func TestSectionNamesTheHeadingItCannotFind(t *testing.T) {
	t.Parallel()
	_, err := docsmd.Section(page, "## Nothing here")
	if err == nil || !strings.Contains(err.Error(), "## Nothing here") {
		t.Errorf("error does not name the missing heading: %v", err)
	}
	if _, err := docsmd.Section(page, "not a heading"); err == nil {
		t.Error("a line with no hashes was accepted as a heading")
	}
}

func TestTableReadsTheHeaderItIsGiven(t *testing.T) {
	t.Parallel()
	section, err := docsmd.Section(page, "## The two views")
	if err != nil {
		t.Fatalf("Section: %v", err)
	}
	rows, err := docsmd.Table(section, "View", "Opened by")
	if err != nil {
		t.Fatalf("Table: %v", err)
	}
	if got := docsmd.Column(rows, 0); len(got) != 2 || got[0] != "board" || got[1] != "help" {
		t.Errorf("rows are %v", got)
	}
	other, err := docsmd.Table(section, "View", "Offered when")
	if err != nil {
		t.Fatalf("Table: %v", err)
	}
	if got := docsmd.Column(other, 1); len(got) != 1 || got[0] != "always" {
		t.Errorf("the two tables with a View column were not told apart: %v", got)
	}
}

func TestTableStopsAtTheProseUnderIt(t *testing.T) {
	t.Parallel()
	rows, err := docsmd.Table(page, "View", "Opened by")
	if err != nil {
		t.Fatalf("Table: %v", err)
	}
	for _, r := range rows {
		if strings.Contains(strings.Join(r, " "), "Prose") {
			t.Errorf("a paragraph was read as a row: %v", r)
		}
	}
}

func TestTableNamesTheHeaderItCannotFind(t *testing.T) {
	t.Parallel()
	_, err := docsmd.Table(page, "Keys", "Action")
	if err == nil || !strings.Contains(err.Error(), "| Keys | Action |") {
		t.Errorf("error does not name the header: %v", err)
	}
	if _, err := docsmd.Table("| A | B |\n| C | D |\n", "A", "B"); err == nil {
		t.Error("a table with no divider row was accepted")
	}
	if _, err := docsmd.Table("| A | B |\n| --- | --- |\n", "A", "B"); err == nil {
		t.Error("a table with no rows was accepted")
	}
}

func TestHeadingReportsTheTextItMatched(t *testing.T) {
	t.Parallel()
	got, err := docsmd.Heading(page, 2, func(text string) bool { return strings.HasSuffix(text, "views") })
	if err != nil {
		t.Fatalf("Heading: %v", err)
	}
	if got != "The two views" {
		t.Errorf("heading is %q", got)
	}
	if _, err := docsmd.Heading(page, 2, func(string) bool { return false }); err == nil {
		t.Error("a heading nothing matched was reported as found")
	}
}

func TestCodesReadsTheBacktickedSpans(t *testing.T) {
	t.Parallel()
	got := docsmd.Codes("`↑`/`k`, `↓`/`j`")
	want := []string{"↑", "k", "↓", "j"}
	if len(got) != len(want) {
		t.Fatalf("codes are %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("code %d is %q, want %q", i, got[i], want[i])
		}
	}
	if len(docsmd.Codes("no code here")) != 0 {
		t.Error("prose with no backticks produced a code")
	}
}

func TestColumnSkipsARowTooShortToHaveIt(t *testing.T) {
	t.Parallel()
	got := docsmd.Column([]docsmd.Row{{"a", "b"}, {"c"}}, 1)
	if len(got) != 1 || got[0] != "b" {
		t.Errorf("column is %v", got)
	}
}

func TestReadResolvesFromTheModuleRoot(t *testing.T) {
	t.Parallel()
	got, err := docsmd.Read("go.mod")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if !strings.Contains(got, "module github.com/heliopsy/tix") {
		t.Error("go.mod was read from somewhere other than the module root")
	}
	if _, err := docsmd.Read("docs/no-such-page.md"); err == nil {
		t.Error("a missing page was read without error")
	}
}
