// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"strconv"
	"testing"
)

// fakeListing is a listing of n rows whose cursor is the index of the next
// row, read the way every keyset listing is: limit rows from a cursor.
func fakeListing(n int, reads *int) walkFunc {
	return func(cursor string, limit int) (int, string, error) {
		*reads++
		from := 0
		if cursor != "" {
			from, _ = strconv.Atoi(cursor)
		}
		to := min(from+limit, n)
		if to >= n {
			return to - from, "", nil
		}
		return to - from, strconv.Itoa(to), nil
	}
}

func TestALocatedPageStartsWhereItsNumberSaysAndCountsTheListing(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, target string
		rows, size   int
		cursor       string
		number       int
		total        int
		capped       bool
	}{
		{"the first page", "/users", 60, 25, "", 1, 60, false},
		{"page 3", "/users?page=3", 60, 25, "50", 3, 60, false},
		{"past the end", "/users?page=9", 60, 25, "50", 3, 60, false},
		{"an exact last page", "/users?page=3", 75, 25, "50", 3, 75, false},
		{"past the cap", "/users?page=2", 5000, 25, "25", 2, walkCountCap, true},
		{"exactly the cap", "/users", walkCountCap, 25, "", 1, walkCountCap, false},
		{"a cursor wins", "/users?page=3&cursor=7", 60, 25, "7", 0, 60, false},
		{"an empty listing", "/users?page=2", 0, 25, "", 1, 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			reads := 0
			pos, err := locate(request(tc.target), tc.size, fakeListing(tc.rows, &reads))
			if err != nil {
				t.Fatal(err)
			}
			if pos.Cursor != tc.cursor || pos.Number != tc.number || pos.Total != tc.total || pos.Capped != tc.capped {
				t.Errorf("located %+v, want cursor %q number %d total %d capped %v",
					pos, tc.cursor, tc.number, tc.total, tc.capped)
			}
			if reads > walkCountCap/walkStretch+4 {
				t.Errorf("counting took %d reads", reads)
			}
		})
	}
}

func TestACappedPagerWritesItsCountsAsAFloorAndOffersNoLast(t *testing.T) {
	t.Parallel()
	p := newPager(request("/users?page=2"), RouteUsers, "next", 25, "users").
		counted(position{Number: 2, Total: walkCountCap, Capped: true}, 25)
	if p.PagesLabel() != "40+" || p.TotalLabel() != "1000+" || p.HasLast() {
		t.Errorf("capped pager says %s pages, %s rows, last %v; want 40+, 1000+ and no Last",
			p.PagesLabel(), p.TotalLabel(), p.HasLast())
	}
	if links := p.Links(); links[len(links)-1].Number != 3 {
		t.Errorf("a capped run ends at page %d, want the page after this one", links[len(links)-1].Number)
	}
}
