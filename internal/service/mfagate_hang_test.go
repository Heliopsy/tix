// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
)

// hangGuard is how long a run that should take milliseconds may take before
// it counts as the deadlock it is guarding against.
const hangGuard = 20 * time.Second

// finishes runs fn and fails the test when it has not returned within
// hangGuard, so a regression fails in seconds instead of waiting out the
// package timeout.
func finishes(t *testing.T, what string, fn func()) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		fn()
	}()
	select {
	case <-done:
	case <-time.After(hangGuard):
		buf := make([]byte, 1<<20)
		t.Fatalf("%s did not finish in %s; goroutines:\n%s", what, hangGuard, buf[:runtime.Stack(buf, true)])
	}
}

// Asking the policy inside a read transaction must not open a second one.
// GetTask authorizes again inside its read, and when that question cost a
// read of its own, enough concurrent readers held every connection in the pool
// while each waited for another: the web suite hung for forty minutes on four
// people reading one task. Many more readers than the pool holds, policy off
// and on, must all come back.
func TestConcurrentReadsDoNotDeadlockOnTheGate(t *testing.T) {
	l, clk, scope, admin := newTOTPLocal(t)
	project := seedTaskProject(t, l, scope, "hang")
	task := seedTask(t, l, scope, project.ID, "todo")
	user := seedUser(t, l, admin, "ada@example.com")
	ada := userActor(t, l, user, core.RoleMember)
	ada.Scopes = []core.Scope{core.ScopeAll}
	enrol(t, l, clk, ada)

	readers := 8 * runtime.NumCPU()
	for _, policy := range []core.MFAPolicy{core.MFAPolicyOff, core.MFAPolicyRequired} {
		setMFAPolicy(t, l, admin, policy, 0)
		finishes(t, "concurrent reads with the policy "+string(policy), func() {
			var wg sync.WaitGroup
			errs := make(chan error, readers)
			for range readers {
				wg.Add(1)
				go func() {
					defer wg.Done()
					who := *ada
					_, err := l.GetTask(authContext(&who), core.TaskRef{ID: task.ID})
					errs <- err
				}()
			}
			wg.Wait()
			close(errs)
			for err := range errs {
				if err != nil {
					t.Errorf("GetTask: %v", err)
				}
			}
		})
	}
}

// A refusal raised inside a write transaction must not wait for the writer to
// record itself: SQLite has one, and the refused request is holding it.
func TestARefusalInsideAWriteDoesNotWaitForItself(t *testing.T) {
	l, _, _, admin := newTOTPLocal(t)
	ada := userActor(t, l, seedUser(t, l, admin, "ada@example.com"), core.RoleMember)
	setMFAPolicy(t, l, admin, core.MFAPolicyRequired, 0)

	finishes(t, "a refusal inside a write", func() {
		err := l.write(context.Background(), admin, func(*mutation) error {
			return l.requireFactor(authContext(ada), ada)
		})
		if !isGated(err) {
			t.Errorf("the refusal = %v, want the gate", err)
		}
	})
	l.gates.pending.Wait()
}
