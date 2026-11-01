// SPDX-License-Identifier: AGPL-3.0-or-later

package httpapi_test

import (
	"net/http"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/wire"
)

func TestTheTrashTurnsOffOverTheAPI(t *testing.T) {
	f := newFixture(t)
	task := f.createTask("kept")
	deleted := f.call(http.MethodDelete, "/api/v1/tasks/"+task.Ref, nil)
	mustStatus(t, deleted, http.StatusNoContent)
	_ = deleted.Body.Close()

	off := true
	patched := f.call(http.MethodPatch, "/api/v1/tenants/acme", core.UpdateTenantInput{TrashOff: &off})
	mustStatus(t, patched, http.StatusOK)
	var tenant core.Tenant
	decodeBody(t, patched, &tenant)
	if !tenant.TrashOff {
		t.Fatalf("tenant after turning the trash off = %+v", tenant)
	}

	status := f.call(http.MethodGet, wire.RouteTrash, nil)
	mustStatus(t, status, http.StatusOK)
	var trash core.TrashStatus
	decodeBody(t, status, &trash)
	if trash.Enabled || trash.Held != 1 {
		t.Errorf("GET %s = %+v, want off and holding one task", wire.RouteTrash, trash)
	}

	restore := f.call(http.MethodPost, "/api/v1/tasks/"+task.Ref+"/restore", nil)
	mustStatus(t, restore, http.StatusUnprocessableEntity)
	var envelope wire.ErrorEnvelope
	decodeBody(t, restore, &envelope)
	if envelope.Error.Code != core.KindPrecondition || envelope.Error.Details["reason"] != core.ErrTrashOff {
		t.Errorf("restore with the trash off answered %+v, want precondition_failed with reason %s",
			envelope.Error, core.ErrTrashOff)
	}
}
