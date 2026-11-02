// SPDX-License-Identifier: AGPL-3.0-or-later

package core

import (
	"strconv"
	"strings"
)

// HousekeepingRule names one housekeeping rule.
type HousekeepingRule string

// Housekeeping rules. Done and cancelled move finished tasks to the trash;
// trash permanently deletes what has been in the trash long enough.
const (
	RuleDone      HousekeepingRule = "done"
	RuleCancelled HousekeepingRule = "cancelled"
	RuleTrash     HousekeepingRule = "trash"
)

// HousekeepingRules returns every rule in a stable order.
func HousekeepingRules() []HousekeepingRule {
	return []HousekeepingRule{RuleDone, RuleCancelled, RuleTrash}
}

// MaxHousekeepingDays bounds a rule, so a typo cannot overflow a cutoff.
const MaxHousekeepingDays = 36500

// HousekeepingPolicy is one level's rules in days. On a project a nil rule
// inherits the tenant's; on the tenant a nil rule is off. Zero is off.
type HousekeepingPolicy struct {
	ProjectID string `json:"project_id,omitempty" yaml:"project_id,omitempty"`
	Done      *int   `json:"done" yaml:"done"`
	Cancelled *int   `json:"cancelled" yaml:"cancelled"`
	Trash     *int   `json:"trash" yaml:"trash"`
}

// Rule returns the days a rule is set to, nil when it inherits.
func (p HousekeepingPolicy) Rule(r HousekeepingRule) *int {
	switch r {
	case RuleDone:
		return p.Done
	case RuleCancelled:
		return p.Cancelled
	case RuleTrash:
		return p.Trash
	}
	return nil
}

// SetRule sets one rule.
func (p *HousekeepingPolicy) SetRule(r HousekeepingRule, days *int) {
	switch r {
	case RuleDone:
		p.Done = days
	case RuleCancelled:
		p.Cancelled = days
	case RuleTrash:
		p.Trash = days
	}
}

// Days returns a rule's days, zero when it is off or inherits.
func (p HousekeepingPolicy) Days(r HousekeepingRule) int {
	if d := p.Rule(r); d != nil {
		return *d
	}
	return 0
}

// Empty reports whether every rule inherits.
func (p HousekeepingPolicy) Empty() bool {
	return p.Done == nil && p.Cancelled == nil && p.Trash == nil
}

// Resolve layers a project override over the tenant default.
func (p HousekeepingPolicy) Resolve(override *HousekeepingPolicy) HousekeepingPolicy {
	out := HousekeepingPolicy{}
	for _, r := range HousekeepingRules() {
		d := p.Days(r)
		if override != nil {
			out.ProjectID = override.ProjectID
			if v := override.Rule(r); v != nil {
				d = *v
			}
		}
		out.SetRule(r, &d)
	}
	return out
}

// HousekeepingInput changes one level of the policy. Each rule takes "off", a
// number of days ("30" or "30d"), or on a project "inherit"; empty leaves it.
type HousekeepingInput struct {
	Project   string `json:"project,omitempty" yaml:"project,omitempty"`
	Done      string `json:"done,omitempty" yaml:"done,omitempty"`
	Cancelled string `json:"cancelled,omitempty" yaml:"cancelled,omitempty"`
	Trash     string `json:"trash,omitempty" yaml:"trash,omitempty"`
}

// Value returns the raw value given for a rule.
func (in HousekeepingInput) Value(r HousekeepingRule) string {
	switch r {
	case RuleDone:
		return in.Done
	case RuleCancelled:
		return in.Cancelled
	case RuleTrash:
		return in.Trash
	}
	return ""
}

// ParseHousekeepingValue reads one rule value. It returns nil for inherit,
// which only a project may ask for.
func ParseHousekeepingValue(rule HousekeepingRule, raw string, project bool) (*int, error) {
	v := strings.ToLower(strings.TrimSpace(raw))
	switch v {
	case "inherit":
		if !project {
			return nil, Invalid("%s: only a project can inherit; use off or a number of days", rule).
				WithDetail(DetailField, string(rule))
		}
		return nil, nil
	case "off", "0":
		zero := 0
		return &zero, nil
	}
	n, err := strconv.Atoi(strings.TrimSuffix(v, "d"))
	if err != nil || n < 0 || n > MaxHousekeepingDays {
		return nil, Invalid("%s %q must be off, inherit or a number of days from 1 to %d", rule, raw, MaxHousekeepingDays).
			WithDetail(DetailField, string(rule))
	}
	return &n, nil
}

// HousekeepingCounts is how many tasks each rule acts on.
type HousekeepingCounts struct {
	Done      int `json:"done" yaml:"done"`
	Cancelled int `json:"cancelled" yaml:"cancelled"`
	Trash     int `json:"trash" yaml:"trash"`
}

// Total sums the counts.
func (c HousekeepingCounts) Total() int { return c.Done + c.Cancelled + c.Trash }

// Add counts n tasks against a rule.
func (c *HousekeepingCounts) Add(r HousekeepingRule, n int) {
	switch r {
	case RuleDone:
		c.Done += n
	case RuleCancelled:
		c.Cancelled += n
	case RuleTrash:
		c.Trash += n
	}
}

// Of returns one rule's count.
func (c HousekeepingCounts) Of(r HousekeepingRule) int {
	switch r {
	case RuleDone:
		return c.Done
	case RuleCancelled:
		return c.Cancelled
	case RuleTrash:
		return c.Trash
	}
	return 0
}

// HousekeepingView is the policy as one level sees it, with what it would
// act on now. Project, Override and Effective describe the project asked
// about; without one, Effective is the tenant default.
type HousekeepingView struct {
	Tenant    HousekeepingPolicy  `json:"tenant" yaml:"tenant"`
	Project   string              `json:"project,omitempty" yaml:"project,omitempty"`
	Override  *HousekeepingPolicy `json:"override,omitempty" yaml:"override,omitempty"`
	Effective HousekeepingPolicy  `json:"effective" yaml:"effective"`
	// Paused is true while the tenant's trash is off: no rule runs then.
	Paused  bool               `json:"paused" yaml:"paused"`
	Pending HousekeepingCounts `json:"pending" yaml:"pending"`
}

// HousekeepingRunInput runs housekeeping once, or previews it.
type HousekeepingRunInput struct {
	Project string `json:"project,omitempty" yaml:"project,omitempty"`
	DryRun  bool   `json:"dry_run,omitempty" yaml:"dry_run,omitempty"`
	Limit   int    `json:"limit,omitempty" yaml:"limit,omitempty"`
}

// HousekeepingResult says what one run did, or would do.
type HousekeepingResult struct {
	DryRun             bool `json:"dry_run" yaml:"dry_run"`
	Paused             bool `json:"paused" yaml:"paused"`
	HousekeepingCounts `yaml:",inline"`
}
