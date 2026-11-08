// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/heliopsy/tix/internal/capability"
	"github.com/heliopsy/tix/internal/core"
)

// openTransferView presses the key that opens the transfer view and fails when
// it did not open.
func openTransferView(t *testing.T, m Model) Model {
	t.Helper()
	m, _ = m.reduce(pressKey("V"))
	if m.view != viewTransfer {
		t.Fatalf("V did not open the transfer view; view = %s", viewName(m.view))
	}
	return m
}

// startTransfer selects the transfer titled title, counting from the top, and
// opens its form.
func startTransfer(t *testing.T, m Model, title string) Model {
	t.Helper()
	rows := m.transferRows()
	at := slices.IndexFunc(rows, func(s transferSpec) bool { return s.title == title })
	if at < 0 {
		t.Fatalf("the transfer view offers no %q row", title)
	}
	m, _ = m.reduce(pressKey("g"))
	for range at {
		m, _ = m.reduce(pressKey("down"))
	}
	m, _ = m.reduce(pressKey("enter"))
	if !m.form.Open() || m.form.Kind != formTransfer {
		t.Fatalf("enter on %q opened no transfer form", title)
	}
	return m
}

// answer sets one field of the open form, through the text field when the
// cursor is on it, so submitting keeps what was set.
func answer(m Model, key, value string) Model {
	m.form = m.form.SetValue(key, value)
	if f, ok := m.form.Selected(); ok && f.Key == key && f.Typed() {
		m.input.SetValue(value)
	}
	return m
}

// submitTransfer applies the form and runs the call it starts.
func submitTransfer(t *testing.T, m Model) Model {
	t.Helper()
	m, cmd := m.reduce(pressKey("enter"))
	if cmd == nil {
		t.Fatalf("applying the form started nothing; status %q, error %q", m.status, m.err)
	}
	return runCmd(t, m, cmd)
}

// transferSection returns the lines of the transfer body from a heading to the
// blank line that ends it, so an assertion about the column mapping cannot be
// met by the preview rows or the title bar.
func transferSection(t *testing.T, frame, heading string) string {
	t.Helper()
	lines := strings.Split(frame, "\n")
	start := slices.IndexFunc(lines, func(l string) bool { return strings.TrimSpace(l) == heading })
	if start < 0 {
		t.Fatalf("the transfer view draws no %q section:\n%s", heading, frame)
	}
	end := len(lines)
	for i := start + 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "" {
			end = i
			break
		}
	}
	return strings.Join(lines[start:end], "\n")
}

// taskTitles lists the titles of the live board's project.
func taskTitles(t *testing.T, b *liveBoard) []string {
	t.Helper()
	page, err := b.local.ListTasks(b.ctx, core.TaskFilter{ProjectKeys: []string{"infra"}})
	if err != nil {
		t.Fatalf("listing tasks: %v", err)
	}
	out := make([]string, 0, len(page.Tasks))
	for _, task := range page.Tasks {
		out = append(out, task.Title)
	}
	slices.Sort(out)
	return out
}

func TestTheTransferViewExportsWhatTheCommandLineWrites(t *testing.T) {
	for _, tc := range []struct {
		name, title string
		set         map[string]string
		want        func(b *liveBoard, w io.Writer) error
	}{
		{
			name: "tasks as csv", title: "export tasks",
			want: func(b *liveBoard, w io.Writer) error {
				return b.local.ExportTasks(b.ctx, core.TaskExportInput{ProjectRef: "infra"}.Normalize(), w)
			},
		},
		{
			name: "tasks as json", title: "export tasks", set: map[string]string{"format": "json"},
			want: func(b *liveBoard, w io.Writer) error {
				return b.local.ExportTasks(b.ctx,
					core.TaskExportInput{ProjectRef: "infra", Format: core.TaskFormatJSON}.Normalize(), w)
			},
		},
		{
			name: "bundle", title: "export bundle",
			want: func(b *liveBoard, w io.Writer) error {
				return b.local.ExportBundle(b.ctx, core.BundleExportInput{ProjectRefs: []string{"infra"}}, w)
			},
		},
		{
			name: "snapshot", title: "export snapshot",
			want: func(b *liveBoard, w io.Writer) error {
				return b.local.ExportTo(b.ctx, core.ExportInput{}, w)
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := newLiveBoard(t)
			path := filepath.Join(t.TempDir(), "out")
			m := startTransfer(t, openTransferView(t, b.model), tc.title)
			for k, v := range tc.set {
				m = answer(m, k, v)
			}
			m = answer(m, "path", path)
			m = submitTransfer(t, m)
			if m.err != "" {
				t.Fatalf("the export failed: %s", m.err)
			}
			got, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("the export wrote no file: %v", err)
			}
			var want bytes.Buffer
			if err := tc.want(b, &want); err != nil {
				t.Fatalf("exporting through the service: %v", err)
			}
			if want.Len() == 0 {
				t.Fatal("the service wrote nothing, so this comparison proves nothing")
			}
			if !bytes.Equal(got, want.Bytes()) {
				t.Errorf("the file differs from what the service writes:\n got %q\nwant %q", got, want.Bytes())
			}
			if !strings.Contains(m.status, path) {
				t.Errorf("the status line %q does not name the file", m.status)
			}
		})
	}
}

func TestExportingOntoAFileThatExistsAsksFirst(t *testing.T) {
	b := newLiveBoard(t)
	path := filepath.Join(t.TempDir(), "infra.csv")
	if err := os.WriteFile(path, []byte("keep me"), 0o600); err != nil {
		t.Fatal(err)
	}
	m := answer(startTransfer(t, openTransferView(t, b.model), "export tasks"), "path", path)
	m, cmd := m.reduce(pressKey("enter"))
	if cmd != nil || !m.confirm.Open() || m.confirm.Kind != confirmOverwrite {
		t.Fatalf("an existing file was not asked about; confirm %+v", m.confirm)
	}
	if !strings.Contains(m.confirm.Question(), path) {
		t.Errorf("the question %q does not name the file", m.confirm.Question())
	}
	m, _ = m.reduce(pressKey("esc"))
	if got, _ := os.ReadFile(path); string(got) != "keep me" {
		t.Fatalf("cancelling the overwrite changed the file: %q", got)
	}
	m = answer(startTransfer(t, m, "export tasks"), "path", path)
	m, _ = m.reduce(pressKey("enter"))
	m, cmd = m.reduce(pressKey("y"))
	runCmd(t, m, cmd)
	if got, _ := os.ReadFile(path); string(got) == "keep me" {
		t.Error("agreeing to the overwrite left the file as it was")
	}
}

func TestATaskImportDryRunsThenWritesOnlyOnceConfirmed(t *testing.T) {
	b := newLiveBoard(t)
	path := filepath.Join(t.TempDir(), "jira.csv")
	csv := "Summary,Status\nFirst imported,todo\nSecond imported,todo\n"
	if err := os.WriteFile(path, []byte(csv), 0o600); err != nil {
		t.Fatal(err)
	}
	before := taskTitles(t, b)

	m := answer(startTransfer(t, openTransferView(t, b.model), "import tasks"), "path", path)
	m = submitTransfer(t, m)
	if m.err != "" {
		t.Fatalf("the dry run failed: %s", m.err)
	}
	if got := taskTitles(t, b); !slices.Equal(got, before) {
		t.Fatalf("the dry run wrote tasks: %v, before %v", got, before)
	}

	frame := m.Frame()
	columns := transferSection(t, frame, "columns, and the field each fills:")
	for _, want := range []string{"Summary", "title", "Status", "status"} {
		if !strings.Contains(columns, want) {
			t.Errorf("the column mapping does not show %q:\n%s", want, columns)
		}
	}
	preview := transferSection(t, frame, "the first rows as they would be written:")
	for _, want := range []string{"First imported", "Second imported", "create"} {
		if !strings.Contains(preview, want) {
			t.Errorf("the preview does not show %q:\n%s", want, preview)
		}
	}
	if !m.confirm.Open() || m.confirm.Kind != confirmImport {
		t.Fatalf("the dry run did not ask before writing; confirm %+v", m.confirm)
	}
	if q := m.confirm.Question(); !strings.Contains(q, "2 rows of jira.csv into infra") {
		t.Errorf("the question %q does not say what it will write", q)
	}

	m, _ = m.reduce(pressKey("esc"))
	if got := taskTitles(t, b); !slices.Equal(got, before) {
		t.Fatalf("cancelling the question wrote tasks: %v", got)
	}
	if m.transferPlan == nil || !strings.Contains(m.footerLines(), "import") {
		t.Fatalf("cancelling the question lost the preview or the key that writes it:\n%s", m.footerLines())
	}

	m, _ = m.reduce(pressKey("y"))
	m, cmd := m.reduce(pressKey("y"))
	if cmd == nil {
		t.Fatal("agreeing to the import started nothing")
	}
	msg := cmd()
	m, _ = m.reduce(msg)
	if m.err != "" {
		t.Fatalf("the import failed: %s", m.err)
	}
	want := append(slices.Clone(before), "First imported", "Second imported")
	slices.Sort(want)
	if got := taskTitles(t, b); !slices.Equal(got, want) {
		t.Errorf("after confirming the project holds %v, want %v", got, want)
	}
	if m.transferPlan != nil {
		t.Error("the preview is still drawn after it was written")
	}
}

func TestATaskImportWithRowsInErrorCannotBeWritten(t *testing.T) {
	b := newLiveBoard(t)
	path := filepath.Join(t.TempDir(), "bad.csv")
	if err := os.WriteFile(path, []byte("title,status\n,todo\nfine,nowhere-state\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	m := answer(startTransfer(t, openTransferView(t, b.model), "import tasks"), "path", path)
	m = submitTransfer(t, m)
	if m.transferPlan == nil || m.transferPlan.taskRes == nil || m.transferPlan.taskRes.Failed == 0 {
		t.Fatalf("the file was meant to fail its dry run; plan %+v, error %q", m.transferPlan, m.err)
	}
	if m.confirm.Open() {
		t.Fatal("a file with rows in error was offered for writing")
	}
	transferSection(t, m.Frame(), "rows in error:")
	if next, cmd := m.reduce(pressKey("y")); cmd != nil || next.confirm.Open() {
		t.Error("y offered to write a file with rows in error")
	}
}

func TestAnUnreadablePathIsReportedNotFatal(t *testing.T) {
	// Longer than the status bar, as a temporary directory on macOS is, so the
	// reason is only visible if it comes ahead of the path.
	missing := filepath.Join(t.TempDir(), strings.Repeat("a-directory-that-is-not-there/", 4), "file.csv")
	for _, title := range []string{"import tasks", "import bundle", "import snapshot", "export tasks"} {
		t.Run(title, func(t *testing.T) {
			b := newLiveBoard(t)
			m := answer(startTransfer(t, openTransferView(t, b.model), title), "path", missing)
			m = submitTransfer(t, m)
			if !strings.Contains(m.err, missing) || !strings.Contains(m.err, "no such file") {
				t.Errorf("the error %q does not say the path cannot be reached", m.err)
			}
			if m.view != viewTransfer || m.fatal != nil {
				t.Errorf("a bad path left the view: view %s, fatal %v", viewName(m.view), m.fatal)
			}
			if lipgloss.Width(m.statusBar()) > m.width {
				t.Fatalf("the status bar is wider than the terminal's %d cells:\n%s", m.width, m.statusBar())
			}
			if !strings.Contains(m.statusBar(), "no such file") {
				t.Errorf("the status bar does not carry the error:\n%s", m.statusBar())
			}
		})
	}
}

// transferSpy records every call to the six transfer operations.
type transferSpy struct {
	core.Service
	mu    sync.Mutex
	calls []string
}

func (s *transferSpy) record(method string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, method)
}

func (s *transferSpy) ExportTasks(context.Context, core.TaskExportInput, io.Writer) error {
	s.record("ExportTasks")
	return nil
}

func (s *transferSpy) ImportTasks(context.Context, io.Reader, core.TaskImportInput) (*core.TaskImportResult, error) {
	s.record("ImportTasks")
	return &core.TaskImportResult{}, nil
}

func (s *transferSpy) ExportBundle(context.Context, core.BundleExportInput, io.Writer) error {
	s.record("ExportBundle")
	return nil
}

func (s *transferSpy) ImportBundle(context.Context, io.Reader, core.BundleImportInput) (*core.BundleResult, error) {
	s.record("ImportBundle")
	return &core.BundleResult{}, nil
}

func (s *transferSpy) ExportTo(context.Context, core.ExportInput, io.Writer) error {
	s.record("ExportTo")
	return nil
}

func (s *transferSpy) ImportFrom(context.Context, io.Reader, core.ImportInput) (*core.ImportResult, error) {
	s.record("ImportFrom")
	return &core.ImportResult{}, nil
}

// spyFiles records every file the view asks for.
type spyFiles struct {
	mu      sync.Mutex
	touched []string
}

func (f *spyFiles) note(op, path string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.touched = append(f.touched, op+" "+path)
}

func (f *spyFiles) ReadFile(path string) ([]byte, error) {
	f.note("read", path)
	return nil, os.ErrNotExist
}
func (f *spyFiles) Create(path string) (io.WriteCloser, error) {
	f.note("create", path)
	return nil, os.ErrPermission
}
func (f *spyFiles) Remove(path string) error { f.note("remove", path); return nil }
func (f *spyFiles) Exists(path string) bool  { f.note("stat", path); return false }

func TestTheHostedTransferViewNamesCommandsAndTouchesNothing(t *testing.T) {
	m := boardModel(t)
	spy := &transferSpy{Service: newFakeService()}
	files := &spyFiles{}
	m.svc, m.files, m.sshCommand = spy, files, "ssh acme@tix.example"
	m.width = 200
	m = openTransferView(t, m)

	frame := m.Frame()
	for _, want := range []string{
		"ssh acme@tix.example export tasks -p infra > infra.csv",
		"ssh acme@tix.example import tasks -p infra --dry-run < tasks.csv",
		"ssh acme@tix.example import tasks -p infra < tasks.csv",
		"ssh acme@tix.example bundle export -p infra > infra.bundle",
		"ssh acme@tix.example bundle import --on-collision skip --preview < infra.bundle",
		"ssh acme@tix.example bundle import --on-collision skip < infra.bundle",
		"ssh acme@tix.example export > snapshot.ndjson",
		"ssh acme@tix.example import --mode merge --dry-run < snapshot.ndjson",
		"ssh acme@tix.example import --mode merge < snapshot.ndjson",
	} {
		if !slices.ContainsFunc(strings.Split(frame, "\n"), func(l string) bool { return strings.TrimSpace(l) == want }) {
			t.Errorf("the hosted view has no line reading %q:\n%s", want, frame)
		}
	}
	if strings.Contains(frame, "relative path") {
		t.Errorf("the hosted view still talks about paths:\n%s", frame)
	}

	var cmds []tea.Cmd
	for _, k := range []string{"enter", "y", "down", "enter", "down", "down", "enter", "y"} {
		var cmd tea.Cmd
		m, cmd = m.reduce(pressKey(k))
		cmds = append(cmds, cmd)
		if m.form.Open() || m.confirm.Open() || m.prompt != promptNone {
			t.Fatalf("pressing %s over SSH opened an input asking for a path", k)
		}
	}
	for _, cmd := range cmds {
		if cmd != nil {
			if batch, ok := cmd().(tea.BatchMsg); ok {
				for _, c := range batch {
					if c != nil {
						c()
					}
				}
			}
		}
	}
	if len(spy.calls) > 0 {
		t.Errorf("the hosted view called the service: %v", spy.calls)
	}
	if len(files.touched) > 0 {
		t.Errorf("the hosted view touched files on the server: %v", files.touched)
	}
	footer := m.footerLines()
	if strings.Contains(footer, "start") {
		t.Errorf("the hosted footer offers to start a transfer:\n%s", footer)
	}
}

func TestTheTransferViewFollowsTheReadersScopes(t *testing.T) {
	for _, tc := range []struct {
		name   string
		scopes []core.Scope
		rows   []string
	}{
		{name: "neither export nor import", scopes: []core.Scope{core.ScopeTaskRead, core.ScopeProjectRead}},
		{
			name:   "export only",
			scopes: []core.Scope{core.ScopeTaskRead, core.ScopeProjectRead, core.ScopeExport},
			rows:   []string{"export tasks", "import bundle", "export snapshot"},
		},
		{
			name:   "import only",
			scopes: []core.Scope{core.ScopeTaskRead, core.ScopeProjectRead, core.ScopeImport},
			rows:   []string{"import tasks", "import bundle", "import snapshot"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			actor := &core.Actor{ID: "k", TenantID: "t", Kind: core.ActorAgent, Scopes: tc.scopes}
			m := New(Config{Actor: actor, Access: capability.TUIAccess(actor), Environ: []string{"NO_COLOR=1"}})
			m.width, m.height = 120, 40
			m.project = core.Project{ID: "p1", Key: "infra"}
			offered := slices.ContainsFunc(m.keys.PaletteActions(m.paletteContext()),
				func(e PaletteEntry) bool { return e.Name == "import and export" })
			next, _ := m.reduce(pressKey("V"))

			if len(tc.rows) == 0 {
				if m.canReach(viewTransfer) || offered || next.view == viewTransfer {
					t.Fatalf("a reader with neither scope reached the transfer view (palette %v, view %s)",
						offered, viewName(next.view))
				}
				return
			}
			if !offered || next.view != viewTransfer {
				t.Fatalf("the reader was not offered the transfer view (palette %v, view %s)",
					offered, viewName(next.view))
			}
			var got []string
			for _, s := range next.transferRows() {
				got = append(got, s.title)
			}
			if !slices.Equal(got, tc.rows) {
				t.Errorf("the view offers %v, want %v", got, tc.rows)
			}
			for _, s := range transferSpecs {
				drawn := strings.Contains(next.Frame(), SelectionMarker(false)+pad(s.title, 18)) ||
					strings.Contains(next.Frame(), SelectionMarker(true)+pad(s.title, 18))
				if drawn != slices.Contains(tc.rows, s.title) {
					t.Errorf("%q drawn = %v, want %v", s.title, drawn, !drawn)
				}
			}
		})
	}
}

func TestTheTransferViewSaysWhenNoProjectIsOpen(t *testing.T) {
	m := New(Config{Access: fullAccess(), Actor: fullActor(), Environ: []string{"NO_COLOR=1"}})
	m.width, m.height = 120, 40
	m = openTransferView(t, m)
	if !strings.Contains(m.Frame(), "no project open") {
		t.Errorf("the view does not say no project is open:\n%s", m.Frame())
	}
	m, _ = m.reduce(pressKey("enter"))
	if m.form.Open() || !strings.Contains(m.err, "open a project") {
		t.Errorf("a task export started with no project; form open %v, error %q", m.form.Open(), m.err)
	}
	m, _ = m.reduce(pressKey("esc"))
	if m.view == viewTransfer {
		t.Error("esc did not leave the transfer view")
	}
}

func TestTheTransferViewFitsANarrowTerminal(t *testing.T) {
	for _, hosted := range []bool{false, true} {
		m := boardModel(t)
		if hosted {
			m.sshCommand = "ssh acme@tix.example"
		}
		m.width, m.height = MinWidth, MinHeight
		m = openTransferView(t, m)
		for _, l := range strings.Split(m.Frame(), "\n") {
			if w := len([]rune(l)); w > MinWidth {
				t.Errorf("hosted=%v: a line is %d wide on a %d column terminal: %q", hosted, w, MinWidth, l)
			}
		}
	}
}

func TestTheHostedCommandsScrollIntoViewOnASmallTerminal(t *testing.T) {
	m := boardModel(t)
	m.sshCommand = "ssh acme@tix.example"
	m.width, m.height = MinWidth, MinHeight
	m = openTransferView(t, m)
	if strings.Contains(m.Frame(), "snapshot.ndjson") {
		t.Skip("the terminal is tall enough to show every command at once")
	}
	for range 200 {
		m, _ = m.reduce(pressKey("down"))
	}
	frame := m.Frame()
	if !strings.Contains(strings.ReplaceAll(frame, "\n      ", " "), "snapshot.ndjson") ||
		!strings.Contains(frame, "merge <") {
		t.Errorf("scrolling to the end never shows the last command:\n%s", frame)
	}
	if m.transferOff != m.transferScrollEnd() {
		t.Fatalf("scrolling stopped at %d, short of the end %d", m.transferOff, m.transferScrollEnd())
	}
	m, _ = m.reduce(pressKey("up"))
	if m.transferOff != m.transferScrollEnd()-1 {
		t.Errorf("up after scrolling past the end moved to %d, want one above the end %d",
			m.transferOff, m.transferScrollEnd())
	}
}

func TestABundleOrSnapshotImportPreviewsThenWritesWhatWasExported(t *testing.T) {
	for _, tc := range []struct {
		name, export, imprt, note, done, preview string
	}{
		{"bundle", "export bundle", "import bundle", "a taken key is skipped", "skipped", "preview of bundle.json"},
		{"snapshot", "export snapshot", "import snapshot", "merged into this tenant", "imported", "dry run of snapshot.json"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := newLiveBoard(t)
			path := filepath.Join(t.TempDir(), tc.name+".json")
			m := answer(startTransfer(t, openTransferView(t, b.model), tc.export), "path", path)
			m = submitTransfer(t, m)
			if m.err != "" {
				t.Fatalf("the export failed: %s", m.err)
			}

			m = answer(startTransfer(t, m, tc.imprt), "path", path)
			m = submitTransfer(t, m)
			if m.err != "" {
				t.Fatalf("the preview failed: %s", m.err)
			}
			if !m.confirm.Open() || m.confirm.Kind != confirmImport {
				t.Fatalf("the preview did not ask before writing; confirm %+v", m.confirm)
			}
			if q := m.confirm.Question(); !strings.Contains(q, filepath.Base(path)) {
				t.Errorf("the question %q does not name the file", q)
			}
			if m.confirm.Note != tc.note {
				t.Errorf("the question's note is %q, want %q", m.confirm.Note, tc.note)
			}
			if frame := m.Frame(); !strings.Contains(frame, tc.preview) {
				t.Errorf("the view does not draw the %q:\n%s", tc.preview, frame)
			}

			m, cmd := m.reduce(pressKey("y"))
			if cmd == nil {
				t.Fatal("agreeing to the import started nothing")
			}
			m, _ = m.reduce(cmd())
			if m.err != "" {
				t.Fatalf("the import failed: %s", m.err)
			}
			if !strings.HasPrefix(m.status, "imported "+filepath.Base(path)+": ") || !strings.Contains(m.status, tc.done) {
				t.Errorf("the status %q does not report the import", m.status)
			}
			if m.transferPlan != nil {
				t.Error("the preview is still drawn after it was written")
			}
		})
	}
}

func TestATransferFormRefusesAnEmptyPath(t *testing.T) {
	b := newLiveBoard(t)
	m := answer(startTransfer(t, openTransferView(t, b.model), "import tasks"), "path", "  ")
	m, cmd := m.reduce(pressKey("enter"))
	if cmd != nil {
		t.Fatal("an empty path started a transfer")
	}
	if m.err != "a file path is required" {
		t.Errorf("the error is %q", m.err)
	}
}

func TestResolvePathReadsATildeAsHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home directory to resolve against")
	}
	for typed, want := range map[string]string{
		"~":           home,
		"~/infra.csv": filepath.Join(home, "infra.csv"),
		"/abs/x.csv":  "/abs/x.csv",
	} {
		got, err := resolvePath(typed)
		if err != nil || got != want {
			t.Errorf("resolvePath(%q) = %q, %v; want %q", typed, got, err, want)
		}
	}
	if got, err := resolvePath("rel.csv"); err != nil || !filepath.IsAbs(got) {
		t.Errorf("a relative path was not made absolute: %q, %v", got, err)
	}
}

func TestAPreviewNamesRenamesReasonsAndWarnings(t *testing.T) {
	bundle := strings.Join(lineTexts(bundlePlanLines("x.bundle", &core.BundleResult{
		BundleName: "starter",
		Outcomes: []core.ComponentOutcome{
			{Kind: core.ComponentProject, Key: "infra", Action: "renamed", NewKey: "infra-2"},
			{Kind: core.ComponentTag, Key: "urgent", Action: "skipped", Reason: "already defined"},
		},
		Warnings: []string{"a workflow state was dropped"},
	})), "\n")
	for _, want := range []string{"preview of x.bundle, bundle starter", "1 renamed, 1 skipped", "as infra-2",
		"already defined", "warnings:", "a workflow state was dropped"} {
		if !strings.Contains(bundle, want) {
			t.Errorf("the bundle preview does not say %q:\n%s", want, bundle)
		}
	}
	snap := strings.Join(lineTexts(snapshotPlanLines("s.ndjson", &core.ImportResult{
		Created: map[string]int{"task": 2}, Deleted: map[string]int{"tag": 1},
	})), "\n")
	for _, want := range []string{"dry run of s.ndjson", "2 created, 1 deleted", "2 task", "1 tag"} {
		if !strings.Contains(snap, want) {
			t.Errorf("the snapshot preview does not say %q:\n%s", want, snap)
		}
	}
	if strings.Contains(snap, "warnings:") {
		t.Errorf("a dry run with no warnings drew a warnings heading:\n%s", snap)
	}
}

// lineTexts is the text of each transfer line.
func lineTexts(lines []transferLine) []string {
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		out = append(out, l.text)
	}
	return out
}
