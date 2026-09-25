// SPDX-License-Identifier: AGPL-3.0-or-later

// Package demo fills a database with backdated, realistic work so that every
// interface can be shown against a product in use rather than an empty shell.
package demo

import (
	"context"
	"sort"
	"time"

	"github.com/heliopsy/tix/internal/clock"
	"github.com/heliopsy/tix/internal/core"
)

// DefaultDays is the window a seed covers when the caller names none.
const DefaultDays = 90

// DefaultPassword is what the seeded sign-in accounts are given when the
// caller names no other. It is documented rather than generated because a
// demonstration nobody can open is not a demonstration, and a secret is worth
// nothing on data that exists to be shown; a seed pointed somewhere it should
// not be is refused by HasData, not by an unguessable password.
const DefaultPassword = "tix-demo-password"

// MinDays is the shortest window the fixture can be replayed into without
// collapsing weeks of history onto one afternoon.
const MinDays = 7

// MaxDays bounds the window, because the statistics read every completion
// inside it as a row.
const MaxDays = 3650

// dayDur is one calendar day of the replayed history.
const dayDur = 24 * time.Hour

// Options selects what one seed writes.
type Options struct {
	// Days is how far back the seeded history reaches.
	Days int
	// Now is the instant the history ends at. The statistics screens read a
	// window ending at the real clock, so a seed that stops in the past
	// reports nothing; callers pass the wall clock here.
	Now time.Time
	// Password is given to every sign-in account the fixture creates, and
	// reported back so the caller can print it. Empty means DefaultPassword.
	Password string
}

// Credential is one account the seed left behind that can sign in, reported so
// that the command can say how to open what it just wrote.
type Credential struct {
	Handle   string    `json:"handle" yaml:"handle"`
	Email    string    `json:"email" yaml:"email"`
	Role     core.Role `json:"role" yaml:"role"`
	Password string    `json:"password" yaml:"password"`
}

// Summary counts what a seed wrote.
type Summary struct {
	Projects     int `json:"projects" yaml:"projects"`
	Actors       int `json:"actors" yaml:"actors"`
	FieldDefs    int `json:"field_defs" yaml:"field_defs"`
	Tasks        int `json:"tasks" yaml:"tasks"`
	Completed    int `json:"completed" yaml:"completed"`
	Comments     int `json:"comments" yaml:"comments"`
	Dependencies int `json:"dependencies" yaml:"dependencies"`
	Abandoned    int `json:"abandoned" yaml:"abandoned"`
	// Credentials are the sign-ins the seed wrote, in the order the fixture
	// lists them.
	Credentials []Credential `json:"credentials" yaml:"credentials"`
	Since       time.Time    `json:"since" yaml:"since"`
	Until       time.Time    `json:"until" yaml:"until"`
}

// normalize fills in the defaults and rejects a window nothing fits into.
func (o Options) normalize() (Options, error) {
	if o.Days == 0 {
		o.Days = DefaultDays
	}
	if o.Days < MinDays || o.Days > MaxDays {
		return Options{}, core.Invalid("demo window must be between %d and %d days", MinDays, MaxDays)
	}
	if o.Now.IsZero() {
		return Options{}, core.Invalid("demo window needs the instant it ends at")
	}
	if o.Password == "" {
		o.Password = DefaultPassword
	}
	if len(o.Password) < core.MinPasswordLength {
		return Options{}, core.Invalid("demo password must be at least %d characters", core.MinPasswordLength)
	}
	o.Now = o.Now.UTC()
	return o, nil
}

// HasData reports whether the target already holds work of its own. Projects
// alone do not count: a fresh installation is given starter lists before any
// command runs, so refusing on those would refuse every empty database.
func HasData(ctx context.Context, svc core.Service) (bool, error) {
	page, err := svc.ListTasks(ctx, core.TaskFilter{IncludeDeleted: true, Page: core.Page{Limit: 1}})
	if err != nil {
		return false, err
	}
	if len(page.Tasks) > 0 {
		return true, nil
	}
	users, _, err := svc.ListUsers(ctx, core.Page{Limit: 1})
	if err != nil {
		return false, err
	}
	return len(users) > 0, nil
}

// Reset removes every task and project the target holds, so a seed can be
// replaced rather than stacked on top of the last one.
//
// The accounts stay. Deleting a user leaves the actor row that owns its handle
// behind, and nothing in the product deletes an actor, so a reset that removed
// the users left seven handles no account could ever be created under again:
// the next seed adopted the orphans, wrote no credential, and produced a demo
// with no way in. Seed re-credentials the accounts it finds instead.
func Reset(ctx context.Context, svc core.Service) error {
	if err := resetTasks(ctx, svc); err != nil {
		return err
	}
	return resetProjects(ctx, svc)
}

func resetTasks(ctx context.Context, svc core.Service) error {
	for {
		page, err := svc.ListTasks(ctx, core.TaskFilter{
			IncludeDeleted: true,
			Page:           core.Page{Limit: core.MaxPageLimit},
		})
		if err != nil {
			return err
		}
		if len(page.Tasks) == 0 {
			return nil
		}
		for _, task := range page.Tasks {
			in := core.DeleteTaskInput{Hard: true, Cascade: true}
			err := svc.DeleteTask(ctx, core.TaskRef{ID: task.ID}, in)
			if err != nil && !core.IsKind(err, core.KindNotFound) {
				return err
			}
		}
	}
}

func resetProjects(ctx context.Context, svc core.Service) error {
	for {
		projects, _, err := svc.ListProjects(ctx, core.ProjectFilter{
			IncludeArchived: true,
			Page:            core.Page{Limit: core.MaxPageLimit},
		})
		if err != nil {
			return err
		}
		if len(projects) == 0 {
			return nil
		}
		for _, p := range projects {
			if err := svc.DeleteProject(ctx, p.ID); err != nil && !core.IsKind(err, core.KindNotFound) {
				return err
			}
		}
	}
}

// Seed replays the fixture through svc, moving clk to the instant each record
// is meant to have happened at.
//
// It goes through the ordinary service calls rather than writing rows because
// the statistics attribute a completion to the actor of the audit entry
// written at exactly the task's completed_at. Rows inserted behind the service
// carry no such entry, which leaves every leaderboard and lead-time figure
// empty -- the one thing this fixture exists to fill.
func Seed(ctx context.Context, svc core.Service, clk *clock.Fake, admin *core.Actor, opts Options) (*Summary, error) {
	opts, err := opts.normalize()
	if err != nil {
		return nil, err
	}
	if admin == nil {
		return nil, core.Invalid("seeding needs the actor the fixture is written by")
	}
	s := &seeder{
		svc:    svc,
		clk:    clk,
		admin:  admin,
		opts:   opts,
		start:  opts.Now.Add(-time.Duration(opts.Days) * dayDur),
		actors: map[string]*core.Actor{},
		refs:   map[string]string{},
	}
	return s.run(ctx)
}

// seeder carries the state one replay builds up.
type seeder struct {
	svc   core.Service
	clk   *clock.Fake
	admin *core.Actor
	opts  Options

	start   time.Time
	actors  map[string]*core.Actor
	refs    map[string]string
	summary Summary
}

// event is one record of the fixture, at the instant it happened.
type event struct {
	at time.Time
	do func(context.Context) error
}

func (s *seeder) run(ctx context.Context) (*Summary, error) {
	s.clk.Set(s.start)
	if err := s.createProjects(ctx); err != nil {
		return nil, err
	}
	if err := s.createActors(ctx); err != nil {
		return nil, err
	}
	if err := s.createFieldDefs(ctx); err != nil {
		return nil, err
	}

	events, err := s.plan()
	if err != nil {
		return nil, err
	}
	sort.SliceStable(events, func(i, j int) bool { return events[i].at.Before(events[j].at) })
	last := s.start
	for _, e := range events {
		s.clk.Set(e.at)
		if err := e.do(ctx); err != nil {
			return nil, err
		}
		last = e.at
	}
	if err := s.abandonClaims(ctx, last); err != nil {
		return nil, err
	}

	s.clk.Set(s.opts.Now)
	s.summary.Since, s.summary.Until = s.start, s.opts.Now
	return &s.summary, nil
}

// at places a fixture day and hour inside the requested window. The fixture is
// written against a fixed number of days, so a shorter or longer window
// compresses or stretches it instead of leaving one end empty.
func (s *seeder) at(day, hour int) time.Time {
	scaled := day * s.opts.Days / fixtureDays
	return s.start.Add(time.Duration(scaled)*dayDur + time.Duration(hour)*time.Hour)
}

// as returns ctx carrying the seeded identity behind handle.
func (s *seeder) as(ctx context.Context, handle string) (context.Context, error) {
	actor, ok := s.actors[handle]
	if !ok {
		return nil, core.Invalid("fixture names actor %q, which it never creates", handle)
	}
	return core.WithSource(core.WithActor(ctx, actor), core.SourceCLI), nil
}

// adminCtx returns ctx carrying the actor the fixture itself is written by.
func (s *seeder) adminCtx(ctx context.Context) context.Context {
	return core.WithSource(core.WithActor(ctx, s.admin), core.SourceCLI)
}

// dropEmptyLists removes the lists a fresh installation is given before the
// fixture writes its own.
//
// A demo tenant is meant to read as one team's workspace, and the starter
// lists are neither part of it nor visibly empty: they pad the Projects
// screen, the project filter and the tenant tree with four names nothing ever
// happens in. Reset already deletes every project, so a re-seed showed six
// lists while a first seed showed ten; this makes both agree. Only a list
// holding no task at all is dropped, so nothing that has been worked in can be
// lost to a seed.
func (s *seeder) dropEmptyLists(ctx context.Context) error {
	admin := s.adminCtx(ctx)
	existing, _, err := s.svc.ListProjects(admin, core.ProjectFilter{
		IncludeArchived: true,
		Page:            core.Page{Limit: core.MaxPageLimit},
	})
	if err != nil {
		return err
	}
	for _, p := range existing {
		page, err := s.svc.ListTasks(admin, core.TaskFilter{
			ProjectIDs:     []string{p.ID},
			IncludeDeleted: true,
			Page:           core.Page{Limit: 1},
		})
		if err != nil {
			return err
		}
		if len(page.Tasks) > 0 {
			continue
		}
		if err := s.svc.DeleteProject(admin, p.ID); err != nil && !core.IsKind(err, core.KindNotFound) {
			return err
		}
	}
	return nil
}

func (s *seeder) createProjects(ctx context.Context) error {
	admin := s.adminCtx(ctx)
	if err := s.dropEmptyLists(ctx); err != nil {
		return err
	}
	for _, p := range projects {
		in := core.CreateProjectInput{
			Key:         p.key,
			Name:        p.name,
			Description: p.description,
			Color:       string(p.color),
			Icon:        p.icon,
		}
		if _, err := s.svc.CreateProject(admin, in); err != nil {
			return err
		}
		s.summary.Projects++
	}
	return nil
}

// createActors creates every seeded identity as a user, and mints an API token
// for the ones the fixture calls agents.
//
// An agent is an actor holding a token: the product has no other route to one,
// because the actor row is created with the account and the token is what
// makes a caller carrying it an agent. The fixture therefore acts as the agent
// exactly as the token verifier would describe it.
func (s *seeder) createActors(ctx context.Context) error {
	admin := s.adminCtx(ctx)
	for _, p := range people {
		id, err := s.createPerson(admin, p)
		if err != nil {
			return err
		}
		actor := &core.Actor{
			ID:          id,
			TenantID:    s.admin.TenantID,
			Kind:        core.ActorUser,
			Handle:      p.handle,
			DisplayName: p.name,
			Role:        p.role,
		}
		if p.agent {
			token, err := s.svc.CreateToken(admin, core.CreateTokenInput{
				Name:    p.handle + "-runner",
				ActorID: id,
				Scopes:  agentScopes,
			})
			if err != nil {
				return err
			}
			actor.Kind, actor.Role = core.ActorAgent, ""
			actor.Scopes, actor.TokenID = token.Scopes, token.ID
		}
		if p.signIn {
			s.summary.Credentials = append(s.summary.Credentials, Credential{
				Handle: p.handle, Email: p.email, Role: p.role, Password: s.opts.Password,
			})
		}
		s.actors[p.handle] = actor
		s.summary.Actors++
	}
	if len(s.summary.Credentials) == 0 {
		return core.Invalid("the fixture leaves no account that can sign in")
	}
	return nil
}

// createPerson returns the identifier of the account behind one seeded person,
// creating it through the same call tix user create makes so that a seeded
// account and a hand-made one are the same kind of record.
func (s *seeder) createPerson(ctx context.Context, p personSeed) (string, error) {
	in := core.CreateUserInput{
		Email:       p.email,
		DisplayName: p.name,
		Handle:      p.handle,
		Role:        p.role,
	}
	if p.signIn {
		in.Password = s.opts.Password
	}
	user, err := s.svc.CreateUser(ctx, in)
	switch {
	case err == nil:
		return user.ID, nil
	case !core.IsKind(err, core.KindConflict):
		return "", err
	}
	return s.adoptPerson(ctx, p)
}

// adoptPerson takes over the account already holding a seeded handle and puts
// this seed's password on it, so a second seed leaves the credentials it
// reports rather than the ones the first seed happened to use.
//
// An actor row owns the handle and outlives its user, and the product has no
// call that deletes an actor. A handle standing with no account behind it can
// therefore never be credentialed again, so that is reported instead of
// seeding a history whose authors nobody can sign in as.
func (s *seeder) adoptPerson(ctx context.Context, p personSeed) (string, error) {
	existing, err := s.svc.GetActor(ctx, p.handle)
	if err != nil {
		return "", err
	}
	switch _, err := s.svc.GetUser(ctx, existing.ID); {
	case core.IsKind(err, core.KindNotFound):
		return "", core.Conflict("handle %q is held by an actor with no account, "+
			"which cannot be given one; seed a fresh database", p.handle)
	case err != nil:
		return "", err
	}
	role := p.role
	update := core.UpdateUserInput{DisplayName: &p.name, Role: &role}
	if p.signIn {
		update.Password = &s.opts.Password
	}
	if _, err := s.svc.UpdateUser(ctx, existing.ID, update); err != nil {
		return "", err
	}
	return existing.ID, nil
}

func (s *seeder) createFieldDefs(ctx context.Context) error {
	admin := s.adminCtx(ctx)
	for _, f := range fieldDefs {
		if _, err := s.svc.PutFieldDef(admin, f.project, f.def); err != nil {
			return err
		}
		s.summary.FieldDefs++
	}
	return nil
}

// plan turns the fixture into the events that will be replayed.
func (s *seeder) plan() ([]event, error) {
	out := make([]event, 0, len(tasks)*3)
	for _, t := range tasks {
		seed := t
		born := s.at(seed.created, seed.hour)
		out = append(out, event{at: born, do: func(ctx context.Context) error {
			return s.createTask(ctx, seed)
		}})
		steps, err := flowOf(seed)
		if err != nil {
			return nil, err
		}
		// A narrow window scales several fixture days onto one, which can put
		// a later move at an earlier hour than the move before it. Each event
		// of a task is therefore held after the one it follows, so no window
		// can ask for a task to be moved before it was created.
		last := born
		for _, st := range steps {
			step, at := st, after(s.at(st.day, st.hour), last)
			last = at
			out = append(out, event{at: at, do: func(ctx context.Context) error {
				return s.transition(ctx, seed.key, step)
			}})
		}
		for _, c := range seed.comments {
			comment, at := c, after(s.at(c.day, c.hour), born)
			out = append(out, event{at: at, do: func(ctx context.Context) error {
				return s.comment(ctx, seed.key, comment)
			}})
		}
	}
	return out, nil
}

// after returns at, moved past floor when a compressed window pulled it back
// before the event it must follow.
func after(at, floor time.Time) time.Time {
	if at.After(floor) {
		return at
	}
	return floor.Add(time.Minute)
}

func (s *seeder) createTask(ctx context.Context, seed taskSeed) error {
	actorCtx, err := s.as(ctx, seed.creator)
	if err != nil {
		return err
	}
	in := core.CreateTaskInput{
		ProjectRef:      seed.project,
		Title:           seed.title,
		Body:            seed.body,
		Priority:        seed.priority,
		Tags:            seed.tags,
		CustomFields:    seed.fields,
		AssigneeActorID: s.actors[seed.assignee].ID,
	}
	if seed.due != noDue {
		due := s.at(seed.due, 17)
		in.DueAt = &due
	}
	if seed.parent != "" {
		if in.ParentRef, err = s.ref(seed.parent); err != nil {
			return err
		}
	}
	for _, key := range seed.dependsOn {
		ref, err := s.ref(key)
		if err != nil {
			return err
		}
		in.DependsOn = append(in.DependsOn, ref)
	}
	task, err := s.svc.CreateTask(actorCtx, in)
	if err != nil {
		return err
	}
	s.refs[seed.key] = task.Ref
	s.summary.Tasks++
	s.summary.Dependencies += len(in.DependsOn)
	return nil
}

func (s *seeder) transition(ctx context.Context, key string, st step) error {
	actorCtx, err := s.as(ctx, st.by)
	if err != nil {
		return err
	}
	ref, err := s.ref(key)
	if err != nil {
		return err
	}
	parsed, err := core.ParseTaskRef(ref)
	if err != nil {
		return err
	}
	task, err := s.svc.TransitionTask(actorCtx, parsed, core.TransitionInput{To: st.to})
	if err != nil {
		return err
	}
	if task.CompletedAt != nil {
		s.summary.Completed++
	}
	return nil
}

// abandonClaims replays the claims an agent took and never gave back.
//
// It drives the ordinary claim and sweep calls rather than writing the
// evidence columns, because the columns are only half of the state: the row
// means "this holder stopped answering", and a row carrying that without the
// claim and lease-expiry entries behind it would be a screenshot of a history
// that never happened.
//
// The instants come from the end of the window rather than from a fixture day.
// The evidence reads as recent for core.LeaseExpiryEvidenceWindow measured
// against the wall clock of whoever is looking, and the fixture days are
// backdated by months, so a lapse placed among them would be long stale before
// the seed finished. floor keeps the replay moving forwards when a short
// window has already put the last event close to the end.
func (s *seeder) abandonClaims(ctx context.Context, floor time.Time) error {
	admin := s.adminCtx(ctx)
	// Oldest lapse first: the replay only moves forwards, so an entry reached
	// out of order would be dragged up to the one before it and lose the gap
	// the fixture gave it.
	ordered := append([]abandonedSeed(nil), abandoned...)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].ago > ordered[j].ago })
	for _, a := range ordered {
		holder, err := s.as(ctx, a.holder)
		if err != nil {
			return err
		}
		ref, err := s.ref(a.key)
		if err != nil {
			return err
		}
		parsed, err := core.ParseTaskRef(ref)
		if err != nil {
			return err
		}
		if a.claims < 1 {
			return core.Invalid("abandoned task %q is dropped %d times", a.key, a.claims)
		}
		lapsed := s.opts.Now.Add(-a.ago)
		for i := 0; i < a.claims; i++ {
			back := time.Duration(a.claims-1-i) * (abandonTTL + abandonSweepDelay + abandonRetryGap)
			at := after(lapsed.Add(-back-abandonTTL), floor)
			s.clk.Set(at)
			in := core.ClaimInput{TTL: core.Duration(abandonTTL)}
			if _, err := s.svc.ClaimTask(holder, parsed, in); err != nil {
				return err
			}
			floor = at.Add(abandonTTL + abandonSweepDelay)
			s.clk.Set(floor)
			if _, err := s.svc.SweepLeases(admin, core.MaxPageLimit); err != nil {
				return err
			}
		}
		s.summary.Abandoned++
	}
	return nil
}

func (s *seeder) comment(ctx context.Context, key string, c commentSeed) error {
	actorCtx, err := s.as(ctx, c.by)
	if err != nil {
		return err
	}
	ref, err := s.ref(key)
	if err != nil {
		return err
	}
	parsed, err := core.ParseTaskRef(ref)
	if err != nil {
		return err
	}
	if _, err := s.svc.AddComment(actorCtx, parsed, c.body); err != nil {
		return err
	}
	s.summary.Comments++
	return nil
}

// ref returns the reference a fixture key was given when its task was created.
func (s *seeder) ref(key string) (string, error) {
	ref, ok := s.refs[key]
	if !ok {
		return "", core.Invalid("fixture refers to task %q before it is created", key)
	}
	return ref, nil
}

// step is one move of a task through its workflow.
type step struct {
	day  int
	hour int
	to   string
	by   string
}

// flowOf derives the moves that take a seeded task to the state it ends in.
func flowOf(seed taskSeed) ([]step, error) {
	if seed.end == outcomeTodo {
		return nil, nil
	}
	started := step{day: seed.started, hour: seed.hour + 2, to: statusDoing, by: seed.assignee}
	switch seed.end {
	case outcomeDoing:
		return []step{started}, nil
	case outcomeBlocked:
		return []step{started, {day: seed.started + 1, hour: 11, to: statusBlocked, by: seed.assignee}}, nil
	case outcomeDone:
		return []step{started, {day: seed.done, hour: seed.doneHour, to: statusDone, by: seed.doneBy}}, nil
	default:
		return nil, core.Invalid("task %q ends in unknown state %q", seed.key, seed.end)
	}
}
