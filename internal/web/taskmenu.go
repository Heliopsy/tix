// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"html"
	"html/template"
	"net/url"

	"github.com/heliopsy/tix/internal/core"
)

// Places a task menu is drawn in. Each draws the same items; the place
// decides only where the button sits, how a form swaps its answer in, and the
// two extras that make sense in one place alone.
const (
	menuLane  = "lane"
	menuBoard = "board"
	menuList  = "list"
	menuPage  = "page"
)

// taskMenu is one task's three dots menu, the one "task-menu" draws on a
// lane card, a board card, a list row and the task screen alike. What it
// offers is decided by the same laneCard every view uses, so a reader sees
// the same items in the same order wherever the task is shown.
type taskMenu struct {
	laneCard
	Place string
	// ID is the popover's id and Field the prefix of its fields' ids, both
	// unique on the page.
	ID    string
	Field string
	CSRF  string
	// Next is where a form returns to, and Leave where a delete does: the
	// listing on a listing, the task screen and the listing it came from on
	// the task screen.
	Next  string
	Leave string
	// Card is the element a form's answer is swapped into when the form
	// carries its own htmx attributes, a lane card; empty where the row's or
	// the screen's data-inplace decides.
	Card string
	// Inplace is the data-inplace the popover itself carries, on the task
	// screen, where the menu sits outside the region it changes.
	Inplace string
	Star    bool
	// Open is the task's address, empty on the task screen itself.
	Open   string
	Rename string
	// Comment is where a move that needs a comment leads: the task, or on
	// the task screen its comment field.
	Comment string
	// Select is the select mode address, on a list row only.
	Select   string
	Assignee string
	Handles  string
	// Subtasks says the task has subtasks, which only the task screen knows,
	// so its delete can take them along.
	Subtasks bool
	// Reclaim is the popover that ends a live lease, on the task screen only.
	Reclaim    string
	Priorities []priorityChoice
	AssignMe   assignMeForm
	Due        dueFieldForm
	Snooze     snoozeForm
}

// HX is the htmx attributes a form posting to path carries on a lane card,
// where only the card and the toast are swapped and lanes.js puts focus back
// and says a refusal inside the card. Elsewhere it is empty and the region the
// form sits in decides (assets/feel.js).
func (m taskMenu) HX(path string) template.HTMLAttr {
	if m.Card == "" {
		return ""
	}
	c := html.EscapeString(m.Card)
	// #nosec G203 -- the attribute names are fixed here, and both values,
	// a task's route and its card's id, are HTML escaped above.
	return template.HTMLAttr(` hx-post="` + html.EscapeString(path) + `" hx-target="` + c + `" hx-select="` + c +
		`" hx-select-oob="#toast" hx-swap="outerHTML" hx-indicator="` + c +
		`" hx-sync="closest .lane-card:drop" data-lane-control="more"`)
}

// Completes reports whether the menu has a Complete or Reopen section: a
// route to take, or a reason a reader who may move tasks cannot take one.
func (m taskMenu) Completes() bool {
	return len(m.Tick) > 0 || m.Locked.Key != "" && m.Locked.Key != "you cannot move tasks"
}

// newTaskMenu builds a task's menu for a template, from the screen being
// drawn (the view and its tasks or task screen data) and the place it sits.
func newTaskMenu(root any, t core.Task, place string) taskMenu {
	v, _ := root.(view)
	has := func(core.Scope) bool { return false }
	if v.Actor != nil {
		has = v.Actor.HasScope
	}
	m := taskMenu{Place: place, CSRF: v.CSRF, Star: v.CanStar(), Priorities: priorityChoices}
	ref := t.Ref
	switch d := v.Data.(type) {
	case tasksView:
		def, ok := d.Moves[t.ProjectID]
		m.laneCard = newLaneCard(t, def, ok, has)
		m.Next, m.Leave = d.Self, d.Self
		m.Open = "/tasks/" + ref + "?next=" + url.QueryEscape(d.Self)
		m.Rename = m.Open + "&" + RenameParam + "=1"
		m.Comment = m.Open
		m.Assignee = d.Names.Label(t.AssigneeActorID)
		m.Handles = "row-handles"
		if place == menuLane {
			m.Handles = "lane-handles"
		}
		if place == menuList && d.Bulk.Any() && !d.Selecting {
			m.Select = d.SelectHref
		}
	case taskView:
		m.laneCard = newLaneCard(t, d.Workflow, d.HaveWorkflow, has)
		here := withoutRename(v.Here)
		m.Next, m.Leave = here, d.Back
		m.Rename = renameHref(here)
		m.Comment = "#" + v.IDPrefix + "comment-body"
		m.Assignee = d.Names.Label(t.AssigneeActorID)
		m.Handles = "actor-handles"
		m.Subtasks = len(d.Subtasks) > 0
		if d.CanReclaim && claimState(t) == "held" {
			m.Reclaim = v.IDPrefix + "reclaim-" + ref
		}
	default:
		m.laneCard = newLaneCard(t, core.WorkflowDefinition{}, false, has)
	}
	prefix := map[string]string{menuLane: "l", menuBoard: "b", menuList: "r", menuPage: v.IDPrefix + "m"}[place]
	m.ID, m.Field = prefix+"more-"+ref, prefix
	if place == menuPage {
		m.ID = v.IDPrefix + "taskmenu-" + ref
		if !v.InDialog {
			m.Inplace = "main.wrap"
		}
	}
	if place == menuLane {
		m.Card = "#t-" + ref
	}
	m.AssignMe = newAssignMe(ref, t.AssigneeActorID, v.Actor, m.Next, m.CSRF, m.Card)
	m.Due = newDueField(ref, t.DueAt, m.Next, m.CSRF, m.Field+"due-"+ref, m.Card)
	m.Snooze = newSnoozeForm(ref, m.Next, m.CSRF, m.Field+"snooze-"+ref, m.Card)
	if place == menuPage {
		m.Snooze.Target = "snooze"
	}
	return m
}

// RenameParam opens the task screen with its title field showing, which is
// where a menu's Rename leads without scripting.
const RenameParam = "rename"

// renameHref is a task screen's address with its title field open.
func renameHref(here string) string { return withRename(here, true) }

// withoutRename is a task screen's address with its title field shut, so a
// form posted from a screen opened at Rename does not open it again.
func withoutRename(here string) string { return withRename(here, false) }

func withRename(here string, open bool) string {
	u, err := url.Parse(here)
	if err != nil {
		return here
	}
	q := u.Query()
	q.Del(RenameParam)
	if open {
		q.Set(RenameParam, "1")
	}
	u.RawQuery = q.Encode()
	return u.String()
}
