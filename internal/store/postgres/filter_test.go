// SPDX-License-Identifier: AGPL-3.0-or-later

package postgres

import (
	"context"
	"reflect"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
	"github.com/heliopsy/tix/internal/store/sqltest"
)

// TestFilterConformance runs the shared corpus, with and without bodies. The same table runs against
// SQLite in internal/store/sqlite, so a weak match or a negated term that
// selected different tasks on the two engines fails here or there. It is the
// whole reason the explicit text terms use LOWER/LIKE rather than each
// engine's native search the way core.TaskFilter.Query does.
func TestFilterConformance(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	f := seed(t, s, clk, "conformance")
	if err := sqltest.Seed(ctx, s, f.scope, f.project.ID, f.actor.ID); err != nil {
		t.Fatalf("seeding the corpus: %v", err)
	}

	for _, c := range sqltest.Cases() {
		t.Run(c.Name, func(t *testing.T) {
			for _, omit := range []bool{false, true} {
				filter := c.Filter
				filter.OmitBody = omit
				var got []core.Task
				if err := s.View(ctx, f.scope, func(tx store.Tx) error {
					var err error
					got, err = tx.ListTasks(ctx, filter)
					return err
				}); err != nil {
					t.Fatalf("listing: %v", err)
				}
				want := sqltest.Sorted(c.Want)
				if have := sqltest.Titles(got); !equalTitles(have, want) {
					t.Errorf("postgres selected %v with OmitBody %v, want %v", have, omit, want)
				}
				for _, task := range got {
					if omit && task.Body != "" {
						t.Errorf("postgres read %q's body for a listing that omits it", task.Title)
					}
				}
			}
		})
	}
}

// equalTitles compares two sorted title lists, treating nil and empty alike.
func equalTitles(got, want []string) bool {
	if len(got) == 0 && len(want) == 0 {
		return true
	}
	return reflect.DeepEqual(got, want)
}
