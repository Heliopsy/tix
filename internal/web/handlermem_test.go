// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"net/http"
	"runtime"
	"testing"
)

// handlerBudget is what one more handler may keep alive once the first has
// paid for the shared templates and compressed assets. Measured at about
// 93 KiB. A handler that parses its own templates keeps about 9 MiB, and one
// that compresses its own assets about 880 KiB; both fail here.
const handlerBudget = 512 << 10

// TestAHandlerIsCheapToBuild guards the web suite's memory, whatever the cause.
// The suite builds one handler per parallel test, so a handler that holds a
// few megabytes of its own made the package peak at 18 GB and got CI's Linux
// runner killed. This builds a crowd of handlers and fails when each costs
// more than handlerBudget.
//
// Not parallel: it reads the process heap, which other tests would move.
func TestAHandlerIsCheapToBuild(t *testing.T) {
	const n = 40
	_ = Handler(nil)
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	kept := make([]http.Handler, n)
	for i := range kept {
		kept[i] = Handler(nil)
	}
	runtime.GC()
	runtime.ReadMemStats(&after)
	runtime.KeepAlive(kept)
	per := (int64(after.HeapAlloc) - int64(before.HeapAlloc)) / n
	t.Logf("one handler keeps %d KiB", per>>10)
	if per > handlerBudget {
		t.Fatalf("one handler keeps %d KiB alive, over the %d KiB budget: the web suite builds one per parallel test", per>>10, handlerBudget>>10)
	}
}
