// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/web"
)

// badgeBudget is what one request for the sidebar badges may allocate with a
// hundred long overdue tasks behind them. Measured at about 1 MiB. Reading the
// tasks' descriptions to count them allocated about 5 MiB more, and counting
// their checklists too, as every listing does, about 50 MiB more.
const badgeBudget = 4 << 20

// longDescription is a description of about size bytes, a checklist among
// links and emphasis, the shape an imported task arrives in.
func longDescription(i, size int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Imported task %d\n\n", i)
	for b.Len() < size {
		fmt.Fprintf(&b, "- [ ] step %d, see [the doc](https://example.test/doc/%d), **bold** and `code`\n", b.Len(), i)
		b.WriteString("  A paragraph under the step that goes on for a while in plain words.\n")
	}
	return b.String()
}

// TestTheSidebarBadgesDoNotReadDescriptions guards the memory of the request
// every screen and every live refresh makes. My work's overdue badge counts
// up to a hundred tasks, and it read each one's description and parsed its
// checklist to do it: on a tenant of imported tasks that was 50 MiB a page,
// and it was the last request before the server was killed for memory.
//
// Not parallel: it reads the process's allocation total, which other tests
// would move.
func TestTheSidebarBadgesDoNotReadDescriptions(t *testing.T) {
	f := newFixture(t)
	ctx := f.ctx()
	due := f.clock.Now().Add(-24 * time.Hour)
	for i := range 101 {
		if _, err := f.svc.CreateTask(ctx, core.CreateTaskInput{ProjectRef: "infra", Title: fmt.Sprintf("late %d", i),
			Body: longDescription(i, 40<<10), AssigneeActorID: f.actorA.ID, DueAt: &due}); err != nil {
			t.Fatal(err)
		}
	}
	b := f.as("alice")
	fetch := func() string {
		resp := b.get(web.RouteInboxBadge)
		defer func() { _ = resp.Body.Close() }()
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("GET %s = %d", web.RouteInboxBadge, resp.StatusCode)
		}
		return string(body)
	}
	if got := fetch(); !strings.Contains(got, `data-badge-count="100"`) {
		t.Fatalf("the overdue badge does not count the hundred late tasks it may spell out:\n%s", got)
	}
	const runs = 3
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	for range runs {
		fetch()
	}
	runtime.ReadMemStats(&after)
	per := (after.TotalAlloc - before.TotalAlloc) / runs
	t.Logf("one badge request allocates %d KiB", per>>10)
	if per > badgeBudget {
		t.Fatalf("one badge request allocates %d KiB, over the %d KiB budget: the overdue count is reading descriptions",
			per>>10, badgeBudget>>10)
	}
}
