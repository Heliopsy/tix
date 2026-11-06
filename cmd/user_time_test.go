// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"regexp"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

// A table follows the tenant's date format when this machine names none, and
// a format this machine names wins over it.
func TestTablesFollowTheTenantUnlessTheMachineSaysOtherwise(t *testing.T) {
	c := newCLI(t)
	c.mustRun("task", "add", "dated")
	c.mustRun("tenant", "edit", "default", "--time-format", "dotted", "--timezone", "Asia/Tokyo")

	dotted := regexp.MustCompile(`\d{2}\.\d{2}\.\d{4} \d{2}:\d{2}`)
	if got := c.mustRun("task", "ls"); !dotted.MatchString(got.out) {
		t.Errorf("task ls does not follow the tenant's dotted format:\n%s", got.out)
	}
	iso := regexp.MustCompile(`\d{4}-\d{2}-\d{2} \d{2}:\d{2}`)
	if got := runWithEnv(c, "TIX_OUTPUT_TIME_FORMAT=iso", "task", "ls"); !iso.MatchString(got.out) || dotted.MatchString(got.out) {
		t.Errorf("a format set on the machine does not win over the tenant's:\n%s", got.out)
	}
	if got := c.mustRun("task", "ls", "-o", "json"); !strings.Contains(got.out, `T`) || dotted.MatchString(got.out) {
		t.Errorf("json output left RFC 3339:\n%s", got.out)
	}

	shown := c.mustRun("user", "time", "-o", "json")
	for _, want := range []string{`"format_source": "tenant"`, `"time_format": "dotted"`, `"personal": false`} {
		if !strings.Contains(shown.out, want) {
			t.Errorf("tix user time -o json lacks %s:\n%s", want, shown.out)
		}
	}
	if got := c.run("user", "time", "--format", "us"); got.code != core.ExitUsage {
		t.Errorf("a caller who is not a person set a preference: exit %d\n%s", got.code, got.err)
	}
	if got := c.run("tenant", "edit", "default", "--timezone", "Mars/Olympus"); got.code != core.ExitUsage {
		t.Errorf("an unknown zone was accepted: exit %d", got.code)
	}
	c.mustRun("tenant", "edit", "default", "--time-format", "")
	if got := c.mustRun("task", "ls"); dotted.MatchString(got.out) {
		t.Errorf("an emptied tenant format still renders dotted:\n%s", got.out)
	}
}
