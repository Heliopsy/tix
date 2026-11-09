// SPDX-License-Identifier: AGPL-3.0-or-later

package httpapi_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/heliopsy/tix/internal/client"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/httpapi"
	"github.com/heliopsy/tix/internal/wire"
)

func TestTheTaskListingAnswersItsTotalAndAPageByNumber(t *testing.T) {
	f := newFixture(t)
	for i := 1; i <= 7; i++ {
		f.createTask(fmt.Sprintf("task %d", i))
	}
	cases := []struct {
		query        string
		items, total int
		page         int
		next         bool
	}{
		{"?limit=3", 3, 7, 0, true},
		{"?limit=3&page=3", 1, 7, 3, false},
		{"?limit=3&page=40", 1, 7, 3, false},
	}
	for _, tc := range cases {
		t.Run(tc.query, func(t *testing.T) {
			resp := f.call(http.MethodGet, wire.RouteTasks+tc.query, nil)
			mustStatus(t, resp, http.StatusOK)
			var page wire.Page[core.Task]
			decodeBody(t, resp, &page)
			if page.Total == nil || *page.Total != tc.total || page.TotalCapped {
				t.Fatalf("total = %v capped %v, want %d exact", page.Total, page.TotalCapped, tc.total)
			}
			if len(page.Items) != tc.items || page.Page != tc.page || (page.NextCursor != "") != tc.next {
				t.Errorf("answered %d items, page %d, next %q; want %d items, page %d, next %v",
					len(page.Items), page.Page, page.NextCursor, tc.items, tc.page, tc.next)
			}
		})
	}
}

func TestAPageNumberThatIsNotOneIsRefused(t *testing.T) {
	f := newFixture(t)
	for _, q := range []string{"?page=0", "?page=two"} {
		resp := f.call(http.MethodGet, wire.RouteTasks+q, nil)
		mustStatus(t, resp, http.StatusBadRequest)
		_ = resp.Body.Close()
	}
}

func TestTheClientCarriesThePageNumberAndTheTotal(t *testing.T) {
	f := newFixtureWith(t, func(fx *apiFixture, cfg *httpapi.Config) {
		cfg.DefaultTenantID = fx.tenantA.ID
	})
	for i := 1; i <= 5; i++ {
		f.createTask(fmt.Sprintf("task %d", i))
	}
	remote, err := client.New(f.server.URL, f.tokenA)
	if err != nil {
		t.Fatalf("building client: %v", err)
	}
	t.Cleanup(func() { _ = remote.Close() })
	page, err := remote.ListTasks(context.Background(), core.TaskFilter{PageNumber: 2, Page: core.Page{Limit: 2}})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 5 || page.Number != 2 || len(page.Tasks) != 2 {
		t.Errorf("client page = total %d number %d with %d tasks, want 5, 2 and 2", page.Total, page.Number, len(page.Tasks))
	}
}
