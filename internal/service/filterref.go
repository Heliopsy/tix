// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"sort"
	"strings"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// maxNamedStatuses bounds the vocabulary an unknown-status error prints. A
// tenant may define many workflows, and an error that lists every state of
// every one of them stops being a hint and becomes a wall of text.
const maxNamedStatuses = 12

// filterRefs resolves the reference-shaped terms of a task filter against the
// store, once per listing.
//
// The rule the whole type exists to enforce: a filter term that names nothing
// must fail the listing, never quietly select no rows. An empty result with a
// zero exit status is the one answer that cannot be told apart from a correct
// one, so "that project does not exist" arrives as an error rather than as
// "that project is empty".
//
// Which terms qualify is a judgement per term, not a blanket rule, and the
// judgement is recorded on each method. Tags are deliberately absent: see
// resolveTaskFilter.
//
// Every lookup is memoised, and every cache is shared between the inclusion
// list and the exclusion list, so a value named on both sides is resolved
// once. The cost of a listing stays proportional to the distinct references
// its filter names rather than to the rows it returns.
type filterRefs struct {
	tx store.Tx

	actors   map[string]string
	projects map[string]*core.Project

	// statuses maps a folded state key to the one canonical spelling, and is
	// loaded at most once per listing, only if a status term is present.
	statuses map[string]string
	loaded   bool
}

func newFilterRefs(tx store.Tx) *filterRefs {
	return &filterRefs{
		tx:       tx,
		actors:   map[string]string{},
		projects: map[string]*core.Project{},
	}
}

// resolveTaskFilter rewrites every reference-shaped term of a filter to the
// stored form, failing the listing when a term names nothing.
//
// Tags are the deliberate exception. A tag is free-form and comes into being
// by being applied, so there is no declaration against which one could be
// called unknown: the only evidence a tag exists is that some task carries it,
// which makes "unknown tag" and "tag with no tasks" the same state. Refusing
// the first would refuse the second, so removing the last task from a tag
// would turn a working filter into an error, and "-tag:ops" over a vocabulary
// nobody has declared would fail for naming a tag that is precisely what the
// caller wants absent. An empty answer to a tag term is therefore the truthful
// answer, not the silence this function removes elsewhere.
func resolveTaskFilter(ctx context.Context, tx store.Tx, f *core.TaskFilter) error {
	r := newFilterRefs(tx)
	if err := r.resolveActors(ctx, f); err != nil {
		return err
	}
	if err := r.resolveProjects(ctx, f); err != nil {
		return err
	}
	if err := r.resolveStatuses(ctx, f); err != nil {
		return err
	}
	return r.resolveParent(ctx, f)
}

// resolveActors resolves every actor-shaped term. Assignee, creator and
// claimant all name a row in the same directory and all reached storage as
// raw identifiers, so a handle in any of the three selected nothing.
func (r *filterRefs) resolveActors(ctx context.Context, f *core.TaskFilter) error {
	lists := []*[]string{
		&f.AssigneeIDs, &f.Exclude.AssigneeIDs,
		&f.CreatorIDs, &f.Exclude.CreatorIDs,
		&f.ClaimedBy, &f.Exclude.ClaimedBy,
	}
	for _, list := range lists {
		resolved, err := resolveAssigneeRefs(ctx, r.tx, *list, r.actors)
		if err != nil {
			return err
		}
		*list = resolved
	}
	return nil
}

// resolveProjects resolves every project-shaped term.
//
// A project key names a row that exists or does not, so an unknown one is a
// typo and nothing else: there is no reading under which "nosuchproject" is a
// question worth answering with silence. Resolution also settles the spelling
// through the same case-insensitive lookup the rest of the product uses,
// because the store matches a key exactly and "INFRA" therefore selected
// nothing at all where it plainly meant the project called infra.
func (r *filterRefs) resolveProjects(ctx context.Context, f *core.TaskFilter) error {
	keys := []*[]string{&f.ProjectKeys, &f.Exclude.ProjectKeys}
	for _, list := range keys {
		resolved, err := r.mapProjects(ctx, *list, func(p *core.Project) string { return p.Key })
		if err != nil {
			return err
		}
		*list = resolved
	}
	resolved, err := r.mapProjects(ctx, f.ProjectIDs, func(p *core.Project) string { return p.ID })
	if err != nil {
		return err
	}
	f.ProjectIDs = resolved
	return nil
}

func (r *filterRefs) mapProjects(ctx context.Context, refs []string, pick func(*core.Project) string) ([]string, error) {
	if len(refs) == 0 {
		return refs, nil
	}
	out := make([]string, 0, len(refs))
	for _, ref := range refs {
		trimmed := strings.TrimSpace(ref)
		if trimmed == "" {
			out = append(out, ref)
			continue
		}
		project, ok := r.projects[trimmed]
		if !ok {
			found, err := lookupProject(ctx, r.tx, trimmed)
			if err != nil {
				if core.IsKind(err, core.KindNotFound) {
					return nil, core.NotFound("no project with key or id %q", trimmed)
				}
				return nil, err
			}
			r.projects[trimmed], project = found, found
		}
		out = append(out, pick(project))
	}
	return out, nil
}

// resolveStatuses checks every status term against the tenant's workflows.
//
// A status is not a row of its own: it is a state some workflow declares, and
// a state valid in one project may be undefined in another. That makes the
// scope of the check the question. Checking against the workflow of the
// project in scope would refuse a legitimate query, because a listing across
// two projects, or across the tenant, may reasonably name a status only one
// workflow defines and should answer with that workflow's tasks. Checking
// against nothing is the defect.
//
// So the vocabulary is the union of the states of every workflow the tenant
// has. A status no workflow declares cannot describe any task and is a typo. A
// status some workflow declares is a real thing that this listing happens not
// to reach, and an empty result there is the truthful answer, not silence.
//
// Spelling is settled the same way a project key is, and for the same reason.
func (r *filterRefs) resolveStatuses(ctx context.Context, f *core.TaskFilter) error {
	for _, list := range []*[]string{&f.Statuses, &f.Exclude.Statuses} {
		resolved, err := r.mapStatuses(ctx, *list)
		if err != nil {
			return err
		}
		*list = resolved
	}
	return nil
}

func (r *filterRefs) mapStatuses(ctx context.Context, terms []string) ([]string, error) {
	if len(terms) == 0 {
		return terms, nil
	}
	if err := r.loadStatuses(ctx); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(terms))
	for _, term := range terms {
		trimmed := strings.TrimSpace(term)
		if trimmed == "" {
			out = append(out, term)
			continue
		}
		canonical, ok := r.statuses[strings.ToLower(trimmed)]
		if !ok {
			return nil, core.NotFound("no workflow of this tenant defines a status %q; the defined statuses are %s",
				trimmed, strings.Join(r.knownStatuses(), ", "))
		}
		out = append(out, canonical)
	}
	return out, nil
}

func (r *filterRefs) loadStatuses(ctx context.Context) error {
	if r.loaded {
		return nil
	}
	workflows, err := r.tx.ListWorkflows(ctx)
	if err != nil {
		return err
	}
	r.statuses = map[string]string{}
	for _, w := range workflows {
		for _, s := range w.Definition.States {
			folded := strings.ToLower(s.Key)
			// First spelling wins, so a state two workflows disagree about
			// the case of resolves to one of them rather than to whichever
			// workflow was listed last.
			if _, seen := r.statuses[folded]; !seen {
				r.statuses[folded] = s.Key
			}
		}
	}
	r.loaded = true
	return nil
}

func (r *filterRefs) knownStatuses() []string {
	out := make([]string, 0, len(r.statuses))
	for _, canonical := range r.statuses {
		out = append(out, canonical)
	}
	sort.Strings(out)
	if len(out) > maxNamedStatuses {
		out = append(out[:maxNamedStatuses:maxNamedStatuses], "...")
	}
	return out
}

// resolveParent resolves the parent term to a task identifier.
//
// A parent names one task, so the reasoning is the project key's. It is also
// the term where silence was most misleading: the store matches the parent
// column, which holds an identifier, so "parent:infra-42" written in the
// reference form the rest of the product shows a reader selected nothing while
// looking entirely correct.
func (r *filterRefs) resolveParent(ctx context.Context, f *core.TaskFilter) error {
	ref := strings.TrimSpace(f.ParentID)
	if ref == "" {
		return nil
	}
	parent, err := taskByRef(ctx, r.tx, ref)
	if err != nil {
		return err
	}
	f.ParentID = parent.ID
	return nil
}

// resolveStatusList checks the status terms of a queue filter.
//
// A claim answers an unmatched filter with no_task_available, which the
// published guidance tells an agent to read as "nothing to do right now". A
// status the workflows never declare would therefore be reported as an idle
// queue, which is the same wrong conclusion the listing used to invite.
func resolveStatusList(ctx context.Context, tx store.Tx, statuses []string) ([]string, error) {
	return newFilterRefs(tx).mapStatuses(ctx, statuses)
}
