// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"net/http"
	"strings"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/i18n"
)

// housekeepingRanKey is the toast after a project's cleanup: how many tasks
// went to the trash and how many were deleted for good.
const housekeepingRanKey = "cleanup ran: %d moved to the trash, %d deleted for good"

// housekeepingRoutes set the tenant's cleanup rules, a project's overrides,
// and run a pass now.
func (h *handler) housekeepingRoutes() []route {
	return []route{
		post(RouteHousekeeping, h.putTenantHousekeeping, "PutHousekeeping"),
		post(RouteHousekeepingRun, h.runHousekeeping, "RunHousekeeping"),
		post(RouteProjectHousekeeping, h.putProjectHousekeeping, "PutHousekeeping"),
		post(RouteProjectHousekeepingRun, h.runProjectHousekeeping, "RunHousekeeping"),
	}
}

// housekeepingForm is one level of the policy as the settings form draws it.
type housekeepingForm struct {
	Project string
	Paused  bool
	Rows    []housekeepingRow
	// Moving and Purging are what a run would move to the trash and delete
	// for good now, under the rules in force.
	Moving, Purging int
}

// Due reports whether a run now would act on anything. Nothing is pending
// while cleanup is paused.
func (f *housekeepingForm) Due() bool { return f.Moving+f.Purging > 0 }

// housekeepingRow is one rule of the form.
type housekeepingRow struct {
	Rule    string
	Project bool
	// Mode is inherit, off or days; Days fills the number box.
	Mode string
	Days int
	// Inherited is the tenant's days, for the inherit option's label.
	Inherited int
	// Effective is the days in force; Pending how many tasks it would act on
	// now.
	Effective int
	Pending   int
}

// newHousekeepingForm turns a view into the rows of its form.
func newHousekeepingForm(v *core.HousekeepingView) *housekeepingForm {
	if v == nil {
		return nil
	}
	f := &housekeepingForm{Project: v.Project, Paused: v.Paused}
	for _, r := range core.HousekeepingRules() {
		row := housekeepingRow{
			Rule: string(r), Project: v.Override != nil, Inherited: v.Tenant.Days(r),
			Effective: v.Effective.Days(r), Pending: v.Pending.Of(r),
		}
		level := v.Tenant.Rule(r)
		if v.Override != nil {
			level = v.Override.Rule(r)
		}
		switch {
		case level == nil && row.Project:
			row.Mode = "inherit"
		case level == nil || *level == 0:
			row.Mode = "off"
		default:
			row.Mode, row.Days = "days", *level
		}
		f.Rows = append(f.Rows, row)
		if row.Effective > 0 && r == core.RuleTrash {
			f.Purging += row.Pending
		} else if row.Effective > 0 {
			f.Moving += row.Pending
		}
	}
	return f
}

// housekeepingInput reads the form's rules as the service takes them.
func housekeepingInput(r *http.Request, project string) (core.HousekeepingInput, error) {
	in := core.HousekeepingInput{Project: project}
	for _, rule := range core.HousekeepingRules() {
		name := string(rule)
		value := field(r, name+"_mode")
		if value == "days" {
			value = strings.TrimSpace(field(r, name+"_days"))
			if value == "" || value == "0" {
				return in, core.Invalid("%s: give a number of days, or choose Off", name).WithDetail(core.DetailField, name)
			}
		}
		switch rule {
		case core.RuleDone:
			in.Done = value
		case core.RuleCancelled:
			in.Cancelled = value
		case core.RuleTrash:
			in.Trash = value
		}
	}
	return in, nil
}

// putTenantHousekeeping saves the tenant's cleanup rules.
func (h *handler) putTenantHousekeeping(w http.ResponseWriter, r *http.Request) error {
	in, err := housekeepingInput(r, "")
	if err != nil {
		return err
	}
	if _, err := h.svc.PutHousekeeping(r.Context(), in); err != nil {
		return err
	}
	redirect(w, r, RouteTenant, "cleanup saved")
	return nil
}

// putProjectHousekeeping saves one project's cleanup overrides.
func (h *handler) putProjectHousekeeping(w http.ResponseWriter, r *http.Request) error {
	key := r.PathValue("key")
	in, err := housekeepingInput(r, key)
	if err != nil {
		return err
	}
	view, err := h.svc.PutHousekeeping(r.Context(), in)
	if err != nil {
		return err
	}
	redirect(w, r, RouteProjects+"/"+view.Project, "cleanup saved")
	return nil
}

// runHousekeeping applies the cleanup rules once, now.
func (h *handler) runHousekeeping(w http.ResponseWriter, r *http.Request) error {
	res, err := h.svc.RunHousekeeping(r.Context(), core.HousekeepingRunInput{})
	if err != nil {
		return err
	}
	if res.Paused {
		redirect(w, r, RouteTenant, "cleanup is paused while the trash is off")
		return nil
	}
	redirect(w, r, RouteTenant, "cleanup ran")
	return nil
}

// runProjectHousekeeping applies the cleanup rules to one project once, now,
// after the reader confirmed it on the board.
func (h *handler) runProjectHousekeeping(w http.ResponseWriter, r *http.Request) error {
	key := r.PathValue("key")
	if field(r, "confirm") != "1" {
		return core.Invalid("confirm the cleanup of %s first", key)
	}
	res, err := h.svc.RunHousekeeping(r.Context(), core.HousekeepingRunInput{Project: key})
	if err != nil {
		return err
	}
	back := RouteProjects + "/" + key + "#housekeeping"
	if res.Paused {
		redirect(w, r, back, "cleanup is paused while the trash is off")
		return nil
	}
	redirect(w, r, back, i18n.T(langOf(r), housekeepingRanKey, res.Done+res.Cancelled, res.Trash))
	return nil
}

// projectHousekeeping reads a project's cleanup form, or nothing for a reader
// who may not see it.
func (h *handler) projectHousekeeping(r *http.Request, key string) (*housekeepingForm, error) {
	v, err := h.svc.GetHousekeeping(r.Context(), key)
	if core.IsKind(err, core.KindForbidden) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return newHousekeepingForm(v), nil
}
