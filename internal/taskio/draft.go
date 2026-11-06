// SPDX-License-Identifier: AGPL-3.0-or-later

package taskio

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/heliopsy/tix/internal/core"
)

// Draft is one row normalised into tix fields, not yet checked against the database.
type Draft struct {
	Line        int
	ExternalID  string
	Aliases     []string
	Ref         string
	Title       string
	Body        string
	Status      string
	StatusRaw   string
	Priority    core.Priority
	PriorityRaw string
	Assignee    string
	AssigneeRaw string
	Tags        []string
	Due         *time.Time
	Parent      string
	DependsOn   []string
	Recurrence  string
	Custom      map[string]string
	Errors      []string
	Skip        string
}

type draftBuilder struct {
	m        *Mapping
	d        Draft
	body     []string
	appendix []string
	dueRaw   string
}

// Draft turns one row into a draft under the resolved column targets.
func (m *Mapping) Draft(row Row, columns map[string]Target) Draft {
	b := &draftBuilder{m: m, d: Draft{Line: row.Line}}
	for _, h := range rowOrder(row, columns) {
		b.add(h, columns[h], strings.TrimSpace(row.Values[h]))
	}
	b.finish()
	return b.d
}

func rowOrder(row Row, columns map[string]Target) []string {
	if len(row.Headers) > 0 {
		return row.Headers
	}
	out := make([]string, 0, len(columns))
	for h := range columns {
		out = append(out, h)
	}
	sort.Strings(out)
	return out
}

func (b *draftBuilder) add(header string, t Target, v string) {
	if v == "" {
		return
	}
	d := &b.d
	switch t {
	case TargetAppendBody:
		b.appendix = append(b.appendix, "**"+BaseHeader(header)+":** "+v)
	case TargetExternalID:
		setOnce(&d.ExternalID, v)
	case TargetAlias:
		d.Aliases = appendUnique(d.Aliases, v)
	case TargetRef:
		setOnce(&d.Ref, v)
	case TargetTitle:
		setOnce(&d.Title, v)
	case TargetBody:
		b.body = append(b.body, v)
	case TargetStatus:
		setOnce(&d.StatusRaw, v)
	case TargetPriority:
		setOnce(&d.PriorityRaw, v)
	case TargetAssignee:
		setOnce(&d.AssigneeRaw, v)
	case TargetTags:
		for _, tag := range b.splitTags(v) {
			if mapped, ok := b.m.lookup(TargetTags, tag); ok {
				tag = mapped
			}
			if tag != "" {
				d.Tags = appendUnique(d.Tags, tag)
			}
		}
	case TargetDue:
		setOnce(&b.dueRaw, v)
	case TargetParent:
		setOnce(&d.Parent, v)
	case TargetDependsOn:
		for _, dep := range strings.FieldsFunc(v, isListSeparator) {
			d.DependsOn = appendUnique(d.DependsOn, dep)
		}
	case TargetRecurrence:
		setOnce(&d.Recurrence, v)
	default:
		b.addCustom(t, v)
	}
}

func (b *draftBuilder) addCustom(t Target, v string) {
	key, ok := t.CustomKey()
	if !ok {
		return
	}
	if mapped, ok := b.m.lookup(t, v); ok {
		v = mapped
	}
	if b.d.Custom == nil {
		b.d.Custom = map[string]string{}
	}
	if prev := b.d.Custom[key]; prev != "" {
		v = prev + ", " + v
	}
	b.d.Custom[key] = v
}

func (b *draftBuilder) finish() {
	d := &b.d
	d.Body = strings.Join(append(b.body, b.appendix...), "\n\n")
	if d.Title == "" {
		d.Errors = append(d.Errors, "title is empty")
	}
	d.Status = b.m.Defaults[TargetStatus]
	if d.StatusRaw != "" {
		d.Status = d.StatusRaw
		if mapped, ok := b.m.lookup(TargetStatus, d.StatusRaw); ok {
			d.Status = mapped
		}
	}
	if d.AssigneeRaw != "" {
		d.Assignee = firstListItem(d.AssigneeRaw)
		if mapped, ok := b.m.lookup(TargetAssignee, d.AssigneeRaw); ok {
			d.Assignee = mapped
		}
	}
	b.priority()
	if b.dueRaw != "" {
		t, err := ParseDate(b.dueRaw, b.m.DateFormat)
		if err != nil {
			d.Errors = append(d.Errors, "due date "+errText(err))
		} else {
			d.Due = &t
		}
	}
}

func (b *draftBuilder) priority() {
	d := &b.d
	if d.PriorityRaw == "" {
		d.Priority = b.defaultPriority()
		return
	}
	value := d.PriorityRaw
	if mapped, ok := b.m.lookup(TargetPriority, value); ok {
		value = mapped
	}
	if p, ok := SuggestPriority(value); ok {
		d.Priority = p
		return
	}
	problem := fmt.Sprintf("priority %q is not recognised; map it under values.priority", d.PriorityRaw)
	switch b.m.Policy(TargetPriority) {
	case PolicySkipRow:
		d.Skip = problem
	case PolicyError, PolicyKeep:
		d.Errors = append(d.Errors, problem)
	case PolicyDefault:
		d.Priority = b.defaultPriority()
	}
}

func (b *draftBuilder) defaultPriority() core.Priority {
	p, _ := SuggestPriority(b.m.Defaults[TargetPriority])
	return p
}

func (m *Mapping) lookup(t Target, v string) (string, bool) {
	values := m.Values[t]
	if mapped, ok := values[v]; ok {
		return mapped, true
	}
	for k, mapped := range values {
		if strings.EqualFold(strings.TrimSpace(k), v) {
			return mapped, true
		}
	}
	return "", false
}

func (b *draftBuilder) splitTags(v string) []string {
	var parts []string
	if sep := b.m.TagsSeparator; sep != "" {
		parts = strings.Split(v, sep)
	} else {
		parts = strings.FieldsFunc(v, func(r rune) bool { return r == ',' || r == ';' || r == '|' })
	}
	out := parts[:0]
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func isListSeparator(r rune) bool {
	return r == ',' || r == ';' || r == ' ' || r == '\t' || r == '\n' || r == '\r'
}

func firstListItem(v string) string {
	if i := strings.IndexAny(v, ",;"); i >= 0 {
		return strings.TrimSpace(v[:i])
	}
	return v
}

func setOnce(dst *string, v string) {
	if *dst == "" {
		*dst = v
	}
}

func appendUnique(list []string, v string) []string {
	for _, have := range list {
		if have == v {
			return list
		}
	}
	return append(list, v)
}

func errText(err error) string {
	var domain *core.Error
	if errors.As(err, &domain) {
		return domain.Message
	}
	return err.Error()
}

var priorityWords = foldKeys(map[string]core.Priority{
	"highest": core.PriorityHighest, "high": core.PriorityHigh, "normal": core.PriorityNormal,
	"medium": core.PriorityNormal, "low": core.PriorityLow, "lowest": core.PriorityLowest,
	"blocker": core.PriorityHighest, "critical": core.PriorityHighest, "urgent": core.PriorityHighest,
	"major": core.PriorityHigh, "minor": core.PriorityLow, "trivial": core.PriorityLowest,
	"p0": core.PriorityHighest, "p1": core.PriorityHigh, "p2": core.PriorityNormal,
	"p3": core.PriorityLow, "p4": core.PriorityLowest,
	"none": 0, "nopriority": 0,
	"най-висок": core.PriorityHighest, "висок": core.PriorityHigh, "нормален": core.PriorityNormal,
	"среден": core.PriorityNormal, "нисък": core.PriorityLow, "най-нисък": core.PriorityLowest,
	"спешен": core.PriorityHighest,
})

func foldKeys[V any](in map[string]V) map[string]V {
	out := make(map[string]V, len(in))
	for k, v := range in {
		out[fold(k)] = v
	}
	return out
}

// SuggestPriority reads a priority name, number or Jira, Linear or P0-P4 word; 0 with true means explicitly none.
func SuggestPriority(value string) (core.Priority, bool) {
	f := fold(value)
	if f == "" {
		return 0, false
	}
	if p, ok := priorityWords[f]; ok {
		return p, true
	}
	if n, err := strconv.Atoi(f); err == nil && core.Priority(n).Valid() {
		return core.Priority(n), true
	}
	return 0, false
}
