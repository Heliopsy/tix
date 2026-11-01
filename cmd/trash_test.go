// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"slices"
	"testing"
)

func TestTaskLsListsTheTrash(t *testing.T) {
	c := newCLI(t)
	c.mustRun("task", "add", "keep me")
	c.mustRun("task", "add", "throw me out")
	c.mustRun("task", "rm", "default-2")

	tests := []struct {
		name string
		args []string
		want []string
	}{
		{"--trashed lists the trash alone", []string{"--trashed"}, []string{"throw me out"}},
		{"is:trashed says the same", []string{"--filter", "is:trashed"}, []string{"throw me out"}},
		{"-is:trashed lists live tasks", []string{"--filter", "-is:trashed"}, []string{"keep me"}},
		{"is:deleted -is:trashed is live only", []string{"--filter", "is:deleted -is:trashed"}, []string{"keep me"}},
		{"--include-deleted --trashed is still the trash", []string{"--include-deleted", "--trashed"}, []string{"throw me out"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := append([]string{"task", "ls", "--sort", "seq", "-o", "ndjson"}, tt.args...)
			if got := refsOf(t, c.mustRun(args...).out); !slices.Equal(got, tt.want) {
				t.Fatalf("listed %v, want %v", got, tt.want)
			}
		})
	}
}
