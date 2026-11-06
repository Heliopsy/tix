// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"html"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/output"
	"github.com/heliopsy/tix/internal/quickadd"
)

// exampleCode is one value a tip offers to type.
var exampleCode = regexp.MustCompile(`<code>([^<]*)</code>`)

// tipExamples are the values the tip right after a field's label offers, read
// from that one tip rather than from the page, so another field's examples
// cannot stand in for these.
func tipExamples(t *testing.T, page, label string) []string {
	t.Helper()
	at := strings.Index(page, label)
	if at < 0 {
		t.Fatalf("the page has no %s:\n%s", label, page)
	}
	tip := between(t, page[at:], `<details class="field-info tip">`, `</details>`)
	block := between(t, tip, `<span class="tip-examples">`, `</span>`)
	if block == "" {
		t.Fatalf("the tip after %s offers no examples: %q", label, tip)
	}
	var out []string
	for _, m := range exampleCode.FindAllStringSubmatch(block, -1) {
		out = append(out, html.UnescapeString(m[1]))
	}
	if len(out) < 3 {
		t.Fatalf("the tip after %s offers %d examples, want a few: %q", label, len(out), block)
	}
	return out
}

// Every value a due, repeat or quick-add tip offers is one the real parser
// takes, under every date format a reader can choose, and the due tip writes
// its date in that format.
func TestEveryExampleATipOffersIsAccepted(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	ref := f.as("alice").createTask("infra", "hint check")
	now := time.Now()
	for _, format := range output.TimeFormats {
		t.Run(format, func(t *testing.T) {
			t.Parallel()
			b := f.reader(t, format, "")
			style, err := output.NewTimeStyle(format, "utc")
			if err != nil {
				t.Fatal(err)
			}
			list, task := b.page("/tasks"), b.page("/tasks/"+ref)

			for _, due := range [][]string{
				tipExamples(t, list, `<label for="new-due">`),
				tipExamples(t, task, `<label for="due">`),
			} {
				if want := style.Date(quickadd.ExampleDay); !containsString(due, want) {
					t.Errorf("the due tip %q does not write its date as %s", due, want)
				}
				for _, ex := range due {
					if _, err := quickadd.ParseDueIn(ex, now, style.DateOrder()); err != nil {
						t.Errorf("the due example %q is refused: %v", ex, err)
					}
				}
			}
			for _, repeat := range [][]string{
				tipExamples(t, list, `<label for="new-repeat">`),
				tipExamples(t, task, `<label for="repeat">`),
			} {
				for _, ex := range repeat {
					if _, err := core.ParseRecurrence(ex); err != nil {
						t.Errorf("the repeat example %q is refused: %v", ex, err)
					}
				}
			}
			for _, ex := range tipExamples(t, list, `aria-label="`+"New task"+`"`) {
				r, err := quickadd.ParseIn("pay rent "+ex, now, style.DateOrder())
				if err != nil || r.Due == nil {
					t.Errorf("the quick-add example %q is refused: %v", ex, err)
				}
			}
		})
	}
}

// containsString reports whether list holds want.
func containsString(list []string, want string) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}
	return false
}

// A typed due date is read in the order the reader's date format writes one:
// ISO always, digits in that order, and digits in an unknown order refused
// rather than guessed. The field then shows the day back in that format.
func TestADueDateIsTypedInTheReadersFormat(t *testing.T) {
	t.Parallel()
	cases := []struct {
		format string
		typed  string
		want   string
		shown  string
	}{
		{output.TimeDotted, "06.11.2026", "2026-11-06", "06.11.2026"},
		{output.TimeDayFirst, "06/11/2026", "2026-11-06", "06/11/2026"},
		{output.TimeUS, "06/11/2026", "2026-06-11", "06/11/2026"},
		{output.TimeLong, "6 Nov 2026", "2026-11-06", "6 Nov 2026"},
		{output.TimeDotted, "2026-11-06", "2026-11-06", "06.11.2026"},
		{output.TimeISO, "2026-11-06", "2026-11-06", "2026-11-06"},
		{output.TimeISO, "06.11.2026", "", ""},
		{output.TimeISO, "06/11/2026", "", ""},
		{output.TimeUS, "06.11.2026", "", ""},
	}
	for _, c := range cases {
		t.Run(c.format+" "+c.typed, func(t *testing.T) {
			t.Parallel()
			f := newFixture(t)
			b := f.reader(t, c.format, "")
			ref := b.createTask("infra", "deadline")
			task, err := f.svc.GetTask(f.ctx(), core.MustParseTaskRef(ref))
			if err != nil {
				t.Fatal(err)
			}
			resp := b.post("/tasks/"+ref, url.Values{"due_at": {c.typed},
				"version": {strconv.Itoa(task.Version)}, "title": {"deadline"}})
			_ = resp.Body.Close()
			got, err := f.svc.GetTask(f.ctx(), core.MustParseTaskRef(ref))
			if err != nil {
				t.Fatal(err)
			}
			if c.want == "" {
				if resp.StatusCode == http.StatusSeeOther || got.DueAt != nil {
					t.Fatalf("%q was read as %v under %s, want a refusal", c.typed, got.DueAt, c.format)
				}
				return
			}
			wantStatus(t, resp, http.StatusSeeOther)
			if got.DueAt == nil || got.DueAt.UTC().Format(time.DateOnly) != c.want {
				t.Fatalf("%q under %s saved %v, want %s", c.typed, c.format, got.DueAt, c.want)
			}
			if shown := inputValue(t, b.page("/tasks/"+ref), "due"); shown != c.shown {
				t.Errorf("the due field shows %q, want %q", shown, c.shown)
			}
		})
	}
}

// Add with details and the quick-add line read a due date the same way, and
// the due field takes a word.
func TestTheAddFormReadsADueDateInTheReadersFormat(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.reader(t, output.TimeDotted, "")
	resp := b.post("/tasks", url.Values{"project_ref": {"infra"}, "title": {"dotted"}, "due_at": {"06.11.2026"}})
	_ = resp.Body.Close()
	wantStatus(t, resp, http.StatusSeeOther)
	resp = b.post("/tasks", url.Values{"project_ref": {"infra"}, "title": {"worded"}, "due_at": {"tomorrow"}})
	_ = resp.Body.Close()
	wantStatus(t, resp, http.StatusSeeOther)
	resp = b.post("/tasks", url.Values{"project_ref": {"infra"}, "quick": {"1"}, "title": {"quick due:06.11.2026"}})
	_ = resp.Body.Close()
	wantStatus(t, resp, http.StatusSeeOther)

	list, err := f.svc.ListTasks(f.ctx(), core.TaskFilter{ProjectKeys: []string{"infra"}})
	if err != nil {
		t.Fatal(err)
	}
	seen := 0
	for _, task := range list.Tasks {
		switch task.Title {
		case "dotted", "quick":
			seen++
			if task.DueAt == nil || task.DueAt.UTC().Format(time.DateOnly) != "2026-11-06" {
				t.Errorf("%s: 06.11.2026 saved as %v", task.Title, task.DueAt)
			}
		case "worded":
			seen++
			if task.DueAt == nil {
				t.Error("tomorrow saved no due date")
			}
		}
	}
	if seen != 3 {
		t.Fatalf("found %d of the three tasks", seen)
	}
}
