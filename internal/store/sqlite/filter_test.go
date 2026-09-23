// SPDX-License-Identifier: AGPL-3.0-or-later

package sqlite

import (
	"context"
	"reflect"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
	"github.com/heliopsy/tix/internal/store/sqltest"
)

// TestFilterConformance runs the shared corpus. The same table runs against
// PostgreSQL in internal/store/postgres, so a weak match or a negated term
// that selected different tasks on the two engines fails here or there.
func TestFilterConformance(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	f := seed(t, s, clk, "conformance")
	if err := sqltest.Seed(ctx, s, f.scope, f.project.ID, f.actor.ID); err != nil {
		t.Fatalf("seeding the corpus: %v", err)
	}

	for _, c := range sqltest.Cases() {
		t.Run(c.Name, func(t *testing.T) {
			var got []core.Task
			if err := s.View(ctx, f.scope, func(tx store.Tx) error {
				var err error
				got, err = tx.ListTasks(ctx, c.Filter)
				return err
			}); err != nil {
				t.Fatalf("listing: %v", err)
			}
			want := sqltest.Sorted(c.Want)
			if have := sqltest.Titles(got); !equalTitles(have, want) {
				t.Errorf("sqlite selected %v, want %v", have, want)
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
