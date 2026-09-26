// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/capability"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/output"
)

// setupSection returns only the lines of one section of the project screen, from
// its heading to the blank line that ends it.
//
// Every assertion about the screen goes through here rather than through the
// frame. The three sections use the same words as each other and as the footer: a
// form titled "field severity" carries the word severity, the fields section
// carries it too, and "key" is a row of both the project and the workflow. An
// assertion that read the frame would pass on whichever of them happened to
// match, which is exactly how a guard passes while the thing it names is broken.
func setupSection(t *testing.T, frame, heading string) string {
	t.Helper()
	lines := strings.Split(frame, "\n")
	start := -1
	for i, l := range lines {
		if strings.HasPrefix(strings.TrimSpace(l), heading) {
			start = i
			break
		}
	}
	if start < 0 {
		t.Fatalf("the project screen draws no %q section:\n%s", heading, frame)
	}
	for i := start + 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "" {
			return strings.Join(lines[start:i], "\n")
		}
	}
	return strings.Join(lines[start:], "\n")
}

// setupRow returns the one row of a section that carries a named label, so an
// assertion about the project's key cannot be satisfied by the workflow's.
func setupRow(t *testing.T, frame, heading, label string) string {
	t.Helper()
	section := setupSection(t, frame, heading)
	var found []string
	for _, l := range strings.Split(section, "\n") {
		if strings.HasPrefix(strings.TrimSpace(l), label+" ") {
			found = append(found, l)
		}
	}
	if len(found) != 1 {
		t.Fatalf("the %q section draws %d rows for %q, want exactly one:\n%s",
			heading, len(found), label, section)
	}
	return found[0]
}

// setupProject is the project the screen tests render.
func setupProject() core.Project {
	return core.Project{ID: "p1", Key: "infra", Name: "Infrastructure",
		Description: "the racks", WorkflowID: "w-1", Color: core.ColorBlue, Icon: "▲"}
}

// setupWorkflow is the state machine the project runs on.
func setupWorkflow() core.Workflow {
	return core.Workflow{ID: "w-1", Key: "standard", Name: "Standard",
		Definition: *testWorkflow()}
}

// setupFields are the custom field definitions the project's tasks carry.
func setupFields() []core.FieldDef {
	return []core.FieldDef{
		{Key: "severity", Label: "Severity", Type: core.FieldEnum, Required: true,
			EnumOptions: []string{"low", "high"}, Position: 3},
		{Key: "owner", Label: "Owner", Type: core.FieldString},
	}
}

// setupService is a service the project screen can be driven against.
func setupService() *fakeService {
	svc := newFakeService()
	p := setupProject()
	svc.projects = []core.Project{p}
	svc.workflows = []core.Workflow{setupWorkflow()}
	svc.fieldDefs = setupFields()
	svc.tasks = []core.Task{task("a", "todo", 1, core.PriorityNormal)}
	return svc
}

// setupModel opens the project screen the way a reader does: from the board, with
// the key, draining the read the key asked for.
func setupModel(t *testing.T) (Model, *fakeService) {
	t.Helper()
	m := boardModel(t)
	svc := setupService()
	m.svc = svc
	m, cmd := m.reduce(pressKey("w"))
	m, _ = m.reduce(run(t, cmd))
	if m.view != viewProject {
		t.Fatalf("w did not open the project screen; view = %v", m.view)
	}
	return m, svc
}

func TestTheProjectScreenStatesTheProjectItFetched(t *testing.T) {
	m, svc := setupModel(t)
	frame := m.View()

	if got := setupRow(t, frame, "project:", "key"); !strings.Contains(got, "infra") {
		t.Errorf("the project's key row says %q", got)
	}
	if got := setupRow(t, frame, "project:", "description"); !strings.Contains(got, "the racks") {
		t.Errorf("the description row says %q", got)
	}
	if got := setupRow(t, frame, "project:", "colour"); !strings.Contains(got, "blue") {
		t.Errorf("the colour row says %q", got)
	}
	if got := setupRow(t, frame, "project:", "state"); !strings.Contains(got, "active") {
		t.Errorf("the state row says %q", got)
	}
	// The workflow is asked for by the identifier the project carries, which is
	// the read that makes this one call rather than a listing of every workflow.
	if !slices.Contains(svc.workflowAsked, "w-1") {
		t.Errorf("the workflow was not fetched by the project's own identifier: %v", svc.workflowAsked)
	}
}

func TestTheProjectScreenDrawsTheStateMachineItCannotEdit(t *testing.T) {
	m, _ := setupModel(t)
	frame := m.View()
	section := setupSection(t, frame, "workflow:")

	if !strings.Contains(section, "Standard") {
		t.Errorf("the workflow section does not name the workflow:\n%s", section)
	}
	if got := setupRow(t, frame, "workflow:", "initial"); !strings.Contains(got, "todo") {
		t.Errorf("the initial row says %q", got)
	}
	for _, want := range []string{"todo", "doing", "review", "done"} {
		if !strings.Contains(section, want) {
			t.Errorf("the workflow section omits the state %q:\n%s", want, section)
		}
	}
	if !strings.Contains(section, "review -> done") {
		t.Errorf("the workflow section omits an edge:\n%s", section)
	}
	if !strings.Contains(section, "terminal") {
		t.Errorf("the workflow section does not mark its terminal state:\n%s", section)
	}
	// The screen is where a reader finds out a workflow is not edited here.
	if !strings.Contains(section, "tix workflow put") {
		t.Errorf("the workflow section does not say where a workflow is edited:\n%s", section)
	}
}

func TestTheProjectScreenSaysWhyAWorkflowIsMissing(t *testing.T) {
	m := boardModel(t)
	svc := setupService()
	svc.workflowErr = core.Forbidden("workflow read is not permitted")
	m.svc = svc
	m, cmd := m.reduce(pressKey("w"))
	m, _ = m.reduce(run(t, cmd))

	section := setupSection(t, m.View(), "workflow:")
	if !strings.Contains(section, "not shown") || !strings.Contains(section, "not permitted") {
		t.Errorf("a refused workflow is not explained:\n%s", section)
	}
	// The rest of the screen still arrives, which is the point of not failing
	// the whole read over one refusal.
	if got := setupRow(t, m.View(), "project:", "key"); !strings.Contains(got, "infra") {
		t.Errorf("a refused workflow lost the project rows: %q", got)
	}
}

func TestTheProjectScreenListsItsFieldDefinitions(t *testing.T) {
	m, _ := setupModel(t)
	frame := m.View()

	if got := setupRow(t, frame, "custom fields (", "severity"); !strings.Contains(got, "enum") ||
		!strings.Contains(got, "required") || !strings.Contains(got, "low / high") {
		t.Errorf("the severity row says %q", got)
	}
	if got := setupRow(t, frame, "custom fields (", "owner"); strings.Contains(got, "required") {
		t.Errorf("an optional field is drawn as required: %q", got)
	}
}

func TestAProjectWithNothingSetSaysSoRatherThanDrawingBlanks(t *testing.T) {
	lines := ProjectView(ProjectState{Project: core.Project{Key: "bare"}})
	frame := renderLines(lines)
	if got := setupRow(t, frame, "project:", "description"); !strings.Contains(got, "none") {
		t.Errorf("an empty description draws %q", got)
	}
	if !strings.Contains(setupSection(t, frame, "custom fields ("), "none defined") {
		t.Error("a project with no field definitions does not say so")
	}
}

// renderLines is the project screen's pure output as one frame, for the tests
// that drive the renderer rather than the terminal.
func renderLines(lines []ProjectLine) string {
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		out = append(out, l.Text)
	}
	return strings.Join(out, "\n")
}

func TestAnArchivedProjectNamesWhenItWasArchived(t *testing.T) {
	when := time.Date(2026, 5, 6, 7, 8, 0, 0, time.UTC)
	p := setupProject()
	p.ArchivedAt = &when
	style, err := output.NewTimeStyle(output.TimeISO, "UTC")
	if err != nil {
		t.Fatalf("building a time style: %v", err)
	}
	frame := renderLines(ProjectView(ProjectState{Project: p, TimeStyle: style}))
	if got := setupRow(t, frame, "project:", "state"); !strings.Contains(got, "archived") ||
		!strings.Contains(got, "2026") {
		t.Errorf("the state row says %q", got)
	}
}

func TestColourOptionsOfferTheWholePaletteAndAWayBack(t *testing.T) {
	options := ColourOptions()
	if options[0] != noColour {
		t.Errorf("the colour list does not start with the absence of one: %v", options)
	}
	if len(options) != len(core.ProjectColors())+1 {
		t.Errorf("the colour list holds %d entries for a palette of %d",
			len(options), len(core.ProjectColors()))
	}
	if ColourValue(noColour) != "" {
		t.Error("choosing no colour does not clear one")
	}
	if ColourValue("blue") != "blue" {
		t.Error("a chosen colour is not passed through")
	}
}

func TestEnumIsOfferedOnlyToAFieldThatAlreadyIsOne(t *testing.T) {
	fresh := FieldTypeOptions("")
	if slices.Contains(fresh, string(core.FieldEnum)) {
		t.Errorf("a new field is offered enum, whose options it cannot gather: %v", fresh)
	}
	existing := FieldTypeOptions(core.FieldEnum)
	if !slices.Contains(existing, string(core.FieldEnum)) {
		t.Errorf("an enum field cannot stay an enum: %v", existing)
	}
	if len(existing) != len(fresh)+1 {
		t.Errorf("the two lists differ by more than enum: %v against %v", existing, fresh)
	}
}

func TestARedefinitionKeepsWhatTheFormNeverAsked(t *testing.T) {
	base := setupFields()[0]
	in := FieldDefUpdate("severity", "", base, core.FieldEnum, false)
	if len(in.EnumOptions) != 2 {
		t.Errorf("a redefinition dropped the enum options: %+v", in)
	}
	if in.Position != 3 {
		t.Errorf("a redefinition dropped the position: %d", in.Position)
	}
	if in.Label != "Severity" {
		t.Errorf("a redefinition dropped the label: %q", in.Label)
	}
	if in.Required {
		t.Error("a redefinition ignored the answer the form gathered")
	}
	// Narrowed away from enum, the options go with it: a string field carrying
	// enum options is refused by the input's own validation.
	narrowed := FieldDefUpdate("severity", "", base, core.FieldString, false)
	if len(narrowed.EnumOptions) != 0 {
		t.Errorf("a field narrowed off enum kept its options: %+v", narrowed)
	}
	if err := narrowed.Validate(); err != nil {
		t.Errorf("the input a redefinition builds is refused: %v", err)
	}
}

func TestSplitFieldEntrySeparatesTheKeyFromTheLabel(t *testing.T) {
	key, label := SplitFieldEntry("  severity  How bad it is ")
	if key != "severity" || label != "How bad it is" {
		t.Errorf("key = %q label = %q", key, label)
	}
	if key, label := SplitFieldEntry("  "); key != "" || label != "" {
		t.Errorf("an empty entry named %q and %q", key, label)
	}
}

func TestTheEditFormHidesTheRowsItsOwnAnswerMakesIrrelevant(t *testing.T) {
	form := ProjectEditForm(setupProject(), []string{"standard", "fast"})
	if form.Value(attrColour) != "" {
		t.Errorf("the colour row is shown while the attribute is the name: %+v", form.Visible())
	}
	for range 3 {
		form = form.Cycle(1)
	}
	if form.Value("attribute") != attrColour {
		t.Fatalf("cycling reached %q", form.Value("attribute"))
	}
	if form.Value(attrColour) != "blue" {
		t.Errorf("the colour row does not open on the colour the project has: %q", form.Value(attrColour))
	}
	if form.Value(attrWorkflow) != "" {
		t.Error("the workflow row is shown while the attribute is the colour")
	}
}

func TestOneWorkflowIsNoChoiceToOffer(t *testing.T) {
	form := ProjectEditForm(setupProject(), []string{"standard"})
	for _, f := range form.Fields {
		if f.Key == attrWorkflow {
			t.Fatal("a tenant with one workflow is offered a workflow to move to")
		}
	}
	attributes := form.Fields[0].Options
	if slices.Contains(attributes, attrWorkflow) {
		t.Errorf("the attribute list offers a workflow with nowhere to go: %v", attributes)
	}
}

func TestAnArchivedProjectIsNotOfferedArchivingAgain(t *testing.T) {
	when := time.Date(2026, 5, 6, 7, 8, 0, 0, time.UTC)
	p := setupProject()
	p.ArchivedAt = &when
	form := ProjectRemoveForm(p, true, true)
	if slices.Contains(form.Fields[0].Options, "archive") {
		t.Errorf("an archived project is offered an archive the service refuses: %v", form.Fields[0].Options)
	}
	if !form.Open() {
		t.Fatal("an archived project cannot be deleted either")
	}
	if !strings.Contains(form.Note, "already archived") {
		t.Errorf("the form does not say the project is archived: %q", form.Note)
	}
	if ProjectRemoveForm(p, true, false).Open() {
		t.Error("a reader who may only archive is offered a form with nothing on it")
	}
}

func TestTheFieldPickerIsNotOfferedWithNothingToPick(t *testing.T) {
	if FieldPickForm(nil, true, true).Open() {
		t.Error("a project with no definitions offers a picker anyway")
	}
	if FieldPickForm(setupFields(), false, false).Open() {
		t.Error("a reader who may do neither is offered a picker")
	}
	only := FieldPickForm(setupFields(), false, true)
	if got := only.Value("action"); got != "remove" {
		t.Errorf("a reader who may only remove is offered %q", got)
	}
}

func TestEditingAProjectAttributeDrawnFromAListSendsIt(t *testing.T) {
	m, svc := setupModel(t)
	m, _ = m.reduce(pressKey("e"))
	frame := m.View()
	if got := formRow(t, frame, "edit project infra", "attribute"); !strings.Contains(got, attrName) {
		t.Errorf("the form opens on %q", got)
	}

	// Step the attribute to the colour, then the colour to the next swatch.
	for range 3 {
		m, _ = m.reduce(pressKey("right"))
	}
	if got := formRow(t, m.View(), "edit project infra", "attribute"); !strings.Contains(got, attrColour) {
		t.Fatalf("the attribute row says %q", got)
	}
	m, _ = m.reduce(pressKey("down"))
	m, _ = m.reduce(pressKey("right"))
	chosen := formRow(t, m.View(), "edit project infra", "colour")

	m, cmd := m.reduce(pressKey("enter"))
	_, _ = m.reduce(run(t, cmd))
	if len(svc.projectsEdited) != 1 {
		t.Fatalf("the edit sent %d updates", len(svc.projectsEdited))
	}
	in := svc.projectsEdited[0]
	if in.Color == nil {
		t.Fatalf("the edit sent no colour: %+v", in)
	}
	if !strings.Contains(chosen, *in.Color) {
		t.Errorf("the form showed %q and sent %q", chosen, *in.Color)
	}
	if in.Name != nil || in.Description != nil {
		t.Errorf("a colour edit also sent other fields: %+v", in)
	}
}

func TestEditingAFreeTextAttributeOpensThePromptSeededWithIt(t *testing.T) {
	m, svc := setupModel(t)
	m, _ = m.reduce(pressKey("e"))
	m, _ = m.reduce(pressKey("enter"))
	if m.prompt != promptProjectName {
		t.Fatalf("the name attribute opened prompt %v", m.prompt)
	}
	if got := m.input.Value(); got != "Infrastructure" {
		t.Errorf("the prompt is seeded with %q", got)
	}
	m.input.SetValue("Infra")
	m, cmd := m.reduce(pressKey("enter"))
	m, _ = m.reduce(run(t, cmd))
	if len(svc.projectsEdited) != 1 || svc.projectsEdited[0].Name == nil {
		t.Fatalf("the name edit sent %+v", svc.projectsEdited)
	}
	if got := *svc.projectsEdited[0].Name; got != "Infra" {
		t.Fatalf("the name edit sent %q", got)
	}
	if !strings.Contains(m.status, "name") || !strings.Contains(m.status, "infra") {
		t.Errorf("the status bar says %q", m.status)
	}
}

func TestArchivingAProjectNamesItBeforeItHappens(t *testing.T) {
	m, svc := setupModel(t)
	m, _ = m.reduce(pressKey("X"))
	if got := formRow(t, m.View(), "remove project infra", "action"); !strings.Contains(got, "archive") {
		t.Fatalf("the removal form opens on %q", got)
	}
	m, _ = m.reduce(pressKey("enter"))

	line := confirmLine(t, m.View(), m.keys.Agree.Help().Key, m.keys.Cancel.Help().Key)
	if !strings.Contains(line, "archive project infra") {
		t.Errorf("the confirmation asks %q", line)
	}
	if strings.Contains(line, ProjectDeleteNote) {
		t.Errorf("an archive claims a deletion's reach: %q", line)
	}

	m, cmd := m.reduce(pressKey("y"))
	_, _ = m.reduce(run(t, cmd))
	if len(svc.archived) != 1 || svc.archived[0] != "infra" {
		t.Fatalf("the archive sent %v", svc.archived)
	}
	if len(svc.projectsGone) != 0 {
		t.Fatalf("an archive deleted the project: %v", svc.projectsGone)
	}
}

func TestDeletingAProjectStatesHowFarItReachesAndLeavesTheScreen(t *testing.T) {
	m, svc := setupModel(t)
	m, _ = m.reduce(pressKey("X"))
	m, _ = m.reduce(pressKey("right"))
	if got := formRow(t, m.View(), "remove project infra", "action"); !strings.Contains(got, "delete") {
		t.Fatalf("the action row says %q", got)
	}
	m, _ = m.reduce(pressKey("enter"))

	line := confirmLine(t, m.View(), m.keys.Agree.Help().Key, m.keys.Cancel.Help().Key)
	if !strings.Contains(line, "delete project infra") || !strings.Contains(line, ProjectDeleteNote) {
		t.Errorf("the confirmation asks %q", line)
	}

	m, cmd := m.reduce(pressKey("y"))
	m, _ = m.reduce(run(t, cmd))
	if len(svc.projectsGone) != 1 || svc.projectsGone[0] != "infra" {
		t.Fatalf("the deletion sent %v", svc.projectsGone)
	}
	if m.view != viewProjects {
		t.Errorf("the screen stayed on a project that no longer exists: view = %v", m.view)
	}
	if m.setup != nil {
		t.Error("the deleted project is still the screen's subject")
	}
}

func TestCancellingAConfirmationLeavesTheProjectAlone(t *testing.T) {
	m, svc := setupModel(t)
	m, _ = m.reduce(pressKey("X"))
	m, _ = m.reduce(pressKey("right"))
	m, _ = m.reduce(pressKey("enter"))
	m, cmd := m.reduce(pressKey("esc"))
	if cmd != nil {
		t.Fatal("cancelling a confirmation asked for work")
	}
	if len(svc.projectsGone) != 0 || len(svc.archived) != 0 {
		t.Fatalf("cancelling removed the project: %v %v", svc.projectsGone, svc.archived)
	}
	if m.confirm.Open() {
		t.Error("the question is still standing after it was cancelled")
	}
}

func TestDefiningANewFieldGathersItsKeyThenItsShape(t *testing.T) {
	m, svc := setupModel(t)
	m, _ = m.reduce(pressKey("n"))
	if m.prompt != promptNewField {
		t.Fatalf("n opened prompt %v", m.prompt)
	}
	m.input.SetValue("impact How much it matters")
	m, _ = m.reduce(pressKey("enter"))

	if got := formRow(t, m.View(), "field impact", "type"); !strings.Contains(got, "string") {
		t.Fatalf("a new field opens on type %q", got)
	}
	m, _ = m.reduce(pressKey("right"))
	m, _ = m.reduce(pressKey("down"))
	m, _ = m.reduce(pressKey("right"))
	if got := formRow(t, m.View(), "field impact", "required"); !strings.Contains(got, yesValue) {
		t.Fatalf("the required row says %q", got)
	}

	m, cmd := m.reduce(pressKey("enter"))
	m, _ = m.reduce(run(t, cmd))
	if len(svc.fieldsPut) != 1 {
		t.Fatalf("the definition sent %d puts", len(svc.fieldsPut))
	}
	in := svc.fieldsPut[0]
	if in.Key != "impact" || in.Label != "How much it matters" || !in.Required {
		t.Errorf("the definition sent %+v", in)
	}
	if in.Type == core.FieldString {
		t.Errorf("stepping the type changed nothing: %+v", in)
	}
	if err := in.Validate(); err != nil {
		t.Errorf("the definition the form built is refused: %v", err)
	}
}

func TestRedefiningAFieldStartsFromTheOneThatWasPicked(t *testing.T) {
	m, svc := setupModel(t)
	m, _ = m.reduce(pressKey("f"))
	if got := formRow(t, m.View(), "custom fields", "field"); !strings.Contains(got, "severity") {
		t.Fatalf("the picker opens on %q", got)
	}
	// Move to the second definition, so the form that follows has to be seeded
	// from it rather than from the first.
	m, _ = m.reduce(pressKey("right"))
	if got := formRow(t, m.View(), "custom fields", "field"); !strings.Contains(got, "owner") {
		t.Fatalf("the picker moved to %q", got)
	}
	m, _ = m.reduce(pressKey("enter"))

	if got := formRow(t, m.View(), "field owner", "type"); !strings.Contains(got, "string") {
		t.Errorf("the second field's form was seeded from another field: %q", got)
	}
	if got := formRow(t, m.View(), "field owner", "required"); !strings.Contains(got, noValue) {
		t.Errorf("an optional field opens as %q", got)
	}
	m, cmd := m.reduce(pressKey("enter"))
	_, _ = m.reduce(run(t, cmd))
	if len(svc.fieldsPut) != 1 || svc.fieldsPut[0].Key != "owner" {
		t.Fatalf("the redefinition sent %+v", svc.fieldsPut)
	}
}

func TestRemovingAFieldDefinitionNamesTheFieldItRemoves(t *testing.T) {
	m, svc := setupModel(t)
	m, _ = m.reduce(pressKey("f"))
	m, _ = m.reduce(pressKey("down"))
	m, _ = m.reduce(pressKey("right"))
	if got := formRow(t, m.View(), "custom fields", "action"); !strings.Contains(got, "remove") {
		t.Fatalf("the action row says %q", got)
	}
	m, _ = m.reduce(pressKey("enter"))

	line := confirmLine(t, m.View(), m.keys.Agree.Help().Key, m.keys.Cancel.Help().Key)
	if !strings.Contains(line, "delete custom field severity") {
		t.Errorf("the confirmation asks %q", line)
	}

	m, cmd := m.reduce(pressKey("y"))
	_, _ = m.reduce(run(t, cmd))
	if len(svc.fieldsGone) != 1 || svc.fieldsGone[0] != [2]string{"infra", "severity"} {
		t.Fatalf("the deletion sent %v", svc.fieldsGone)
	}
}

func TestAConfirmationWithNothingNamedIsNotOpened(t *testing.T) {
	m, _ := setupModel(t)
	blank := *m.setup
	blank.project.Key = ""
	m.setup = &blank
	next, cmd := m.confirmProjectRemoval(Form{Kind: formProjectRemove,
		Fields: []FormField{{Key: "action", Value: "delete"}}})
	if next.confirm.Open() {
		t.Error("a confirmation that can name nothing was opened")
	}
	if cmd != nil {
		t.Error("an unnamed confirmation asked for work")
	}
	if next.err == "" {
		t.Error("the refusal says nothing")
	}
}

func TestTheProjectScreenOpensOnTheRowTheListingSelected(t *testing.T) {
	m := New(Config{Access: fullAccess(), Actor: fullActor(), Environ: []string{"NO_COLOR=1"}})
	m.width, m.height = 120, 40
	svc := setupService()
	svc.projects = append(svc.projects, core.Project{ID: "p2", Key: "web", Name: "Website",
		WorkflowID: "w-1"})
	m.svc = svc
	m, _ = m.reduce(projectsMsg{projects: svc.projects})
	m, _ = m.reduce(pressKey("down"))
	m, cmd := m.reduce(pressKey("w"))
	m, _ = m.reduce(run(t, cmd))

	if got := setupRow(t, m.View(), "project:", "key"); !strings.Contains(got, "web") {
		t.Errorf("the screen opened on %q rather than the selected row", got)
	}
}

func TestAReaderWhoMayNotConfigureAProjectIsOfferedNothingToConfigureWith(t *testing.T) {
	actor := &core.Actor{ID: "r", TenantID: "t", Kind: core.ActorAgent,
		Scopes: []core.Scope{core.ScopeProjectRead, core.ScopeWorkflowRead, core.ScopeTaskRead}}
	m := New(Config{Access: capability.TUIAccess(actor), Actor: actor, Environ: []string{"NO_COLOR=1"}})
	m.width, m.height = 120, 40
	svc := setupService()
	m.svc = svc
	m, _ = m.reduce(projectsMsg{projects: svc.projects})
	m, cmd := m.reduce(pressKey("w"))
	m, _ = m.reduce(run(t, cmd))
	if m.view != viewProject {
		t.Fatalf("a reader who may read a project cannot open its screen: %v", m.view)
	}

	footer := lastLine(m.View())
	for _, key := range []string{"edit project", "archive or delete", "new field", "custom fields"} {
		if strings.Contains(footer, key) {
			t.Errorf("the footer offers %q to a reader who may not do it: %q", key, footer)
		}
	}
	// And the keys themselves do nothing, so a reader who knows them from
	// another session is refused here rather than by the service.
	for _, press := range []string{"e", "X", "n", "f"} {
		next, cmd := m.reduce(pressKey(press))
		if cmd != nil {
			t.Errorf("%q asked for work from a reader who may not press it", press)
		}
		if next.form.Open() || next.confirm.Open() || next.prompt != promptNone {
			t.Errorf("%q opened an input for a reader who may not press it", press)
		}
	}
	if len(svc.projectsEdited)+len(svc.archived)+len(svc.projectsGone)+len(svc.fieldsPut) != 0 {
		t.Error("a refused reader still reached the service")
	}
}

func TestAReaderNotOfferedTheProjectScreenDoesNotEvenReadForIt(t *testing.T) {
	actor := &core.Actor{ID: "t", TenantID: "t", Kind: core.ActorAgent,
		Scopes: []core.Scope{core.ScopeTaskRead}}
	access := capability.TUIAccess(actor)
	if access[viewName(viewProject)] {
		t.Fatal("a reader who may read neither projects nor workflows is offered the screen")
	}
	m := New(Config{Access: access, Actor: actor, Environ: []string{"NO_COLOR=1"}})
	m.width, m.height = 120, 40
	svc := setupService()
	m.svc = svc
	m, _ = m.reduce(projectsMsg{projects: svc.projects})

	next, cmd := m.reduce(pressKey("w"))
	if cmd != nil {
		t.Error("a refused reader's keypress asked the service for a project")
	}
	if next.setup != nil || next.view == viewProject {
		t.Error("a refused reader reached the project screen")
	}
	// And the overlay does not name the key either.
	for _, e := range m.keys.GlobalHelp(m.offersView()) {
		if e.Desc == "project setup" {
			t.Error("a refused reader is told about the key that opens the screen")
		}
	}
}

func TestTheHelpOverlayOnTheProjectScreenNamesOnlyWhatTheReaderMayDo(t *testing.T) {
	keys := DefaultKeyMap()
	full := keys.ViewHelp(viewProject, permitAll)
	descriptions := func(entries []HelpEntry) []string {
		out := make([]string, 0, len(entries))
		for _, e := range entries {
			out = append(out, e.Desc)
		}
		return out
	}
	for _, want := range []string{"edit project", "archive or delete", "new field", "custom fields"} {
		if !slices.Contains(descriptions(full), want) {
			t.Errorf("a reader who may do everything is not told about %q", want)
		}
	}
	// The borrowed keys are described by what they do here, not on the board.
	if slices.Contains(descriptions(full), "edit title") {
		t.Error("the project screen describes its edit key as the board's")
	}

	narrow := descriptions(keys.ViewHelp(viewProject, permitOnly("UpdateProject")))
	if !slices.Contains(narrow, "edit project") {
		t.Errorf("a reader who may edit is not told about it: %v", narrow)
	}
	for _, unwanted := range []string{"archive or delete", "new field", "custom fields"} {
		if slices.Contains(narrow, unwanted) {
			t.Errorf("a narrowed overlay still offers %q", unwanted)
		}
	}
	if len(keys.ViewHelp(viewProject, nil)) >= len(full) {
		t.Error("a nil predicate offered as much as full authority")
	}
}

func TestTheFieldPickerIsNotAdvertisedWhereThereIsNothingToPick(t *testing.T) {
	keys := DefaultKeyMap()
	with := keys.ShortHelp(viewProject, ActionContext{May: permitAll, HasFields: true})
	without := keys.ShortHelp(viewProject, ActionContext{May: permitAll})
	found := func(entries []HelpEntry, desc string) bool {
		for _, e := range entries {
			if e.Desc == desc {
				return true
			}
		}
		return false
	}
	if !found(with, "custom fields") {
		t.Error("a project with definitions does not advertise the picker")
	}
	if found(without, "custom fields") {
		t.Error("a project with no definitions advertises a picker with nothing in it")
	}
	if !found(without, "new field") {
		t.Error("a project with no definitions cannot be told how to get one")
	}
}

func TestEverySchemeDrivesTheProjectScreen(t *testing.T) {
	for _, scheme := range Schemes() {
		t.Run(string(scheme), func(t *testing.T) {
			keys := KeyMapFor(scheme)
			m := boardModel(t)
			m.svc = setupService()
			m = m.installScheme(string(scheme))
			if m.err != "" {
				t.Fatalf("the scheme did not install: %s", m.err)
			}
			m, cmd := m.reduce(keyMsgFor(keys.Project.Help().Key))
			m, _ = m.reduce(run(t, cmd))
			if m.view != viewProject {
				t.Fatalf("%q did not open the project screen", keys.Project.Help().Key)
			}
			m, _ = m.reduce(keyMsgFor(keys.EditTitle.Help().Key))
			if !m.form.Open() || m.form.Kind != formProject {
				t.Fatalf("%q did not open the edit form", keys.EditTitle.Help().Key)
			}
			m, _ = m.reduce(keyMsgFor(keys.Cancel.Help().Key))
			m, _ = m.reduce(keyMsgFor(keys.Delete.Help().Key))
			if m.form.Kind != formProjectRemove {
				t.Fatalf("%q did not open the removal form", keys.Delete.Help().Key)
			}
			// The form moves and cycles on the keys the scheme carries, with no
			// bindings of its own.
			m, _ = m.reduce(keyMsgFor(keys.Right.Help().Key))
			if got := formRow(t, m.View(), "remove project infra", "action"); !strings.Contains(got, "delete") {
				t.Errorf("%q did not cycle the answer: %q", keys.Right.Help().Key, got)
			}
			m, _ = m.reduce(keyMsgFor(keys.Accept.Help().Key))
			if !m.confirm.Open() {
				t.Fatalf("%q did not submit the form", keys.Accept.Help().Key)
			}
		})
	}
}

func TestAnEventOnTheOpenProjectReadsItAgain(t *testing.T) {
	m, svc := setupModel(t)
	before := len(svc.workflowAsked)
	cmd := m.reloadFor(core.Event{Type: core.EventProjectUpdated, ProjectID: "p1"})
	if cmd == nil {
		t.Fatal("an event about the open project reloaded nothing")
	}
	_, _ = m.reduce(cmd())
	if len(svc.workflowAsked) <= before {
		t.Error("the project screen was not read again")
	}
}

func TestRefreshOnTheProjectScreenReadsTheProject(t *testing.T) {
	m, svc := setupModel(t)
	before := len(svc.workflowAsked)
	m, cmd := m.reduce(pressKey("r"))
	m, _ = m.reduce(run(t, cmd))
	if len(svc.workflowAsked) <= before {
		t.Error("refresh on the project screen did not read the project")
	}
	if m.view != viewProject {
		t.Errorf("refresh left the screen: %v", m.view)
	}
}

func TestSteppingBackLeavesTheProjectScreen(t *testing.T) {
	m, _ := setupModel(t)
	m, _ = m.reduce(pressKey("esc"))
	if m.view == viewProject {
		t.Error("esc did not leave the project screen")
	}
}

// lastLine is the footer's key line, which is the last line of a frame.
func lastLine(frame string) string {
	lines := strings.Split(strings.TrimRight(frame, "\n"), "\n")
	return lines[len(lines)-1]
}
