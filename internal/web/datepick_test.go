// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"encoding/json"
	"html"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/output"
	"github.com/heliopsy/tix/internal/quickadd"
	"github.com/heliopsy/tix/internal/testenv"
)

// writeDates runs assets/datepick.js's writeDate on each pattern and day, in
// node, so what is asserted is what the browser runs.
func writeDates(t *testing.T, patterns []string, days []string) map[string][]string {
	t.Helper()
	node, err := exec.LookPath("node")
	if err != nil {
		if os.Getenv("GITHUB_ACTIONS") == "true" {
			t.Fatalf("no node on this CI runner: %s", nodeCapability.Why)
		}
		testenv.Skip(t, nodeCapability)
	}
	script, err := filepath.Abs(filepath.Join("assets", "datepick.js"))
	if err != nil {
		t.Fatal(err)
	}
	in, err := json.Marshal(map[string]any{"script": script, "patterns": patterns, "days": days})
	if err != nil {
		t.Fatal(err)
	}
	const driver = `
const vm = require("node:vm"), fs = require("node:fs");
const input = JSON.parse(process.argv[1]);
const ctx = vm.createContext({ document: { readyState: "complete", querySelectorAll: () => [], addEventListener() {} } });
vm.runInContext("var window = globalThis; window.tix = {};", ctx);
vm.runInContext(fs.readFileSync(input.script, "utf8"), ctx);
const out = {};
for (const p of input.patterns) out[p] = input.days.map((d) => ctx.tix.writeDate(p, d));
process.stdout.write(JSON.stringify(out));
`
	got, err := exec.Command(node, "-e", driver, string(in)).Output()
	if err != nil {
		t.Fatalf("running datepick.js in node: %v", err)
	}
	out := map[string][]string{}
	if err := json.Unmarshal(got, &out); err != nil {
		t.Fatalf("reading %s: %v", got, err)
	}
	return out
}

// attrOf is one attribute of the input with the given id.
func attrOf(t *testing.T, page, id, attr string) string {
	t.Helper()
	tag := between(t, page, `<input id="`+id+`"`, ">")
	if tag == "" {
		t.Fatalf("the page has no input %q", id)
	}
	return html.UnescapeString(between(t, tag+">", attr+`="`, `"`))
}

// A day picked from the calendar is written into the due field in the
// pattern the page draws there, by the shipped script, and the server reads
// what it wrote back as the same day, under every date format.
func TestAPickedDayReadsBackAsTheSameDay(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	ref := f.as("alice").createTask("infra", "picked")
	days := []string{"2026-11-06", "2026-01-31", "2027-12-09", "2028-02-29"}

	patterns := map[string]string{}
	var all []string
	for _, format := range output.TimeFormats {
		b := f.as("alice")
		b.setCookie("tix_time_format", format)
		list, task := b.page("/tasks"), b.page("/tasks/"+ref)
		p := attrOf(t, list, "new-due", "data-date-pattern")
		if other := attrOf(t, task, "due", "data-date-pattern"); other != p {
			t.Errorf("%s: the add form writes %q and the task screen %q", format, p, other)
		}
		for _, page := range []string{list, task} {
			if !strings.Contains(page, `<span class="datepick" data-datepick="new-due" hidden>`) &&
				!strings.Contains(page, `<span class="datepick" data-datepick="due" hidden>`) {
				t.Errorf("%s: a due field has no calendar button hidden until the script shows it", format)
			}
		}
		patterns[format] = p
		all = append(all, p)
	}
	written := writeDates(t, all, days)
	for format, p := range patterns {
		style, err := output.NewTimeStyle(format, "utc")
		if err != nil {
			t.Fatal(err)
		}
		for i, day := range days {
			text := written[p][i]
			got, err := quickadd.ParseDueIn(text, time.Now(), style.DateOrder())
			if err != nil {
				t.Errorf("%s: the picked day %s was written %q, which is refused: %v", format, day, text, err)
				continue
			}
			if got.Format(time.DateOnly) != day {
				t.Errorf("%s: the picked day %s was written %q, read back as %s", format, day, text, got.Format(time.DateOnly))
			}
		}
		if want := style.DateOrder(); want != output.DateISO && written[p][0] == days[0] {
			t.Errorf("%s: a day-first or month-first reader got the day written as ISO", format)
		}
	}
}

// A due field shows what it takes before anything is typed: the placeholder
// is drawn from the same examples as the tip, its date in the reader's format,
// and every value in it is one the field accepts.
func TestTheDuePlaceholderIsAcceptedExamples(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	ref := f.as("alice").createTask("infra", "placeholder")
	for _, format := range output.TimeFormats {
		b := f.as("alice")
		b.setCookie("tix_time_format", format)
		style, err := output.NewTimeStyle(format, "utc")
		if err != nil {
			t.Fatal(err)
		}
		examples := quickadd.DueExamples(style.Date(quickadd.ExampleDay))
		for _, at := range []struct{ path, id string }{{"/tasks", "new-due"}, {"/tasks/" + ref, "due"}} {
			ph := attrOf(t, b.page(at.path), at.id, "placeholder")
			values := strings.Split(strings.Replace(ph, " or ", ", ", 1), ", ")
			if len(values) != 3 || values[2] != style.Date(quickadd.ExampleDay) {
				t.Errorf("%s %s: placeholder %q does not end with the date in the reader's format", format, at.path, ph)
			}
			for _, v := range values {
				if !containsString(examples, v) {
					t.Errorf("%s %s: placeholder value %q is not one of the tip's examples %q", format, at.path, v, examples)
				}
				if _, err := quickadd.ParseDueIn(v, time.Now(), style.DateOrder()); err != nil {
					t.Errorf("%s %s: placeholder value %q is refused: %v", format, at.path, v, err)
				}
			}
		}
	}
}
