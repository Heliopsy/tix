// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/output"
)

// DueParam is the query parameter the deadline control submits. Its values are
// the filter language's own due: windows, so a link a reader shares and a
// filter they type say the same thing.
const DueParam = "due"

// AllParam turns off the open-work default, so the listing shows finished
// tasks too. It is a parameter of its own rather than a filter expression
// because "everything" is not a term the filter language has, and writing one
// in to mean it would put a rule in the box that the reader did not type.
const AllParam = "all"

// priorityChoice is one option of the priority control.
type priorityChoice struct {
	Value core.Priority
	Label string
}

// priorityChoices is the priority vocabulary the forms offer.
var priorityChoices = []priorityChoice{
	{core.PriorityHighest, "highest"},
	{core.PriorityHigh, "high"},
	{core.PriorityNormal, "normal"},
	{core.PriorityLow, "low"},
	{core.PriorityLowest, "lowest"},
}

// tasksView is what the filterable task list renders.
type tasksView struct {
	// Mine marks the reader's own work, drawn as a list grouped by
	// deadline, and Held that it takes in the tasks they hold as well.
	// HeldHref turns that on or off, and More says the page was cut short.
	Mine     bool
	Held     bool
	HeldHref string
	More     bool

	Tasks    []core.Task
	Projects []core.Project
	Tags     []core.Tag
	// Templates are what the add box menu offers to create a task from.
	Templates  []core.TaskTemplate
	Priorities []priorityChoice
	SortFields []string
	Query      string
	Sort       string

	// Due is the deadline window the control has selected, and DueWindows the
	// vocabulary it offers. The control writes a due: term into the same
	// expression the box holds, so the browser, the board and the command line
	// answer one question with one parser.
	Due        string
	DueWindows []string

	Pager pager

	// Self is this screen's own URL, query string and all, so a row action
	// can return to the list the reader was actually looking at.
	Self string

	// Summary is the one line under the heading. A tracker that only ever
	// shows rows tells you nothing about the shape of your day.
	Summary taskSummary

	// CompleteState maps a project id to the state a one click complete moves
	// its tasks into, so the list can offer a tick box without the reader
	// knowing the project's workflow.
	CompleteState map[string]string

	// Accent maps a project id to the stripe and icon its rows carry, so a
	// mixed list can be read one project at a time.
	Accent map[string]projectAccent

	// Moves lists, per project, the workflow that project runs, so a row can
	// offer the states its own task may move to. A listing mixes projects,
	// and a state that is legal in one is not necessarily legal in another.
	Moves map[string]core.WorkflowDefinition

	// Prefs holds the shared preference controls this screen renders -- the
	// view switch, the column picker and the project visibility choice --
	// each one the same form the settings screen renders, from the same
	// constructor, so neither screen can show a different answer for a
	// preference they both edit. See prefs.go.
	Prefs prefsView

	// View is how this reader draws the screen. Both views answer one
	// filter, one page and one visibility choice, so switching changes the
	// drawing and never the selection. It is read off Prefs rather than
	// resolved again, so the board the screen builds and the position the
	// switch shows come from one answer.
	View string

	// Board is the board the switch draws, or the workflows that stop it
	// being drawable.
	Board taskBoard

	// Lanes are the same page drawn one lane per project, built only when
	// that is the view being drawn.
	Lanes taskLanes

	// ListGroup is how the list groups its rows, and ListGroups the groups
	// it draws, only when the list is drawn grouped.
	ListGroup  string
	ListGroups []listGroup

	// Arrange is the reader's own arrangement of the groups being drawn: the
	// board's columns, the lanes, or the list's groups. Its View is empty
	// when nothing on the screen is grouped.
	Arrange arrangement

	// Names labels the actors holding a lease, so a held row can say who has
	// it rather than only that somebody does.
	Names actorNames

	// FilterError is what the filter bar says when the expression in the box
	// cannot be answered. It is rendered beside the box rather than instead
	// of the page, because a typo in a filter is not a missing page, and the
	// reader has to be able to see the query to correct it.
	//
	// The screen is still served as a success: the listing asked for is what
	// rendered, and a boosted browser swaps nothing from a response that is
	// not one, so an error status would leave the old page on screen with no
	// message on it at all.
	FilterError string

	// Live is what keeps this screen current while it is open: the fragment
	// route carrying this listing's own state, the events worth re-rendering
	// for, and the regions the answer replaces. See live.go.
	Live liveScreen

	// Hidden is how many projects are put away, and Filtered is set when an
	// explicit project: filter overrode that choice, so a reader is never
	// shown a shorter task list without being told which rule produced it.
	Hidden   int
	Filtered bool
	Empty    bool

	// OpenOnly is set when nothing was asked for and the listing was narrowed
	// to unfinished work. It is surfaced with a way to turn it off, because a
	// list that quietly shows a subset is worse than one that shows the wrong
	// thing: the reader cannot tell which they are looking at.
	OpenOnly bool

	// AllHref is where the affordance beside it leads: this same listing with
	// the default turned off.
	AllHref string

	// Trash is set when the filter asks for deleted tasks alone. The screen
	// is then the trash: titled so, drawn as the list, and without the
	// quick-add row, which would add live tasks to a screen that hides them.
	Trash bool

	// Form is what the add forms hold: the project the listing is filtered
	// to, or everything a refused add was submitted with. Refused draws Add
	// with details in place rather than as a popover, with Errors beside its
	// fields.
	Form    taskForm
	Errors  fieldErrors
	Refused bool

	// ExportHref downloads this listing as CSV.
	ExportHref string

	// Bulk is the selection bar under the list: what a selection of rows can
	// have done to it in one request.
	Bulk bulkBar

	// Quick is what the one-line box shows under itself: what its line will
	// set, and why a refused line was refused. QuickLine is that line, kept
	// in the box after a refusal.
	Quick     quickPreview
	QuickLine string
}

// filterFault reports whether an error is the filter expression's fault.
//
// The expression names things the reader typed -- a status, a project key, a
// tag, a handle -- and every one of them is refused by the service as invalid
// or not found. Kind is what is matched rather than the individual failures,
// because the set of things a filter term can name grows, and a rule per term
// would go stale the next time it does.
func filterFault(err error) bool {
	switch core.KindOf(err) {
	case core.KindInvalid, core.KindNotFound:
		return true
	}
	return trashRefusal(err)
}

// trashRefusal reports whether the service refused because the tenant's
// trash is off, which the trash screen says in its filter bar rather than as
// an error page: the reader asked a fair question with a link that may have
// been bookmarked before the trash was turned off.
func trashRefusal(err error) bool {
	var domain *core.Error
	return errors.As(err, &domain) && domain.Details["reason"] == core.ErrTrashOff
}

// projectAccent is how one project marks its rows apart from another's.
type projectAccent struct {
	Key   string
	Name  string
	Color core.ProjectColor
	Icon  string

	IconName  string
	IconImage string
}

// projectAccents maps each project to its row marking.
func projectAccents(projects []core.Project) map[string]projectAccent {
	out := make(map[string]projectAccent, len(projects))
	for _, p := range projects {
		out[p.ID] = projectAccent{Key: p.Key, Name: p.Name, Color: p.Color, Icon: p.Icon,
			IconName: p.IconName, IconImage: p.IconImage}
	}
	return out
}

// taskSummary counts what is on the page, so the heading can say something.
// Projects is how many distinct lists the rows come from, which is the count
// that tells a reader whether they are looking at one project's work or at
// everything at once -- the same question the visibility control answers, so
// the two belong on screen together.
type taskSummary struct {
	Open     int
	Held     int
	Done     int
	Blocked  int
	Projects int
}

// Total is how many tasks the page carries.
func (s taskSummary) Total() int { return s.Open + s.Done }

// AllDone reports whether everything on the page is finished.
func (s taskSummary) AllDone() bool { return s.Total() > 0 && s.Done == s.Total() }

// summarise counts the page by what a reader cares about: what is left, what
// is moving, and what an agent is holding right now.
func summarise(tasks []core.Task, complete map[string]string, now time.Time) taskSummary {
	var out taskSummary
	seen := make(map[string]bool, len(tasks))
	for _, t := range tasks {
		if t.Status == complete[t.ProjectID] {
			out.Done++
		} else {
			out.Open++
		}
		if t.Blocked {
			out.Blocked++
		}
		if t.ClaimedByActorID != "" && t.LeaseExpiresAt != nil && t.LeaseExpiresAt.After(now) {
			out.Held++
		}
		if t.ProjectID != "" && !seen[t.ProjectID] {
			seen[t.ProjectID] = true
			out.Projects++
		}
	}
	return out
}

// showTasks renders the filterable task list.
func (h *handler) showTasks(w http.ResponseWriter, r *http.Request) error {
	data, err := h.buildTasksView(r)
	if err != nil {
		return err
	}
	title := "Tasks"
	if data.Trash {
		title = "Trash"
	}
	return h.render(w, r, "tasks.html", title, data)
}

// showTasksLive renders only the regions a live update replaces -- the
// summary line and the listing, whichever way it is drawn -- from the same
// view the whole screen is built from. It reads the filter, the deadline
// window, the sort and the cursor out of its own query string, which the
// page wrote into the element live.js fetches from, so an update never
// answers a wider question than the screen was already answering.
func (h *handler) showTasksLive(w http.ResponseWriter, r *http.Request) error {
	data, err := h.buildTasksView(r)
	if err != nil {
		return err
	}
	return h.renderFragment(w, r, "tasks.html", "tasks-live", "Tasks", data)
}

// listingFilter answers the filter half of the task screen: the expression,
// the deadline window, the open-work default and the page, read off the
// request the same way for the screen, its live fragment and the CSV export.
func (h *handler) listingFilter(r *http.Request, view string) (filter core.TaskFilter, due, refused string, openOnly bool, err error) {
	query := r.URL.Query()
	expression := query.Get("q")
	// The deadline control is folded into the expression rather than set on
	// the parsed filter, so it is answered by the one parser the box, the
	// board and the command line share, and a value the language does not
	// have is refused with that parser's message rather than silently
	// answering a different question. The control's own state stays in the
	// control and in the URL; it is not written into the box, which would put
	// the same term in two places for the reader to keep in step. The term
	// leads the expression, so a due: term typed by hand wins.
	due = strings.TrimSpace(query.Get(DueParam))
	if due != "" {
		expression = strings.TrimSpace("due:" + due + " " + expression)
	}
	filter, err = ParseFilter(expression)
	if err != nil {
		if expression == "" || !filterFault(err) {
			return filter, due, refused, openOnly, err
		}
		refused, filter = safeMessage(err, core.KindOf(err)), core.TaskFilter{}
	}
	// With nothing asked for, show work that is still open.
	//
	// The default sort is urgency, which is priority then due date and never
	// looks at status, so a tenant with any history opened this screen on its
	// finished tasks: in the seeded fixture, twenty-one of the first
	// twenty-five rows were closed. The first screen of a task tracker should
	// be the work.
	//
	// The statuses come from the tenant's own workflows rather than from the
	// five built-in names, because terminality is a property of the workflow:
	// a tenant that renamed done to shipped must not have its listing silently
	// stop narrowing.
	//
	// The board and the lanes are exempt. The board's columns are the workflow, terminal states
	// included, so narrowing it to the open states empties the columns a card
	// is moved into: a reader who finished a card watched it leave the board
	// altogether. Lanes draw a finished task ticked, and their own Show
	// completed control is what hides one. The default belongs to the list,
	// which is the screen that opened on the wrong set.
	trash := filter.Trashed == core.Yes
	showAll := query.Get(AllParam) != "" || trash
	if trash {
		view = TaskViewList
	}
	//
	// Any filter that does not name a status gets the same narrowing, not just
	// the empty one. Keying it on "the reader typed nothing" made every other
	// filter mean something different from the screen it was reached from:
	// the overdue badge is drawn only for work that can still miss its
	// deadline, because a finished task has none, but pressing it asked for
	// every task with a past due date and answered with twenty-one closed
	// ones and four open. The badge and the filter used the same word for two
	// different questions. A deadline, a tag or a priority is a question
	// about work; "and the finished ones too" is what the All control is for.
	if !showAll && view == TaskViewList && len(filter.Statuses) == 0 {
		if open, err := h.unfinishedStatuses(r.Context()); err == nil && len(open) > 0 {
			filter.Statuses = open
			openOnly = true
		}
	}

	filter.Page = core.Page{
		Cursor:    query.Get(CursorParam),
		Sort:      query.Get("sort"),
		Direction: core.Ascending,
		Limit:     h.rowsPerPage(r),
	}
	if filter.Page.Sort == "" {
		filter.Page.Sort = core.SortUrgency
	}
	return filter, due, refused, openOnly, nil
}

// buildTasksView answers one request for the task screen, for the page and
// for its live fragment alike.
func (h *handler) buildTasksView(r *http.Request) (tasksView, error) {
	return h.buildTaskScreen(r, false)
}

// buildTaskScreen answers the task screen, or with mine the reader's own
// work: the tasks assigned to them, and with held=1 the ones they hold too,
// grouped by deadline. My work asks its fixed question through the same
// listingFilter, on a copy of the request that carries it.
func (h *handler) buildTaskScreen(r *http.Request, mine bool) (tasksView, error) {
	held := mine && r.URL.Query().Get(HeldParam) != ""
	asked := r
	view := taskViewOf(r)
	if mine {
		asked, view = myWorkRequest(r), TaskViewList
	}
	query := asked.URL.Query()
	filter, due, refused, openOnly, err := h.listingFilter(asked, view)
	if err != nil {
		return tasksView{}, err
	}
	trash := filter.Trashed == core.Yes
	if trash {
		view = TaskViewList
	}
	if mine {
		if query.Get("sort") == "" {
			filter.Page.Sort = core.SortDueAt
		}
		filter.Page.Cursor, filter.Page.Limit = "", myWorkLimit
	}
	expression := query.Get("q")
	if due != "" {
		expression = strings.TrimSpace("due:" + due + " " + expression)
	}
	projects, whole, err := h.allProjects(r, false)
	if err != nil {
		return tasksView{}, err
	}
	prefs := h.prefsFor(r, projects, whole)
	away := hiddenProjects(r)
	filtered := len(filter.ProjectKeys) > 0
	hidden := prefs.Projects.Hidden
	// Put away is an exclusion, never an inclusion. Naming the projects to
	// show would make the listing depend on the reader's screen having been
	// able to name all of them, and would grow one term per project in a
	// tenant that has thousands; naming the few that are hidden is the same
	// answer at any size. An explicit project: filter is the reader asking
	// for that project by name, so it wins over what the control put away.
	if live := liveHiddenKeys(projects, away); !filtered && len(live) > 0 {
		filter.Exclude.ProjectKeys = append(filter.Exclude.ProjectKeys, live...)
	}
	var page core.TaskPage
	if refused == "" {
		if page, err = h.svc.ListTasks(r.Context(), filter); err != nil {
			if expression == "" || !filterFault(err) {
				return tasksView{}, err
			}
			refused, page = safeMessage(err, core.KindOf(err)), core.TaskPage{}
		}
	}
	if held && refused == "" {
		if page, err = h.withHeld(r, filter, page); err != nil {
			return tasksView{}, err
		}
	}
	tags, err := h.svc.ListTags(r.Context())
	if err != nil {
		return tasksView{}, err
	}
	complete, err := h.completeStates(r, projects)
	if err != nil {
		return tasksView{}, err
	}
	moves, err := h.workflowsByProject(r, projects)
	if err != nil {
		return tasksView{}, err
	}
	// The board is resolved from the same filter the list was answered with,
	// so the two views cover one selection of tasks. Only when it is the view
	// being drawn: a reader on the list pays nothing for a board they are not
	// looking at, and what the board would have said is on the board.
	var board taskBoard
	if view == TaskViewBoard {
		if board, err = h.taskBoardFor(r, projects, filter, page.Tasks, moves); err != nil {
			return tasksView{}, err
		}
	}
	var lanes taskLanes
	if view == TaskViewLanes {
		lanes = buildLanes(projects, filter, page.Tasks, laneMoves{Workflows: moves, HasScope: readerHasScope(r)})
	}
	self := screenHref(r, RouteTasks)
	group := listGroupOf(r)
	if mine {
		self, group = screenHref(r, RouteMine), ListGroupDue
	}
	var listGroups []listGroup
	var arrange arrangement
	switch {
	case view == TaskViewBoard && board.Agreed:
		arrange = h.arrangeBoard(r, &board, self)
	case view == TaskViewLanes:
		arrange = h.arrangeLanes(r, &lanes, projects, self)
	case mine && refused == "":
		listGroups, arrange = h.arrangeMine(r, page.Tasks, self)
	case view == TaskViewList && group != ListGroupNone && refused == "":
		listGroups, arrange = h.arrangeList(r, group, candidateProjects(projects, filter), page.Tasks, moves, self)
	}
	live := h.liveTasks(r)
	pageNav := newPager(r, RouteTasks, page.NextCursor, len(page.Tasks), "tasks", "q", "sort", DueParam, SizeParam, AllParam)
	if mine {
		live.Href = fragmentHref(r, RouteMineLive)
		pageNav = pager{}
	}
	return tasksView{
		Mine:          mine,
		Held:          held,
		HeldHref:      heldHref(r, !held),
		More:          mine && page.NextCursor != "",
		Tasks:         page.Tasks,
		Projects:      projects,
		Tags:          tags,
		Templates:     h.addTemplates(r),
		Priorities:    priorityChoices,
		SortFields:    core.TaskSortFields,
		Query:         query.Get("q"),
		Sort:          filter.Page.Sort,
		Due:           due,
		DueWindows:    FilterDueWindows,
		Pager:         pageNav,
		Self:          self,
		CompleteState: complete,
		Accent:        projectAccents(projects),
		Summary:       summarise(page.Tasks, complete, time.Now()),
		FilterError:   refused,
		Names:         h.resolveActors(r, rowActors(page.Tasks)...),
		Moves:         moves,
		Prefs:         prefs,
		View:          view,
		Board:         board,
		Lanes:         lanes,
		ListGroup:     group,
		ListGroups:    listGroups,
		Arrange:       arrange,
		Hidden:        hidden,
		Filtered:      filtered && hidden > 0,
		OpenOnly:      openOnly,
		AllHref:       showAllHref(r),
		Trash:         trash,
		Empty:         hidden > 0 && hidden == len(prefs.Projects.Choices),
		Live:          live,
		Form:          taskForm{Project: soleProject(filter.ProjectKeys)},
		Bulk:          bulkBarUnlessTrash(r, page.Tasks, moves, trash),
		ExportHref:    csvHref(asked),
	}, nil
}

// inTrash reports whether a request is for the task list showing the trash,
// which is the list filtered by is:trashed rather than a route of its own, so
// the filter box says what the screen is.
func inTrash(r *http.Request) bool {
	if r.URL.Path != RouteTasks {
		return false
	}
	f, err := ParseFilter(r.URL.Query().Get("q"))
	return err == nil && f.Trashed == core.Yes
}

// readerHasScope answers whether the signed-in reader holds a scope, so a
// control the service would refuse is not drawn. The service still checks.
func readerHasScope(r *http.Request) func(core.Scope) bool {
	actor, ok := core.ActorFrom(r.Context())
	return func(s core.Scope) bool { return ok && actor.HasScope(s) }
}

// rowActors lists every actor the listing names: whoever holds a lease on a
// row, whoever it is assigned to, and whoever let a claim lapse on it. They
// are gathered before the page renders so one pass over the directory covers
// the whole listing, rather than a lookup per row. resolveActors then folds
// the repeats, so a list of fifty rows held by three agents costs three
// lookups.
func rowActors(tasks []core.Task) []string {
	out := make([]string, 0, len(tasks))
	for _, t := range tasks {
		out = append(out, t.ClaimedByActorID, t.AssigneeActorID, t.LeaseExpiredByActorID, t.SnoozedByActorID)
	}
	return out
}

// completeStates resolves, per project, the state that finishing a task moves
// it to: the first terminal state in the done category, or failing that the
// first terminal state at all.
func (h *handler) completeStates(r *http.Request, projects []core.Project) (map[string]string, error) {
	byID, err := h.workflowsByID(r)
	if err != nil {
		return nil, err
	}
	out := make(map[string]string, len(projects))
	for _, p := range projects {
		w, ok := byID[p.WorkflowID]
		if !ok {
			continue
		}
		if key := doneState(w.Definition); key != "" {
			out[p.ID] = key
		}
	}
	return out, nil
}

// workflowsByProject maps each project to the state machine it runs, so the
// listing can offer a row the moves its own workflow allows rather than a
// fixed set that happens to suit one project.
func (h *handler) workflowsByProject(r *http.Request, projects []core.Project) (map[string]core.WorkflowDefinition, error) {
	byID, err := h.workflowsByID(r)
	if err != nil {
		return nil, err
	}
	out := make(map[string]core.WorkflowDefinition, len(projects))
	for _, p := range projects {
		if w, ok := byID[p.WorkflowID]; ok {
			out[p.ID] = w.Definition
		}
	}
	return out, nil
}

// doneState picks the state a completed task belongs in.
func doneState(d core.WorkflowDefinition) string {
	fallback := ""
	for _, st := range d.States {
		if !st.Terminal {
			continue
		}
		if st.Category == core.CategoryDone {
			return st.Key
		}
		if fallback == "" {
			fallback = st.Key
		}
	}
	return fallback
}

// createTask creates a task from the list screen's form.
func (h *handler) createTask(w http.ResponseWriter, r *http.Request) error {
	form := readTaskForm(r)
	in, err := form.input(r)
	if err == nil && form.Quick {
		in, err = h.quickInput(r, in)
	}
	if err != nil {
		return h.refuseTask(w, r, form, err)
	}
	task, err := h.svc.CreateTask(r.Context(), in)
	if err != nil {
		return h.refuseTask(w, r, form, err)
	}
	// Both add paths land here, the one-line box and Add with details, so the
	// preference governs both rather than one of them. Staying returns to the
	// listing exactly as it was, filter, sort, page and all, because the
	// referring URL carries them and typing the next task is the whole reason
	// to stay. The flash links the new task and the fragment marks its row,
	// so staying never leaves the reader wondering whether it worked.
	if !openAfterAdd(r) {
		redirectTo(w, r, listingReferer(r), "t-"+task.Ref, "task created")
		return nil
	}
	redirect(w, r, RouteTasks+"/"+task.Ref, "task created")
	return nil
}

// refuseTask answers an add the reader can correct by drawing the listing it
// came from again, with Add with details drawn open in place, every value they
// entered still in it and the message beside the field it is about. It is the
// response to the ordinary form post, so scripting off gets it too.
func (h *handler) refuseTask(w http.ResponseWriter, r *http.Request, form taskForm, err error) error {
	if !refusable(err) {
		return err
	}
	data, viewErr := h.buildTasksView(refusedRequest(r))
	if viewErr != nil {
		return viewErr
	}
	if form.Quick {
		data.Form.Project, data.QuickLine = form.Project, form.Title
		data.Quick = h.previewLine(r, form.Title)
		data.Quick.Error = refusalMessage(err)
		return h.renderStatus(w, r, core.KindOf(err).HTTPStatus(), "tasks.html", "Tasks", data)
	}
	data.Form, data.Errors, data.Refused = form, refusal(err), true
	return h.renderStatus(w, r, core.KindOf(err).HTTPStatus(), "tasks.html", "Tasks", data)
}

// listingReferer is the listing a create came from, or the plain task list
// when the request carries nothing this trusts.
//
// Allow-listed rather than sanitised. safeNext cannot be used here: it
// answers a rejection with /projects, which is itself a real listing, so a
// forged header and a genuine one would be indistinguishable by its return
// value. Only a path this screen actually posts from is accepted, and the
// query string comes along because the filter, sort and page are in it and
// going back to a listing that forgot them is not going back.
func listingReferer(r *http.Request) string {
	ref := r.Referer()
	if ref == "" {
		return RouteTasks
	}
	u, err := url.Parse(ref)
	if err != nil || u.IsAbs() && u.Host != r.Host {
		return RouteTasks
	}
	path := u.Path
	if path != RouteTasks && !strings.HasPrefix(path, RouteProjects+"/") {
		return RouteTasks
	}
	return withoutFlash(path, u.Query())
}

// withoutFlash is a listing address minus the message a previous redirect
// left on it. Adding twice in a row would otherwise carry the first task's
// flash into the second redirect, and the toast would name the wrong task.
func withoutFlash(path string, query url.Values) string {
	query.Del("flash")
	query.Del("flashref")
	query.Del(FlashUndoParam)
	if len(query) == 0 {
		return path
	}
	return path + "?" + query.Encode()
}

// setPriority changes one task's priority and nothing else.
//
// The listing offered a status menu and no way at all to change a priority:
// the chip looked like every other attribute and the only route to editing it
// was the whole task form on the task screen. A priority is a one-word
// decision made while reading a list, which is where this now is.
//
// No version is submitted, so the update carries none and skips the
// optimistic check. That is deliberate: a row has no version to show, and a
// priority is not a field two readers race over the way a body is. The
// transition menu beside it works the same way.
func (h *handler) setPriority(w http.ResponseWriter, r *http.Request) error {
	ref, err := taskRefOf(r)
	if err != nil {
		return err
	}
	priority, err := priorityField(r, "priority")
	if err != nil {
		return err
	}
	if priority == 0 {
		return core.Invalid("no priority given")
	}
	undo := ""
	if before, err := h.svc.GetTask(r.Context(), ref); err == nil && before.Priority != priority {
		undo = undoPriorityPrefix + strconv.Itoa(int(before.Priority))
	}
	if _, err := h.svc.UpdateTask(r.Context(), ref, core.UpdateTaskInput{Priority: &priority}); err != nil {
		return err
	}
	h.afterTransition(w, r, ref, "priority is now "+priorityName(priority), undo)
	return nil
}

// completeTask finishes a task in one click. A workflow rarely allows a jump
// straight from the opening state to a terminal one, so this walks the
// shortest legal path and applies each transition. Doing it any other way
// would mean the tick box works only for tasks that happen to be one step
// from done.
func (h *handler) completeTask(w http.ResponseWriter, r *http.Request) error {
	ref, err := taskRefOf(r)
	if err != nil {
		return err
	}
	task, err := h.svc.GetTask(r.Context(), ref)
	if err != nil {
		return err
	}
	// The project and the workflow are addressed by key on the service, while
	// a task carries their identifiers, so both are matched by id here.
	project, err := h.projectByID(r, task.ProjectID)
	if err != nil {
		return err
	}
	flow, err := h.workflowOf(r, project)
	if err != nil {
		return err
	}
	// The control is a tick box, so it toggles: a finished task reopens into
	// the workflow's starting state. Treating it as complete-only left the
	// click on an already finished row doing nothing at all.
	target, done := doneState(flow.Definition), "completed "
	if flow.Definition.IsTerminal(task.Status) {
		target, done = flow.Definition.Initial, "reopened "
	}
	if target == "" {
		return core.Precondition("workflow %q has no state to move %q into", flow.Key, task.Ref)
	}
	path := pathBetween(flow.Definition, task.Status, target)
	if path == nil {
		return core.Precondition("no route from %q to %q in workflow %q", task.Status, target, flow.Key)
	}
	for _, next := range path {
		if _, err := h.svc.TransitionTask(r.Context(), ref, core.TransitionInput{To: next}); err != nil {
			return err
		}
	}
	// Back to the list the reader was on, at the row they ticked. safeNext
	// refuses anything that is not a relative path, so a crafted next cannot
	// turn this into an open redirect.
	back := safeNext(field(r, "next"))
	if back == RouteProjects {
		// safeNext's fallback is the projects screen, which is the wrong
		// place to land from a task list; an absent or rejected next means
		// the plain list.
		back = RouteTasks
	}
	redirectTo(w, r, back, "t-"+task.Ref, done+output.ShowRef(task.Ref))
	return nil
}

// projectByID finds a project by identifier. The service addresses a project
// by key, while a task carries the identifier.
func (h *handler) projectByID(r *http.Request, id string) (*core.Project, error) {
	projects, _, err := h.allProjects(r, true)
	if err != nil {
		return nil, err
	}
	for i := range projects {
		if projects[i].ID == id {
			return &projects[i], nil
		}
	}
	return nil, core.NotFound("project %q", id)
}

// pathBetween returns the states to move through to get from one state to
// another, excluding the starting state. It returns nil when no route exists,
// and an empty slice when the task is already there.
func pathBetween(d core.WorkflowDefinition, from, to string) []string {
	if from == to {
		return []string{}
	}
	next := map[string][]string{}
	for _, t := range d.Transitions {
		next[t.From] = append(next[t.From], t.To)
	}
	prev := map[string]string{from: ""}
	queue := []string{from}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, n := range next[cur] {
			if _, seen := prev[n]; seen {
				continue
			}
			prev[n] = cur
			if n == to {
				return walkBack(prev, from, to)
			}
			queue = append(queue, n)
		}
	}
	return nil
}

// walkBack turns a breadth-first predecessor map into a forward path.
func walkBack(prev map[string]string, from, to string) []string {
	var out []string
	for at := to; at != from; at = prev[at] {
		out = append(out, at)
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out
}

// unfinishedStatuses is every state this tenant's workflows do not treat as
// terminal. It is the union across workflows, because one listing spans
// projects that may each run a different one.
func (h *handler) unfinishedStatuses(ctx context.Context) ([]string, error) {
	flows, err := h.svc.ListWorkflows(ctx)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var out []string
	for _, flow := range flows {
		for _, state := range flow.Definition.States {
			if state.Terminal || seen[state.Key] {
				continue
			}
			seen[state.Key] = true
			out = append(out, state.Key)
		}
	}
	return out, nil
}

// showAllHref is this listing with the open-work default turned off, keeping
// the reader's filter, deadline window, sort and page size.
//
// The route is named rather than taken from the request, for screenHref's
// reason: the request may have arrived at the live fragment. The cursor and
// its trail go, for narrowHref's reason in reverse: they address positions in
// the narrowed walk, and carrying one onto the wider set names rows this
// reader was never on.
func showAllHref(r *http.Request) string {
	query := r.URL.Query()
	query.Del(CursorParam)
	query.Del(TrailParam)
	query.Del(PageParam)
	query.Del("flash")
	query.Del("flashref")
	query.Del(FlashUndoParam)
	query.Set(AllParam, "1")
	return RouteTasks + "?" + query.Encode()
}
