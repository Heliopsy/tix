// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

func TestTaskLsPrintsOnePageByNumber(t *testing.T) {
	c := newCLI(t)
	for i := 1; i <= 7; i++ {
		c.mustRun("task", "add", fmt.Sprintf("task %d", i))
	}
	got := c.mustRun("task", "ls", "--sort", "seq", "--limit", "3", "--page", "2", "-o", "json")
	if !strings.Contains(got.err, "page 2 of 3, 7 tasks") {
		t.Errorf("stderr = %q, want the page position", got.err)
	}
	titles := taskTitles(t, c, "--sort", "seq", "--limit", "3", "--page", "2")
	if want := []string{"task 4", "task 5", "task 6"}; !slices.Equal(titles, want) {
		t.Errorf("page 2 = %v, want %v", titles, want)
	}
	for _, args := range [][]string{{"--page", "2", "--all"}, {"--page", "2", "--cursor", "x"}, {"--page", "-1"}} {
		if res := c.run(append([]string{"task", "ls"}, args...)...); res.code == 0 {
			t.Errorf("task ls %v succeeded, want a usage error", args)
		}
	}
}
