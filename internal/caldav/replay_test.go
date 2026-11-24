// SPDX-License-Identifier: AGPL-3.0-or-later

package caldav

import (
	"bufio"
	"context"
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

// step is one request of a client sequence and what its answer must hold,
// or an action on tix between requests.
type step struct {
	title   string
	line    int
	method  string
	path    string
	header  map[string]string
	body    string
	expect  int
	asserts []string
}

// readSequence reads a testdata file: comment lines, then steps opened by
// "### title", each a request line, headers, a blank line, a body and an
// EXPECT line with its assertions.
func readSequence(t *testing.T, path string) []step {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	var (
		steps []step
		cur   *step
		phase string
		body  []string
	)
	sc := bufio.NewScanner(file)
	for n := 1; sc.Scan(); n++ {
		line := sc.Text()
		switch {
		case strings.HasPrefix(line, "### "):
			steps = append(steps, step{title: strings.TrimPrefix(line, "### "), line: n, header: map[string]string{}})
			cur, phase, body = &steps[len(steps)-1], "request", nil
		case cur == nil || strings.HasPrefix(line, "#"):
		case phase == "request":
			if strings.TrimSpace(line) == "" {
				continue
			}
			cur.method, cur.path, _ = strings.Cut(line, " ")
			phase = "headers"
		case phase == "headers" && strings.TrimSpace(line) == "":
			phase = "body"
		case phase == "headers":
			k, v, _ := strings.Cut(line, ":")
			cur.header[k] = strings.TrimSpace(v)
		case strings.HasPrefix(line, "EXPECT "):
			cur.expect, err = strconv.Atoi(strings.TrimPrefix(line, "EXPECT "))
			if err != nil {
				t.Fatalf("%s:%d: %v", path, n, err)
			}
			cur.body = strings.TrimSpace(strings.Join(body, "\n"))
			phase = "assert"
		case phase == "body":
			body = append(body, line)
		case phase == "assert" && strings.TrimSpace(line) != "":
			cur.asserts = append(cur.asserts, strings.TrimSpace(line))
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	return steps
}

// replay runs one sequence against a fresh fixture.
type replay struct {
	t     *testing.T
	f     *fixture
	ctx   context.Context
	vars  map[string]string
	tasks map[string]*core.Task
}

func (rp *replay) expand(s string) string {
	for k, v := range rp.vars {
		s = strings.ReplaceAll(s, "{"+k+"}", v)
	}
	return s
}

// clarkName reads {namespace}local.
func clarkName(s string) xml.Name {
	space, local, _ := strings.Cut(strings.TrimPrefix(s, "{"), "}")
	return xml.Name{Space: space, Local: local}
}

func (rp *replay) run(s step) {
	t := rp.t
	t.Helper()
	where := s.title + " (line " + strconv.Itoa(s.line) + ")"
	if s.method == "DO" {
		rp.do(where, rp.expand(s.path))
		return
	}
	header := map[string]string{}
	for k, v := range s.header {
		header[k] = rp.expand(v)
	}
	path, body := rp.expand(s.path), rp.expand(s.body)
	var r reply
	if path == core.DAVWellKnownPath {
		w := httptest.NewRecorder()
		rp.f.h.WellKnown().ServeHTTP(w, httptest.NewRequest(s.method, path, strings.NewReader(body)))
		r = reply{status: w.Code, header: w.Header(), body: w.Body.String()}
	} else {
		r = rp.f.serve(rp.ctx, s.method, path, header, body)
	}
	if r.status != s.expect {
		t.Fatalf("%s: %s %s answered %d, want %d: %s", where, s.method, path, r.status, s.expect, r.body)
	}
	var doc *msDoc
	ms := func() msDoc {
		if doc == nil {
			d := parseMS(t, r)
			doc = &d
		}
		return *doc
	}
	for _, a := range s.asserts {
		fields := strings.Fields(rp.expand(a))
		switch fields[0] {
		case "HEADER":
			if got := r.header.Get(fields[1]); !strings.Contains(got, strings.Join(fields[2:], " ")) {
				t.Errorf("%s: header %s is %q", where, fields[1], got)
			}
		case "RESPONSES":
			if n, _ := strconv.Atoi(fields[1]); len(ms().Responses) != n {
				t.Errorf("%s: %d responses, want %d: %v", where, len(ms().Responses), n, ms().hrefs())
			}
		case "STATUS":
			if got := ms().response(t, fields[1]).Status; got != "HTTP/1.1 "+fields[2]+" "+http.StatusText(atoi(fields[2])) {
				t.Errorf("%s: %s has status %q, want %s", where, fields[1], got, fields[2])
			}
		case "PROP":
			p, status, ok := ms().response(t, fields[1]).prop(clarkName(fields[2]))
			want := "HTTP/1.1 " + fields[3] + " " + http.StatusText(atoi(fields[3]))
			if !ok || status != want {
				t.Errorf("%s: %s of %s is %v in %q, want %s", where, fields[2], fields[1], ok, status, want)
				continue
			}
			if v := strings.Join(fields[4:], " "); v != "" && !strings.Contains(p.Inner, v) && !strings.Contains(p.Text, v) {
				t.Errorf("%s: %s of %s is %q, want it to hold %q", where, fields[2], fields[1], p.Inner, v)
			}
		case "SAVE":
			if fields[2] == "sync-token" {
				rp.vars[fields[1]] = ms().SyncToken
				continue
			}
			rp.vars[fields[1]] = ms().response(t, fields[2]).okProp(t, clarkName(fields[3])).Text
		case "SAME", "DIFFERENT":
			if (rp.vars[fields[1]] == rp.vars[fields[2]]) != (fields[0] == "SAME") {
				t.Errorf("%s: %s %q and %s %q, want %s", where, fields[1], rp.vars[fields[1]], fields[2],
					rp.vars[fields[2]], strings.ToLower(fields[0]))
			}
		case "HAS":
			if v := strings.Join(fields[1:], " "); !strings.Contains(r.body, v) {
				t.Errorf("%s: the body lacks %q", where, v)
			}
		case "LACKS":
			if v := strings.Join(fields[1:], " "); strings.Contains(r.body, v) {
				t.Errorf("%s: the body holds %q", where, v)
			}
		case "SAVEHEADER":
			rp.vars[fields[1]] = r.header.Get(fields[2])
		case "TASK":
			rp.checkTask(where, fields[1], fields[2], strings.Join(fields[3:], " "))
		default:
			t.Fatalf("%s: unknown assertion %q", where, a)
		}
	}
}

func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

// do changes tix between requests, as a person in the browser would.
func (rp *replay) do(where, action string) {
	verb, task, _ := strings.Cut(action, " ")
	tk, ok := rp.tasks[task]
	if !ok {
		rp.t.Fatalf("%s: no task %q", where, task)
	}
	switch verb {
	case "retitle":
		title := tk.Title + " (edited)"
		if _, err := rp.f.svc.UpdateTask(adminCtx(rp.f.acme), core.TaskRef{ID: tk.ID}, core.UpdateTaskInput{Title: &title}); err != nil {
			rp.t.Fatalf("%s: %v", where, err)
		}
	case "cancel":
		rp.f.move(rp.f.acme, tk, "cancelled")
	case "tag":
		tags := []string{"ops", "security"}
		if _, err := rp.f.svc.UpdateTask(adminCtx(rp.f.acme), core.TaskRef{ID: tk.ID}, core.UpdateTaskInput{Tags: &tags}); err != nil {
			rp.t.Fatalf("%s: %v", where, err)
		}
	default:
		rp.t.Fatalf("%s: unknown action %q", where, verb)
	}
}

// checkTask asserts one field of a task as tix holds it: a task of the
// sequence by its key, or a task the client created by its title.
func (rp *replay) checkTask(where, key, field, want string) {
	t := rp.t
	t.Helper()
	var got *core.Task
	if tk, ok := rp.tasks[key]; ok {
		var err error
		if got, err = rp.f.svc.GetTask(adminCtx(rp.f.acme), core.TaskRef{ID: tk.ID}); err != nil {
			t.Fatalf("%s: %v", where, err)
		}
	} else {
		page, err := rp.f.svc.ListTasks(adminCtx(rp.f.acme), core.TaskFilter{IncludeSnoozed: true,
			Page: core.Page{Limit: core.MaxPageLimit}})
		if err != nil {
			t.Fatal(err)
		}
		for i := range page.Tasks {
			if page.Tasks[i].Title == strings.ReplaceAll(key, "_", " ") {
				got = &page.Tasks[i]
			}
		}
		if got == nil {
			t.Fatalf("%s: no task titled %q", where, key)
		}
	}
	var have string
	switch field {
	case "title":
		have = got.Title
	case "status":
		have = got.Status
	case "tags":
		have = strings.Join(got.Tags, ",")
	case "due":
		if got.DueAt != nil {
			have = got.DueAt.UTC().Format("2006-01-02T15:04Z")
		}
	case "snoozed":
		if got.SnoozedUntil != nil {
			have = got.SnoozedUntil.UTC().Format("2006-01-02T15:04Z")
		}
	case "assignee":
		have = got.AssigneeActorID
	case "trashed":
		have = strconv.FormatBool(got.DeletedAt != nil)
	default:
		t.Fatalf("%s: unknown task field %q", where, field)
	}
	if have != want {
		t.Errorf("%s: %s of %s is %q, want %q", where, field, key, have, want)
	}
}

// TestClientSequencesReplay runs every sequence in testdata from discovery
// through a full and an incremental sync.
func TestClientSequencesReplay(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("testdata", "*.txt"))
	if err != nil || len(files) < 5 {
		t.Fatalf("found %d sequences: %v", len(files), err)
	}
	for _, path := range files {
		t.Run(filepath.Base(path), func(t *testing.T) {
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(raw), "not captured traffic") {
				t.Errorf("%s does not say it was written from published behaviour", path)
			}
			f := newFixture(t)
			p := f.readOnly(f.acme.bob, core.DAVCollectionsAll)
			if strings.HasPrefix(filepath.Base(path), "write-") {
				p = f.password(f.acme.bob, core.DAVCollectionsAll)
			}
			tasks := map[string]*core.Task{
				"open1": f.task(f.acme, "Rotate keys", f.acme.bob.ID),
				"open2": f.task(f.acme, "Renew certificate", f.acme.bob.ID),
				"done":  f.task(f.acme, "Order laptops", f.acme.bob.ID),
			}
			f.move(f.acme, tasks["done"], "done")
			homePath := home(f.acme, f.acme.bob)
			vars := map[string]string{
				"principal": "/dav/acme/principals/" + strings.ToLower(f.acme.bob.ID) + "/",
				"home":      homePath,
				"assigned":  homePath + "assigned/",
				"infra":     homePath + "p-" + strings.ToLower(f.acme.project.ID) + "/",
				"web":       homePath + "p-" + strings.ToLower(f.acme.web.ID) + "/",
				"bob":       f.acme.bob.ID,
			}
			for k, tk := range tasks {
				vars[k] = name(tk)
			}
			rp := &replay{t: t, f: f, ctx: davCtx(f.acme.scope.TenantID, p), vars: vars, tasks: tasks}
			steps := readSequence(t, path)
			if len(steps) < 6 {
				t.Fatalf("only %d steps", len(steps))
			}
			for _, s := range steps {
				rp.run(s)
			}
		})
	}
}
