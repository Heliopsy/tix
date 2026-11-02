// SPDX-License-Identifier: AGPL-3.0-or-later

package httpapi_test

import (
	"net/http"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/wire"
)

func TestHousekeepingRoutes(t *testing.T) {
	f := newFixture(t)

	var view core.HousekeepingView
	got := f.call(http.MethodGet, wire.RouteHousekeeping, nil)
	mustStatus(t, got, http.StatusOK)
	decodeBody(t, got, &view)
	if !view.Tenant.Empty() || view.Paused {
		t.Fatalf("a fresh tenant = %+v, want nothing set", view)
	}

	put := f.call(http.MethodPut, wire.RouteHousekeeping, core.HousekeepingInput{Done: "30", Trash: "off"})
	mustStatus(t, put, http.StatusOK)
	decodeBody(t, put, &view)
	if view.Tenant.Days(core.RuleDone) != 30 || view.Tenant.Trash == nil || view.Effective.Days(core.RuleTrash) != 0 {
		t.Fatalf("after put = %+v, want done 30 and trash off", view)
	}

	inherit := f.call(http.MethodPut, wire.RouteHousekeeping, core.HousekeepingInput{Done: "inherit"})
	mustStatus(t, inherit, http.StatusBadRequest)
	_ = inherit.Body.Close()

	var res core.HousekeepingResult
	run := f.call(http.MethodPost, wire.RouteHousekeepingRun, core.HousekeepingRunInput{DryRun: true})
	mustStatus(t, run, http.StatusOK)
	decodeBody(t, run, &res)
	if !res.DryRun || res.Total() != 0 {
		t.Fatalf("preview = %+v, want a dry run with nothing due", res)
	}

	off := f.call(http.MethodPatch, "/api/v1/tenants/acme", map[string]any{"trash_off": true})
	mustStatus(t, off, http.StatusOK)
	_ = off.Body.Close()
	refused := f.call(http.MethodPut, wire.RouteHousekeeping, core.HousekeepingInput{Cancelled: "7"})
	mustStatus(t, refused, http.StatusUnprocessableEntity)
	var body struct {
		Error struct {
			Details map[string]any `json:"details"`
		} `json:"error"`
	}
	decodeBody(t, refused, &body)
	if body.Error.Details["reason"] != core.ErrTrashOff {
		t.Errorf("refusal details = %v, want reason trash_off", body.Error.Details)
	}
}
