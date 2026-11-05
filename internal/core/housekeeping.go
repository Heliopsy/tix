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
	// ProjectID is the project this override belongs to, empty for the tenant default.
	ProjectID string `json:"project_id,omitempty" yaml:"project_id,omitempty"`
	// Done is the days a done task stays before it moves to the trash; null
	// inherits on a project and is off on the tenant, 0 is off.
	Done *int `json:"done" yaml:"done"`
	// Cancelled is the days a cancelled task stays before it moves to the
	// trash; null inherits on a project and is off on the tenant, 0 is off.
	Cancelled *int `json:"cancelled" yaml:"cancelled"`
	// Trash is the days a task stays in the trash before it is deleted for
	// good; null inherits on a project and is off on the tenant, 0 is off.
	Trash *int `json:"trash" yaml:"trash"`
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
	// Project is the key or ID of the project whose override changes; empty
	// changes the tenant default.
	Project string `json:"project,omitempty" yaml:"project,omitempty"`
	// Done sets the done rule: "off", days ("30" or "30d"), or "inherit" on a project.
	Done string `json:"done,omitempty" yaml:"done,omitempty"`
	// Cancelled sets the cancelled rule: "off", days, or "inherit" on a project.
	Cancelled string `json:"cancelled,omitempty" yaml:"cancelled,omitempty"`
	// Trash sets the trash rule: "off", days, or "inherit" on a project.
	Trash string `json:"trash,omitempty" yaml:"trash,omitempty"`
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
	// Done is the number of done tasks moved, or due to move, to the trash.
	Done int `json:"done" yaml:"done"`
	// Cancelled is the number of cancelled tasks moved, or due to move, to the trash.
	Cancelled int `json:"cancelled" yaml:"cancelled"`
	// Trash is the number of trashed tasks deleted, or due to be deleted, for good.
	Trash int `json:"trash" yaml:"trash"`
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
	// Tenant is the tenant default policy.
	Tenant HousekeepingPolicy `json:"tenant" yaml:"tenant"`
	// Project is the key of the project asked about, empty for the tenant.
	Project string `json:"project,omitempty" yaml:"project,omitempty"`
	// Override is the project's own rules, a null rule inheriting the tenant's.
	Override *HousekeepingPolicy `json:"override,omitempty" yaml:"override,omitempty"`
	// Effective is the policy that applies, with every rule resolved to days.
	Effective HousekeepingPolicy `json:"effective" yaml:"effective"`
	// Paused is true while the tenant's trash is off: no rule runs then.
	Paused bool `json:"paused" yaml:"paused"`
	// Pending counts the tasks each rule would act on now.
	Pending HousekeepingCounts `json:"pending" yaml:"pending"`
}

// HousekeepingRunInput runs housekeeping once, or previews it.
type HousekeepingRunInput struct {
	// Project is the key or ID of one project to run for; empty runs every project.
	Project string `json:"project,omitempty" yaml:"project,omitempty"`
	// DryRun counts what a run would act on and changes nothing.
	DryRun bool `json:"dry_run,omitempty" yaml:"dry_run,omitempty"`
	// Limit caps the tasks each rule acts on in this run; 0 means 200, negative is refused.
	Limit int `json:"limit,omitempty" yaml:"limit,omitempty"`
}

// HousekeepingResult says what one run did, or would do.
type HousekeepingResult struct {
	// DryRun is true when the run was a preview and changed nothing.
	DryRun bool `json:"dry_run" yaml:"dry_run"`
	// Paused is true while the tenant's trash is off, so no rule acted.
	Paused bool `json:"paused" yaml:"paused"`
	// HousekeepingCounts holds how many tasks each rule acted on, or would.
	HousekeepingCounts `yaml:",inline"`
}
