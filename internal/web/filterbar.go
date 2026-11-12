// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/query"
)

// OrderParam turns the listing's sort around: "desc" lists the sort field
// from its far end. Anything else is the ascending default.
const OrderParam = "order"

// OrderDescending is the one value OrderParam reads.
const OrderDescending = "desc"

// pickPrefix and wasPrefix name the Filters popover's pickers and the value
// each was drawn with. A picker is applied only when the two differ, so a term
// typed into the box in the same submission is not overwritten by a picker
// that was never touched.
const (
	pickPrefix = "pick_"
	wasPrefix  = "was_"
	pickKeep   = "*"
)

// pickedKeys are the terms a picker replaces. A tag picker only adds.
var pickedKeys = []string{"status", "priority", "assignee"}

// pickTag is the tag picker's key.
const pickTag = "tag"

// listingDropped are the parameters a change of filter leaves behind: the
// position in the old selection, and what belonged to the page it was on.
var listingDropped = []string{CursorParam, TrailParam, PageParam, "flash", "flashref", FlashUndoParam, DialogParam}

// foldPicks applies the Filters popover's pickers to an expression.
func foldPicks(expression string, form url.Values) string {
	terms := query.Terms(expression)
	for _, key := range pickedKeys {
		if !form.Has(pickPrefix + key) {
			continue
		}
		v := strings.TrimSpace(form.Get(pickPrefix + key))
		if v == pickKeep || v == strings.TrimSpace(form.Get(wasPrefix+key)) {
			continue
		}
		terms = slices.DeleteFunc(terms, func(t query.Term) bool { return t.Key == key && !t.Negated })
		if v != "" {
			raw := query.Quote(key, strings.TrimPrefix(v, query.AssigneePrefix))
			terms = append(terms, query.Term{Raw: raw})
		}
	}
	if tag := strings.TrimSpace(strings.TrimPrefix(form.Get(pickPrefix+pickTag), "#")); tag != "" {
		if !slices.ContainsFunc(terms, func(t query.Term) bool {
			return t.Key == pickTag && !t.Negated && strings.EqualFold(t.Value, tag)
		}) {
			terms = append(terms, query.Term{Raw: query.Quote(pickTag, tag)})
		}
	}
	return query.Join(terms)
}

// hasPicks reports whether a request carries the Filters popover's pickers.
func hasPicks(q url.Values) bool {
	for k := range q {
		if strings.HasPrefix(k, pickPrefix) || strings.HasPrefix(k, wasPrefix) {
			return true
		}
	}
	return false
}

// redirectPicks answers a submission of the Filters popover with the address
// it means: the pickers folded into q, the empty fields and the page position
// gone. It reports whether it answered.
func redirectPicks(w http.ResponseWriter, r *http.Request, route string) bool {
	in := r.URL.Query()
	if !hasPicks(in) {
		return false
	}
	out := url.Values{}
	for k, vals := range in {
		if strings.HasPrefix(k, pickPrefix) || strings.HasPrefix(k, wasPrefix) || slices.Contains(listingDropped, k) {
			continue
		}
		for _, v := range vals {
			if strings.TrimSpace(v) != "" {
				out.Add(k, v)
			}
		}
	}
	out.Del("q")
	if q := strings.TrimSpace(foldPicks(in.Get("q"), in)); q != "" {
		out.Set("q", q)
	}
	target := route
	if len(out) > 0 {
		target += "?" + out.Encode()
	}
	// #nosec G710 -- the target is this screen's own route, a constant or a
	// project key the router already resolved, with an encoded query.
	http.Redirect(w, r, target, http.StatusSeeOther)
	return true
}

// filterChip is one active term under the toolbar, with the link that drops
// it and nothing else.
type filterChip struct {
	Key     string
	Value   string
	Negated bool
	Raw     string
	Href    string
}

// Label is what the chip reads: the term as the language writes it.
func (c filterChip) Label() string {
	if c.Key == "" {
		return c.Raw
	}
	neg := ""
	if c.Negated {
		neg = query.NegationPrefix
	}
	return neg + c.Key + ": " + c.Value
}

// chipBase is the address a chip's link starts from: the screen as it is,
// less its position and the page-only fields.
func chipBase(r *http.Request) url.Values {
	base := r.URL.Query()
	for _, k := range listingDropped {
		base.Del(k)
	}
	return base
}

// chipHref is the screen at base with q, the deadline window and the project
// control set as given.
func chipHref(route string, base url.Values, set map[string]string) string {
	q := url.Values{}
	for k, vals := range base {
		q[k] = append([]string(nil), vals...)
	}
	for k, v := range set {
		if v == "" {
			q.Del(k)
		} else {
			q.Set(k, v)
		}
	}
	for k, vals := range q {
		if len(vals) == 1 && strings.TrimSpace(vals[0]) == "" {
			q.Del(k)
		}
	}
	if len(q) == 0 {
		return route
	}
	return route + "?" + q.Encode()
}

// filterChips lists the terms the screen is answering, each with a link to the
// screen without it. On a project page the project comes from the path and
// is not a chip.
func filterChips(r *http.Request, route string, scoped bool) []filterChip {
	in := r.URL.Query()
	base := chipBase(r)
	var out []filterChip
	if due := strings.TrimSpace(in.Get(DueParam)); due != "" {
		out = append(out, filterChip{Key: "due", Value: due, Raw: "due:" + due,
			Href: chipHref(route, base, map[string]string{DueParam: ""})})
	}
	if !scoped {
		picked := projectPick(in)
		for i, key := range picked {
			rest := slices.Delete(slices.Clone(picked), i, i+1)
			out = append(out, filterChip{Key: "project", Value: key, Raw: "project:" + key,
				Href: chipHref(route, base, map[string]string{ProjectParam: strings.Join(rest, ",")})})
		}
	}
	terms := query.Terms(in.Get("q"))
	for i, t := range terms {
		rest := slices.Delete(slices.Clone(terms), i, i+1)
		out = append(out, filterChip{Key: t.Key, Value: t.Value, Negated: t.Negated, Raw: t.Raw,
			Href: chipHref(route, base, map[string]string{"q": query.Join(rest)})})
	}
	if in.Get(AllParam) != "" {
		out = append(out, filterChip{Key: AllParam, Raw: AllParam,
			Href: chipHref(route, base, map[string]string{AllParam: ""})})
	}
	return out
}

// pickState is what one picker shows: the single selecting term of its key,
// or Several when the expression holds more than one.
type pickState struct {
	Value   string
	Several bool
}

// pickStates reads each picker's state off the expression in the box.
func pickStates(expression string) map[string]pickState {
	out := map[string]pickState{}
	for _, t := range query.Terms(expression) {
		if t.Negated || !slices.Contains(pickedKeys, t.Key) {
			continue
		}
		s, seen := out[t.Key]
		if seen {
			s.Several, s.Value = true, ""
		} else {
			s.Value = t.Value
		}
		out[t.Key] = s
	}
	return out
}

// pickOption is one choice of a picker.
type pickOption struct {
	Value    string
	Label    string
	Selected bool
}

// filterPickers is what the Filters popover draws.
type filterPickers struct {
	Status      []pickOption
	StatusWas   string
	Several     map[string]bool
	Priority    []pickOption
	PriorityWas string
	Assignee    string
}

// newFilterPickers draws the pickers for an expression: the statuses are the
// states of the workflows the screen's projects run, in the order met.
func newFilterPickers(expression string, projects []core.Project, workflows map[string]core.WorkflowDefinition) filterPickers {
	states := pickStates(expression)
	p := filterPickers{Several: map[string]bool{}}
	for k, s := range states {
		p.Several[k] = s.Several
	}
	seen := map[string]bool{}
	for _, pr := range projects {
		for _, st := range workflows[pr.ID].States {
			if seen[strings.ToLower(st.Key)] {
				continue
			}
			seen[strings.ToLower(st.Key)] = true
			p.Status = append(p.Status, pickOption{Value: st.Key, Label: core.StateLabel(st),
				Selected: strings.EqualFold(st.Key, states["status"].Value)})
		}
	}
	p.StatusWas = pickWas(states["status"])
	for _, c := range priorityChoices {
		p.Priority = append(p.Priority, pickOption{Value: c.Label, Label: c.Label,
			Selected: strings.EqualFold(c.Label, states["priority"].Value)})
	}
	p.PriorityWas = pickWas(states["priority"])
	p.Assignee = states["assignee"].Value
	return p
}

// pickWas is the value a picker reports it was drawn with.
func pickWas(s pickState) string {
	if s.Several {
		return pickKeep
	}
	return s.Value
}

// filterSuggest is the data the filter box suggests from, written into the
// page once so typing costs no request.
type filterSuggest struct {
	Keys     []string         `json:"keys"`
	Project  []suggestProject `json:"project"`
	Tag      []string         `json:"tag"`
	Person   []string         `json:"person"`
	Status   []string         `json:"status"`
	Priority []string         `json:"priority"`
	Due      []string         `json:"due"`
	Is       []string         `json:"is"`
}

// suggestProject is one project the box can complete.
type suggestProject struct {
	Key  string `json:"k"`
	Name string `json:"n"`
}

// suggestKeys are the keys the box offers for a bare word, the ones a reader
// types most, in that order.
var suggestKeys = []string{"project", "tag", "assignee", "status", "priority", "due", "is", "title", "creator", "has"}

// suggestIs are the is: states the box offers.
var suggestIs = []string{"starred", "waiting-on", "claimed", "unclaimed", "recurring", "snoozed", "root"}

// newFilterSuggest gathers the suggestions from what the screen loaded.
func newFilterSuggest(projects []core.Project, tags []core.Tag, handles []string, pickers filterPickers, scoped bool) filterSuggest {
	s := filterSuggest{Keys: suggestKeys, Due: FilterDueWindows, Is: suggestIs, Person: handles}
	if scoped {
		s.Keys = slices.DeleteFunc(slices.Clone(suggestKeys), func(k string) bool { return k == "project" })
	} else {
		for _, p := range projects {
			s.Project = append(s.Project, suggestProject{Key: p.Key, Name: p.Name})
		}
	}
	for _, t := range tags {
		s.Tag = append(s.Tag, t.Name)
	}
	for _, o := range pickers.Status {
		s.Status = append(s.Status, o.Value)
	}
	for _, o := range pickers.Priority {
		s.Priority = append(s.Priority, o.Value)
	}
	if s.Person == nil {
		s.Person = []string{}
	}
	return s
}

// pageSizes are the page sizes the More menu offers besides the default.
var pageSizes = []int{25, 50, 100, 200}

// sortDirection reads the direction off a request.
func sortDirection(q url.Values) core.SortDirection {
	if q.Get(OrderParam) == OrderDescending {
		return core.Descending
	}
	return core.Ascending
}

// Suggest is what the filter box completes from. A method rather than a field,
// so the actor directory is listed only when the toolbar is drawn, never for a
// live refresh of the listing.
func (v tasksView) Suggest() filterSuggest {
	return newFilterSuggest(v.Projects, v.Tags, v.Names.Handles(), v.Pickers, v.Scope != nil)
}

// quickSuggest is what the quick-add box completes from: the projects for
// +key, the tags for #tag and the people for @handle.
type quickSuggest struct {
	Project []suggestProject `json:"project"`
	Tag     []string         `json:"tag"`
	Person  []string         `json:"person"`
}

// QuickSuggest is what the quick-add box completes from. Unlike Suggest it
// keeps every project on a project page, where +key adds to another one.
func (v tasksView) QuickSuggest() quickSuggest {
	s := newFilterSuggest(v.Projects, v.Tags, v.Names.Handles(), filterPickers{}, false)
	return quickSuggest{Project: s.Project, Tag: s.Tag, Person: s.Person}
}
