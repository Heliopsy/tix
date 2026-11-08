// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/heliopsy/tix/internal/core"
)

// transferOp names one of the six movements of data in and out of tix.
type transferOp int

// The transfers, in the order the view lists them.
const (
	transferExportTasks transferOp = iota
	transferImportTasks
	transferExportBundle
	transferImportBundle
	transferExportSnapshot
	transferImportSnapshot
)

// transferSpec is one row of the transfer view: what it is called, the
// operation whose authority it needs, and whether it acts on a project.
type transferSpec struct {
	op           transferOp
	title        string
	method       string
	needsProject bool
	summary      string
}

// transferSpecs is the one list of transfers. The rows, the hosted commands and
// the gating all read it, so none of them can offer a transfer the others lack.
var transferSpecs = []transferSpec{
	{transferExportTasks, "export tasks", "ExportTasks", true,
		"the project's tasks to a file: csv, json, ndjson or markdown"},
	{transferImportTasks, "import tasks", "ImportTasks", true,
		"tasks from a csv, json or ndjson file, after a dry run shows the column mapping"},
	{transferExportBundle, "export bundle", "ExportBundle", true,
		"the project as a bundle another tenant can be set up from"},
	{transferImportBundle, "import bundle", "ImportBundle", false,
		"a bundle's components, after a preview of what each becomes"},
	{transferExportSnapshot, "export snapshot", "ExportTo", false,
		"every project of this tenant as an ndjson snapshot"},
	{transferImportSnapshot, "import snapshot", "ImportFrom", false,
		"a snapshot, after a dry run of what it would change"},
}

// imports reports whether a transfer reads a file into tix.
func (op transferOp) imports() bool {
	return op == transferImportTasks || op == transferImportBundle || op == transferImportSnapshot
}

// transferFiles is the file system the local transfer view reads and writes.
// It is a seam so a test can prove the hosted view never touches one.
type transferFiles interface {
	ReadFile(path string) ([]byte, error)
	Create(path string) (io.WriteCloser, error)
	Remove(path string) error
	Exists(path string) bool
}

// osFiles is the machine's own file system.
type osFiles struct{}

// ReadFile reads a file the reader named.
func (osFiles) ReadFile(path string) ([]byte, error) {
	// #nosec G304 -- the path is the reader's own, typed into their own terminal.
	return os.ReadFile(path)
}

// Create opens a file the reader named for writing, truncating it.
func (osFiles) Create(path string) (io.WriteCloser, error) {
	// #nosec G304 -- the path is the reader's own, typed into their own terminal.
	return os.Create(path)
}

// Remove deletes a file an export failed to finish.
func (osFiles) Remove(path string) error { return os.Remove(path) }

// Exists reports whether a path already names something.
func (osFiles) Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// transferJob is an export the reader asked for, held while an overwrite is
// confirmed.
type transferJob struct {
	op       transferOp
	path     string
	label    string
	tasks    core.TaskExportInput
	bundle   core.BundleExportInput
	snapshot core.ExportInput
}

// transferPlan is an import read and checked, waiting for the reader to agree
// to the write. The file is held as read, so what is written is exactly what
// the dry run showed.
type transferPlan struct {
	op      transferOp
	path    string
	project string
	data    []byte

	tasks     core.TaskImportInput
	taskRes   *core.TaskImportResult
	bundle    core.BundleImportInput
	bundleRes *core.BundleResult
	snapshot  core.ImportInput
	snapRes   *core.ImportResult
}

// transferPlanMsg carries a dry run back to the view.
type transferPlanMsg struct {
	plan *transferPlan
	err  error
}

// transferDoneMsg reports a finished export or import.
type transferDoneMsg struct {
	op      transferOp
	summary string
	err     error
}

// hosted reports whether this session runs on a server the client's files are
// not on.
func (m Model) hosted() bool { return m.sshCommand != "" }

// transferRows are the transfers this reader may perform.
func (m Model) transferRows() []transferSpec {
	out := make([]transferSpec, 0, len(transferSpecs))
	for _, s := range transferSpecs {
		if m.mayPerform(s.method) {
			out = append(out, s)
		}
	}
	return out
}

// transferTarget is the project the view acts on: the row under the cursor on
// the project list, and the open project anywhere else.
func (m Model) transferTarget() core.Project {
	if m.view == viewProjects && m.projectSel < len(m.projects) {
		return m.projects[m.projectSel]
	}
	return m.project
}

// openTransfer opens the transfer view on the project in focus.
func (m Model) openTransfer() Model {
	if !m.canReach(viewTransfer) {
		return m
	}
	m.transferProject = m.transferTarget()
	m.transferSel, m.transferOff, m.transferPlan, m.transferJob = 0, 0, nil, nil
	return m.enterView(viewTransfer)
}

// handleTransferKey moves through the transfers and starts one, or scrolls a
// preview and agrees to it. Over SSH there is nothing to start: the view only
// names commands.
func (m Model) handleTransferKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	scrolling := m.hosted() || m.transferPlan != nil
	switch {
	case key.Matches(msg, m.keys.Back):
		if m.transferPlan != nil {
			m.transferPlan, m.transferOff = nil, 0
			return m, nil
		}
		return m.leave(nil)
	case key.Matches(msg, m.keys.Up):
		if scrolling {
			m.transferOff = max(0, m.transferOff-1)
		} else {
			m.transferSel = clamp(m.transferSel-1, 0, len(m.transferRows())-1)
		}
	case key.Matches(msg, m.keys.Down):
		if scrolling {
			m.transferOff = min(m.transferOff+1, m.transferScrollEnd())
		} else {
			m.transferSel = clamp(m.transferSel+1, 0, len(m.transferRows())-1)
		}
	case key.Matches(msg, m.keys.Top):
		m.transferSel, m.transferOff = 0, 0
	case key.Matches(msg, m.keys.Bottom):
		m.transferSel = max(0, len(m.transferRows())-1)
	case m.hosted():
		return m, nil
	case m.transferPlan != nil && key.Matches(msg, m.keys.Agree):
		return m.askImport()
	case m.transferPlan == nil && key.Matches(msg, m.keys.Enter):
		return m.openTransferForm()
	}
	return m, nil
}

// transferScrollEnd is the furthest a preview or a command list scrolls: its
// last line at the foot of the body.
func (m Model) transferScrollEnd() int {
	layout := LayoutFor(m.width, m.height, len(m.columns))
	body, _ := m.transferBody(layout.Width)
	return max(0, len(body)-VisibleRows(layout.BodyHeight, len(body)))
}

// openTransferForm asks for the path and the choices of the selected transfer.
func (m Model) openTransferForm() (Model, tea.Cmd) {
	rows := m.transferRows()
	if m.hosted() || len(rows) == 0 {
		return m, nil
	}
	spec := rows[clamp(m.transferSel, 0, len(rows)-1)]
	if spec.needsProject && m.transferProject.Key == "" {
		m.err = "open a project before you " + spec.title
		return m, nil
	}
	m.transferOp = spec.op
	return m.openForm(TransferForm(spec, m.transferProject.Key))
}

// The file names a transfer suggests, beside the project key when it has one.
const (
	snapshotFile = "snapshot.ndjson"
	bundleExt    = ".bundle"
)

// taskFileExt is the extension a task export in a format is written with.
func taskFileExt(f core.TaskFormat) string {
	if f == core.TaskFormatMarkdown {
		return ".md"
	}
	return "." + string(f)
}

// mappingAsMapped is the mode answer that leaves the mode to the mapping file,
// or to the default upsert when there is none.
const mappingAsMapped = "as mapped"

// TransferForm gathers what one transfer needs: always a path, and the choices
// the operation offers.
func TransferForm(spec transferSpec, project string) Form {
	path := func(seed string) FormField {
		return FormField{Key: "path", Label: "file", Kind: FieldText, Value: seed, Limit: 4096}
	}
	form := Form{Kind: formTransfer, Title: spec.title}
	switch spec.op {
	case transferExportTasks:
		formats := make([]string, 0, len(core.TaskFormats))
		for _, f := range core.TaskFormats {
			formats = append(formats, string(f))
		}
		form.Note = "an untouched file name follows the format"
		form.Fields = []FormField{
			{Key: "format", Label: "format", Options: formats, Value: string(core.TaskFormatCSV)},
			path(project + taskFileExt(core.TaskFormatCSV)),
		}
	case transferImportTasks:
		modes := []string{mappingAsMapped}
		for _, mode := range core.TaskImportModes {
			modes = append(modes, string(mode))
		}
		form.Note = "a dry run shows which column fills which field before anything is written"
		form.Fields = []FormField{
			path(""),
			{Key: "mapping", Label: "mapping file", Kind: FieldText, Limit: 4096},
			{Key: "mode", Label: "mode", Options: modes, Value: mappingAsMapped},
		}
	case transferExportBundle:
		form.Fields = []FormField{path(project + bundleExt)}
	case transferImportBundle:
		policies := make([]string, 0, len(core.CollisionPolicies))
		for _, p := range core.CollisionPolicies {
			policies = append(policies, string(p))
		}
		form.Note = "a preview shows what each component becomes before anything is written"
		form.Fields = []FormField{
			path(""),
			{Key: "collision", Label: "key taken", Options: policies, Value: string(core.CollisionSkip)},
		}
	case transferExportSnapshot:
		form.Fields = []FormField{
			path(snapshotFile),
			{Key: "comments", Label: "comments", Options: switchOptions(), Value: noValue},
			{Key: "artifacts", Label: "artifacts", Options: switchOptions(), Value: noValue},
		}
	case transferImportSnapshot:
		modes := make([]string, 0, len(core.ImportModes))
		for _, mode := range core.ImportModes {
			modes = append(modes, string(mode))
		}
		form.Note = "a dry run shows what would change before anything is written"
		form.Fields = []FormField{
			path(""),
			{Key: "mode", Label: "mode", Options: modes, Value: string(core.ImportMerge)},
		}
	}
	return form
}

// resolvePath turns what the reader typed into the absolute path it names, with
// a leading ~ read as their home directory.
func resolvePath(typed string) (string, error) {
	typed = strings.TrimSpace(typed)
	if typed == "" {
		return "", errors.New("a file path is required")
	}
	if typed == "~" || strings.HasPrefix(typed, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("finding your home directory: %w", err)
		}
		typed = filepath.Join(home, strings.TrimPrefix(typed, "~"))
	}
	return filepath.Abs(typed)
}

// applyTransferForm starts the transfer the answered form describes.
func (m Model) applyTransferForm(form Form) (Model, tea.Cmd) {
	if m.hosted() {
		return m, nil
	}
	op, project := m.transferOp, m.transferProject.Key
	typed := form.Value("path")
	if op == transferExportTasks && typed == project+taskFileExt(core.TaskFormatCSV) {
		typed = project + taskFileExt(core.TaskFormat(form.Value("format")))
	}
	path, err := resolvePath(typed)
	if err != nil {
		m.err = err.Error()
		return m, nil
	}
	if op.imports() {
		return m.startImport(op, path, form)
	}
	job := transferJob{op: op, path: path}
	switch op {
	case transferExportTasks:
		job.label = project + "'s tasks"
		job.tasks = core.TaskExportInput{ProjectRef: project, Format: core.TaskFormat(form.Value("format"))}.Normalize()
	case transferExportBundle:
		job.label = project + " as a bundle"
		job.bundle = core.BundleExportInput{ProjectRefs: []string{project}}
	case transferExportSnapshot:
		job.label = "a snapshot"
		job.snapshot = core.ExportInput{IncludeComments: form.Yes("comments"), IncludeArtifacts: form.Yes("artifacts")}
	}
	if m.files.Exists(path) {
		m.transferJob = &job
		return m.askConfirm(Confirm{Kind: confirmOverwrite, Target: path, Note: "with " + job.label},
			"there is no file named to write")
	}
	m.err = ""
	return m, m.runExport(job)
}

// startImport reads the named file and dry runs it.
func (m Model) startImport(op transferOp, path string, form Form) (Model, tea.Cmd) {
	plan := transferPlan{op: op, path: path, project: m.transferProject.Key}
	mapping := ""
	switch op {
	case transferImportTasks:
		mode := form.Value("mode")
		if mode == mappingAsMapped {
			mode = ""
		}
		plan.tasks = core.TaskImportInput{ProjectRef: plan.project, Filename: filepath.Base(path),
			Mode: core.TaskImportMode(mode)}
		if typed := strings.TrimSpace(form.Value("mapping")); typed != "" {
			resolved, err := resolvePath(typed)
			if err != nil {
				m.err = err.Error()
				return m, nil
			}
			mapping = resolved
		}
	case transferImportBundle:
		plan.bundle = core.BundleImportInput{OnCollision: core.CollisionPolicy(form.Value("collision"))}
	case transferImportSnapshot:
		plan.snapshot = core.ImportInput{Mode: core.ImportMode(form.Value("mode"))}
	}
	m.err = ""
	return m, m.planImport(plan, mapping)
}

// fileError drops the operation and path an os error repeats, since the
// message around it names the path already.
func fileError(err error) error {
	var pe *fs.PathError
	if errors.As(err, &pe) {
		return pe.Err
	}
	return err
}

// planImport reads the file and asks the service what importing it would do,
// writing nothing.
func (m Model) planImport(plan transferPlan, mappingPath string) tea.Cmd {
	if m.svc == nil {
		return nil
	}
	svc, ctx, files := m.svc, m.ctx, m.files
	return func() tea.Msg {
		data, err := files.ReadFile(plan.path)
		if err != nil {
			return transferPlanMsg{err: fmt.Errorf("reading %s: %w", plan.path, fileError(err))}
		}
		plan.data = data
		if mappingPath != "" {
			raw, err := files.ReadFile(mappingPath)
			if err != nil {
				return transferPlanMsg{err: fmt.Errorf("reading %s: %w", mappingPath, fileError(err))}
			}
			plan.tasks.Mapping = string(raw)
		}
		switch plan.op {
		case transferImportTasks:
			in := plan.tasks
			in.DryRun = true
			plan.taskRes, err = svc.ImportTasks(ctx, bytes.NewReader(data), in)
		case transferImportBundle:
			in := plan.bundle
			in.Preview = true
			plan.bundleRes, err = svc.ImportBundle(ctx, bytes.NewReader(data), in)
		case transferImportSnapshot:
			in := plan.snapshot
			in.DryRun = true
			plan.snapRes, err = svc.ImportFrom(ctx, bytes.NewReader(data), in)
		}
		if err != nil {
			return transferPlanMsg{err: err}
		}
		return transferPlanMsg{plan: &plan}
	}
}

// onTransferPlan shows a dry run and asks whether to write it.
func (m Model) onTransferPlan(msg transferPlanMsg) (Model, tea.Cmd) {
	if msg.err != nil {
		m.err = "import failed: " + msg.err.Error()
		return m, nil
	}
	m.transferPlan, m.transferOff = msg.plan, 0
	if m.transferPlan.blocked() != "" {
		m.err = m.transferPlan.blocked()
		return m, nil
	}
	return m.askImport()
}

// askImport puts the write of the previewed file to the reader.
func (m Model) askImport() (Model, tea.Cmd) {
	p := m.transferPlan
	if p == nil {
		return m, nil
	}
	if reason := p.blocked(); reason != "" {
		m.err = reason
		return m, nil
	}
	return m.askConfirm(Confirm{Kind: confirmImport, Target: p.target(), Note: p.note()},
		"there is no file named to import")
}

// blocked says why a previewed import cannot be written, or nothing.
func (p transferPlan) blocked() string {
	if p.op == transferImportTasks && p.taskRes != nil && p.taskRes.Failed > 0 {
		return strconv.Itoa(p.taskRes.Failed) + " row(s) in error; nothing can be imported until the file or the mapping is fixed"
	}
	return ""
}

// target names what the import confirmation is about.
func (p transferPlan) target() string {
	name := filepath.Base(p.path)
	if p.op == transferImportTasks && p.taskRes != nil {
		return strconv.Itoa(p.taskRes.Rows) + " rows of " + name + " into " + p.project
	}
	return name
}

// note says what the import will do beyond its subject.
func (p transferPlan) note() string {
	switch p.op {
	case transferImportTasks:
		return "with the column mapping shown"
	case transferImportBundle:
		return "a taken key is " + map[core.CollisionPolicy]string{
			core.CollisionSkip: "skipped", core.CollisionRename: "renamed", core.CollisionReplace: "replaced",
		}[p.bundle.OnCollision]
	case transferImportSnapshot:
		if p.snapshot.Mode == core.ImportReplace {
			return "replacing what the imported projects hold and the snapshot does not"
		}
		return "merged into this tenant"
	}
	return ""
}

// commitImport writes the previewed file, the bytes the dry run read.
func (m Model) commitImport() tea.Cmd {
	if m.svc == nil || m.transferPlan == nil {
		return nil
	}
	svc, ctx, p := m.svc, m.ctx, *m.transferPlan
	return func() tea.Msg {
		r := bytes.NewReader(p.data)
		name := filepath.Base(p.path)
		switch p.op {
		case transferImportTasks:
			in := p.tasks
			if p.taskRes != nil && p.taskRes.Mapping != "" {
				in.Mapping = p.taskRes.Mapping
			}
			res, err := svc.ImportTasks(ctx, r, in)
			if err == nil && res.Failed > 0 {
				err = core.Invalid("%d row(s) in error; nothing was imported", res.Failed)
			}
			if err != nil {
				return transferDoneMsg{op: p.op, err: err}
			}
			return transferDoneMsg{op: p.op, summary: fmt.Sprintf("imported %d rows of %s into %s: %d created, %d updated, %d skipped",
				res.Rows, name, p.project, res.Created, res.Updated, res.Skipped)}
		case transferImportBundle:
			res, err := svc.ImportBundle(ctx, r, p.bundle)
			if err != nil {
				return transferDoneMsg{op: p.op, err: err}
			}
			return transferDoneMsg{op: p.op, summary: "imported " + name + ": " + bundleCounts(res)}
		default:
			res, err := svc.ImportFrom(ctx, r, p.snapshot)
			if err != nil {
				return transferDoneMsg{op: p.op, err: err}
			}
			return transferDoneMsg{op: p.op, summary: "imported " + name + ": " + snapshotCounts(res)}
		}
	}
}

// runExport writes an export to its file, removing a file it could not finish.
func (m Model) runExport(job transferJob) tea.Cmd {
	if m.svc == nil {
		return nil
	}
	svc, ctx, files := m.svc, m.ctx, m.files
	return func() tea.Msg {
		write := func(w io.Writer) error {
			switch job.op {
			case transferExportTasks:
				return svc.ExportTasks(ctx, job.tasks, w)
			case transferExportBundle:
				return svc.ExportBundle(ctx, job.bundle, w)
			default:
				return svc.ExportTo(ctx, job.snapshot, w)
			}
		}
		n, err := writeFile(files, job.path, write)
		if err != nil {
			return transferDoneMsg{op: job.op, err: err}
		}
		return transferDoneMsg{op: job.op, summary: fmt.Sprintf("exported %s to %s (%d bytes)", job.label, job.path, n)}
	}
}

// countingWriter counts what passes through it, for the status line.
type countingWriter struct {
	w io.Writer
	n int64
}

func (c *countingWriter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	c.n += int64(n)
	return n, err
}

// writeFile creates path and hands it to write, removing it when the write
// fails so a half export is never left looking like a whole one.
func writeFile(files transferFiles, path string, write func(io.Writer) error) (int64, error) {
	f, err := files.Create(path)
	if err != nil {
		return 0, fmt.Errorf("creating %s: %w", path, fileError(err))
	}
	cw := &countingWriter{w: f}
	if err := write(cw); err != nil {
		_ = f.Close()
		_ = files.Remove(path)
		return 0, err
	}
	if err := f.Close(); err != nil {
		return 0, fmt.Errorf("writing %s: %w", path, fileError(err))
	}
	return cw.n, nil
}

// onTransferDone reports a finished transfer and reloads what it changed.
func (m Model) onTransferDone(msg transferDoneMsg) (Model, tea.Cmd) {
	verb := "export"
	if msg.op.imports() {
		verb = "import"
	}
	if msg.err != nil {
		m.err = verb + " failed: " + msg.err.Error()
		return m, nil
	}
	m.status, m.err = msg.summary, ""
	m.transferPlan, m.transferOff = nil, 0
	if !msg.op.imports() {
		return m, nil
	}
	if m.project.ID != "" {
		return m, tea.Batch(m.loadProjects(), m.loadTasks())
	}
	return m, m.loadProjects()
}

// bundleCounts sums a bundle import's outcomes by what happened to each.
func bundleCounts(r *core.BundleResult) string {
	counts := map[string]int{}
	for _, o := range r.Outcomes {
		counts[string(o.Action)]++
	}
	return countList(counts, "no components")
}

// snapshotCounts sums a snapshot import's record counts by what happened.
func snapshotCounts(r *core.ImportResult) string {
	sum := func(m map[string]int) int {
		n := 0
		for _, v := range m {
			n += v
		}
		return n
	}
	return countList(map[string]int{
		"created": sum(r.Created), "updated": sum(r.Updated),
		"skipped": sum(r.Skipped), "deleted": sum(r.Deleted),
	}, "no records")
}

// countList renders counts as "2 created, 1 skipped", in name order, leaving
// out the zeros.
func countList(counts map[string]int, none string) string {
	names := make([]string, 0, len(counts))
	for name, n := range counts {
		if n > 0 {
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		return none
	}
	sort.Strings(names)
	parts := make([]string, 0, len(names))
	for _, name := range names {
		parts = append(parts, strconv.Itoa(counts[name])+" "+name)
	}
	return strings.Join(parts, ", ")
}

// transferCommands are the commands a hosted session names for one transfer,
// each doing it through standard input or output. A key the session does not
// know is left as KEY for the reader to fill in.
func transferCommands(ssh, project string, op transferOp) []string {
	if project == "" {
		project = "KEY"
	}
	bundle := project + bundleExt
	switch op {
	case transferExportTasks:
		return []string{ssh + " export tasks -p " + project + " > " + project + ".csv"}
	case transferImportTasks:
		return []string{
			ssh + " import tasks -p " + project + " --dry-run < tasks.csv",
			ssh + " import tasks -p " + project + " < tasks.csv",
		}
	case transferExportBundle:
		return []string{ssh + " bundle export -p " + project + " > " + bundle}
	case transferImportBundle:
		return []string{
			ssh + " bundle import --on-collision skip --preview < " + bundle,
			ssh + " bundle import --on-collision skip < " + bundle,
		}
	case transferExportSnapshot:
		return []string{ssh + " export > " + snapshotFile}
	default:
		return []string{
			ssh + " import --mode merge --dry-run < " + snapshotFile,
			ssh + " import --mode merge < " + snapshotFile,
		}
	}
}

// transferLine is one rendered row of the transfer view, tagged with how to
// draw it.
type transferLine struct {
	text     string
	heading  bool
	dim      bool
	alert    bool
	selected bool
}

// transferBody is every line the view would draw and the one the cursor is on.
func (m Model) transferBody(width int) ([]transferLine, int) {
	lines := []transferLine{{text: "  transfer", heading: true}}
	project := "  project: " + m.transferProject.Key
	if m.transferProject.Key == "" {
		project = "  no project open: task and bundle transfers act on the project a board is open on"
	} else if m.transferProject.Name != "" && m.transferProject.Name != m.transferProject.Key {
		project += " " + m.transferProject.Name
	}
	lines = append(lines, transferLine{text: project, dim: true}, transferLine{})
	switch {
	case m.hosted():
		return append(lines, m.hostedLines(width)...), m.transferOff
	case m.transferPlan != nil:
		return append(lines, planLines(*m.transferPlan)...), m.transferOff
	}
	rows := m.transferRows()
	if len(rows) == 0 {
		for _, l := range (EmptyState{Title: "Nothing here is yours to move.",
			Hint: "Exports need the export scope and imports the import scope."}).Lines() {
			lines = append(lines, transferLine{text: l})
		}
		return lines, 0
	}
	selected := 0
	for i, spec := range rows {
		row := "  " + SelectionMarker(i == m.transferSel) + pad(spec.title, 18) + spec.summary
		line := transferLine{text: row, selected: i == m.transferSel}
		if spec.needsProject && m.transferProject.Key == "" {
			line.text += " (open a project first)"
			line.dim = !line.selected
		}
		if line.selected {
			selected = len(lines)
		}
		lines = append(lines, line)
	}
	lines = append(lines, transferLine{},
		transferLine{text: "  a relative path is read from the directory tix started in; ~ is your home", dim: true})
	return lines, selected
}

// hostedLines name, for every transfer this reader may perform, the commands
// that do it from the reader's own shell. They are wrapped rather than cut,
// because a command missing its tail is a command that does something else.
func (m Model) hostedLines(width int) []transferLine {
	var lines []transferLine
	for _, part := range Wrap("over SSH this server cannot reach your files; run these from your own shell:", max(1, width-2)) {
		lines = append(lines, transferLine{text: "  " + part})
	}
	for _, spec := range m.transferRows() {
		lines = append(lines, transferLine{}, transferLine{text: "  " + spec.title, heading: true})
		for _, cmd := range transferCommands(m.sshCommand, m.transferProject.Key, spec.op) {
			for i, part := range Wrap(cmd, max(1, width-6)) {
				indent := "    "
				if i > 0 {
					indent = "      "
				}
				lines = append(lines, transferLine{text: indent + part})
			}
		}
	}
	return lines
}

// planLines render a dry run the way `tix import tasks --dry-run` reports it:
// the counts, which column fills which field, the first rows as they would be
// written and every row in error.
func planLines(p transferPlan) []transferLine {
	name := filepath.Base(p.path)
	var out []transferLine
	switch p.op {
	case transferImportTasks:
		out = taskPlanLines(name, p.taskRes)
	case transferImportBundle:
		out = bundlePlanLines(name, p.bundleRes)
	case transferImportSnapshot:
		out = snapshotPlanLines(name, p.snapRes)
	}
	out = append(out, transferLine{})
	if reason := p.blocked(); reason != "" {
		return append(out, transferLine{text: "  " + reason, alert: true})
	}
	return append(out, transferLine{text: "  nothing is written until you confirm; esc discards this preview", dim: true})
}

func taskPlanLines(name string, r *core.TaskImportResult) []transferLine {
	if r == nil {
		return nil
	}
	source := ""
	if r.Source != "" {
		source = ", looks like " + r.Source
	}
	out := []transferLine{
		{text: fmt.Sprintf("  dry run: %d rows of %s read as %s%s", r.Rows, name, r.Format, source), heading: true},
		{text: fmt.Sprintf("  %d create, %d update, %d skip, %d in error", r.Created, r.Updated, r.Skipped, r.Failed)},
	}
	if len(r.Columns) > 0 {
		out = append(out, transferLine{}, transferLine{text: "  columns, and the field each fills:", heading: true})
		for _, c := range r.Columns {
			note := ""
			if c.Suggested {
				note = "suggested"
			}
			row := "    " + pad(c.Header, 20) + "→ " + pad(c.Target, 14) + pad(note, 11) + strings.Join(c.Sample, ", ")
			out = append(out, transferLine{text: strings.TrimRight(row, " "), dim: c.Target == "ignore"})
		}
		if !r.Confident {
			out = append(out, transferLine{text: "  some columns are guesses; check them, or load a mapping file", alert: true})
		}
	}
	if len(r.Preview) > 0 {
		out = append(out, transferLine{}, transferLine{text: "  the first rows as they would be written:", heading: true})
		for _, p := range r.Preview {
			note := strings.TrimSpace(strings.Join(append([]string{p.Reason}, p.Errors...), " "))
			row := "    " + pad("line "+strconv.Itoa(p.Line), 10) + pad(p.Action, 8) + pad(p.Ref, 12) + p.Title
			if p.Status != "" {
				row += "  " + p.Status
			}
			if note != "" {
				row += "  " + note
			}
			out = append(out, transferLine{text: row, alert: p.Action == core.ImportRowError})
		}
	}
	if len(r.Errors) > 0 {
		out = append(out, transferLine{}, transferLine{text: "  rows in error:", heading: true})
		for _, e := range r.Errors {
			out = append(out, transferLine{text: fmt.Sprintf("    line %d: %s", e.Line, e.Message), alert: true})
		}
	}
	return out
}

func bundlePlanLines(name string, r *core.BundleResult) []transferLine {
	if r == nil {
		return nil
	}
	head := "  preview of " + name
	if r.BundleName != "" {
		head += ", bundle " + r.BundleName
	}
	out := []transferLine{{text: head, heading: true}, {text: "  " + bundleCounts(r)}}
	if len(r.Outcomes) > 0 {
		out = append(out, transferLine{})
	}
	for _, o := range r.Outcomes {
		row := "    " + pad(string(o.Kind), 18) + pad(o.Key, 20) + string(o.Action)
		if o.NewKey != "" {
			row += " as " + o.NewKey
		}
		if o.Reason != "" {
			row += "  " + o.Reason
		}
		out = append(out, transferLine{text: row})
	}
	return append(out, warningLines(r.Warnings)...)
}

func snapshotPlanLines(name string, r *core.ImportResult) []transferLine {
	if r == nil {
		return nil
	}
	out := []transferLine{{text: "  dry run of " + name, heading: true}, {text: "  " + snapshotCounts(r)}}
	for _, part := range []struct {
		what   string
		counts map[string]int
	}{{"created", r.Created}, {"updated", r.Updated}, {"skipped", r.Skipped}, {"deleted", r.Deleted}} {
		if c := countList(part.counts, ""); c != "" {
			out = append(out, transferLine{text: "    " + pad(part.what, 10) + c})
		}
	}
	return append(out, warningLines(r.Warnings)...)
}

func warningLines(warnings []string) []transferLine {
	if len(warnings) == 0 {
		return nil
	}
	out := []transferLine{{}, {text: "  warnings:", heading: true}}
	for _, w := range warnings {
		out = append(out, transferLine{text: "    " + w, alert: true})
	}
	return out
}

// transferLines renders the view, scrolled so the selected row, or the part
// of a preview or a command list being read, is on screen.
func (m Model) transferLines(layout Layout) []string {
	raw, at := m.transferBody(layout.Width)
	rows := VisibleRows(layout.BodyHeight, len(raw))
	offset := ScrollWindow(m.transferOff, at, rows, len(raw))
	if m.hosted() || m.transferPlan != nil {
		offset = clamp(m.transferOff, 0, max(0, len(raw)-rows))
	}
	out := make([]string, 0, rows+1)
	for _, l := range raw[offset:min(len(raw), offset+rows)] {
		out = append(out, m.fit(m.transferStyle(l)))
	}
	if hint := ScrollHint(offset, rows, len(raw)); hint != "" {
		out = append(out, m.theme.Dim.Render("  "+hint))
	}
	return out
}

// transferStyle draws one line of the view by what it is.
func (m Model) transferStyle(l transferLine) string {
	switch {
	case l.selected:
		return m.selection().Render(Truncate(l.text, m.width))
	case l.heading:
		return m.theme.Header.Render(l.text)
	case l.alert:
		return m.theme.Error.Render(l.text)
	case l.dim:
		return m.theme.Dim.Render(l.text)
	default:
		return l.text
	}
}
