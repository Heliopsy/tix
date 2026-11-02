// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
)

// TestTheDetailSaysHowLongAClaimLasts reads the lease through the detail's
// own load and draws it, so a lease the load dropped and a lease the view
// forgot both fail.
func TestTheDetailSaysHowLongAClaimLasts(t *testing.T) {
	svc := newFakeService()
	svc.tasks = []core.Task{{ID: "t1", Ref: "infra-1", Title: "long build", Status: "todo"}}
	svc.lease = &core.LeaseGrant{TTL: core.Duration(4 * time.Hour), Source: core.LeaseFromProject,
		Max: core.Duration(24 * time.Hour), MaxSource: core.LeaseFromBuiltin}
	m := New(Config{Access: fullAccess(), Service: svc})
	msg, ok := m.detailFor(core.TaskRef{ID: "t1"})().(detailMsg)
	if !ok {
		t.Fatalf("detailFor did not return a detailMsg")
	}
	m.detail = &msg
	m.width = 120
	var line string
	for _, l := range m.detailLines(Layout{BodyHeight: 40}) {
		if strings.Contains(l, "lease:") {
			line = l
		}
	}
	if !strings.Contains(line, "lease: claims last 4h, from the project") {
		t.Fatalf("the detail's lease line = %q, want the project's 4h", line)
	}
}
