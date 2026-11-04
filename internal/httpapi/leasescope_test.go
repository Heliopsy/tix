// SPDX-License-Identifier: AGPL-3.0-or-later

package httpapi_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/wire"
)

func TestLeaseLengthOverTheAPI(t *testing.T) {
	f := newFixture(t)
	task := f.createTaskWithFields("long build", nil)
	leasePath := strings.Replace(wire.RouteTaskLease, "{ref}", task.Ref, 1)

	patched := f.call(http.MethodPatch, "/api/v1/projects/infra", map[string]any{"lease_default": "2h", "lease_max": "6h"})
	mustStatus(t, patched, http.StatusOK)
	var project core.Project
	decodeBody(t, patched, &project)
	if project.LeaseDefault.D() != 2*time.Hour || project.LeaseMax.D() != 6*time.Hour {
		t.Fatalf("project = %s / %s, want 2h / 6h", project.LeaseDefault, project.LeaseMax)
	}

	var grant core.LeaseGrant
	got := f.call(http.MethodGet, leasePath, nil)
	mustStatus(t, got, http.StatusOK)
	decodeBody(t, got, &grant)
	if grant.TTL.D() != 2*time.Hour || grant.Source != core.LeaseFromProject || grant.MaxSource != core.LeaseFromProject {
		t.Fatalf("lease = %+v, want 2h from the project under its 6h maximum", grant)
	}

	edited := f.call(http.MethodPatch, "/api/v1/tasks/"+task.Ref, map[string]any{"lease": "4h"})
	mustStatus(t, edited, http.StatusOK)
	_ = edited.Body.Close()
	claimed := f.call(http.MethodPost, strings.Replace(wire.RouteTaskClaim, "{ref}", task.Ref, 1), nil)
	mustStatus(t, claimed, http.StatusOK)
	var claim core.Claim
	decodeBody(t, claimed, &claim)
	if claim.Lease == nil || claim.Lease.TTL.D() != 4*time.Hour || claim.Lease.Source != core.LeaseFromTask {
		t.Fatalf("claim lease = %+v, want 4h from the task", claim.Lease)
	}

	tooLong := f.call(http.MethodPatch, "/api/v1/tasks/"+task.Ref, map[string]any{"lease": "7h"})
	mustStatus(t, tooLong, http.StatusBadRequest)
	_ = tooLong.Body.Close()
}

func TestCreateTaskTakesALeaseOverTheAPI(t *testing.T) {
	f := newFixture(t)
	created := f.call(http.MethodPost, "/api/v1/tasks", map[string]any{"project_ref": "infra", "title": "build", "lease": "2h"})
	mustStatus(t, created, http.StatusCreated)
	var task core.Task
	decodeBody(t, created, &task)
	if task.Lease.D() != 2*time.Hour {
		t.Fatalf("created task lease = %s, want 2h", task.Lease)
	}

	tooLong := f.call(http.MethodPost, "/api/v1/tasks", map[string]any{"project_ref": "infra", "title": "x", "lease": "48h"})
	mustStatus(t, tooLong, http.StatusBadRequest)
	_ = tooLong.Body.Close()
}
