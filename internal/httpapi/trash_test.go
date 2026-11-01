// SPDX-License-Identifier: AGPL-3.0-or-later

package httpapi_test

import (
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/wire"
)

func TestTheTaskListingTakesTrashed(t *testing.T) {
	f := newFixture(t)
	f.createTask("keep")
	gone := f.createTask("gone")
	resp := f.call(http.MethodDelete, strings.Replace(wire.RouteTask, "{ref}", gone.Ref, 1), nil)
	_ = resp.Body.Close()
	if resp.StatusCode >= 300 {
		t.Fatalf("deleting %s answered %d", gone.Ref, resp.StatusCode)
	}

	tests := []struct {
		query string
		want  []string
	}{
		{"?trashed=true", []string{"gone"}},
		{"?trashed=false", []string{"keep"}},
		{"?trashed=false&include_deleted=true", []string{"keep"}},
		{"?include_deleted=true&sort=seq", []string{"keep", "gone"}},
	}
	for _, tt := range tests {
		if got := f.listTitles(tt.query); !slices.Equal(got, tt.want) {
			t.Errorf("%s listed %v, want %v", tt.query, got, tt.want)
		}
	}
}
