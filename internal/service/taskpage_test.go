// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"fmt"
	"slices"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// seedNumbered creates n tasks titled t01..tNN in f's project, every third one
// of high priority, plus three in a second project, and returns that project.
func seedNumbered(t *testing.T, f *claimFixture, n int) *core.Project {
	t.Helper()
	ctx := context.Background()
	other := core.Project{Key: "web", Name: "Web", WorkflowID: f.workflow.ID}
	if err := f.local.store.Update(ctx, f.scope, func(tx store.Tx) error {
		if err := tx.CreateProject(ctx, &other); err != nil {
			return err
		}
		for i := 1; i <= n; i++ {
			task := core.Task{ProjectID: f.project.ID, Title: fmt.Sprintf("t%02d", i), Status: "todo",
				Priority: core.PriorityNormal, CreatorActorID: f.actor.ID}
			if i%3 == 0 {
				task.Priority = core.PriorityHigh
			}
			if err := tx.CreateTask(ctx, &task); err != nil {
				return err
			}
		}
		for i := 1; i <= 3; i++ {
			task := core.Task{ProjectID: other.ID, Title: fmt.Sprintf("w%d", i), Status: "todo",
				Priority: core.PriorityNormal, CreatorActorID: f.actor.ID}
			if err := tx.CreateTask(ctx, &task); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatalf("seeding: %v", err)
	}
	return &other
}

func pageTitles(p core.TaskPage) []string {
	out := []string{}
	for _, task := range p.Tasks {
		out = append(out, task.Title)
	}
	return out
}

func TestAListingCountsWhatItsFilterSelects(t *testing.T) {
	f := newClaimFixture(t, 0)
	seedNumbered(t, f, 30)
	cases := []struct {
		name   string
		filter core.TaskFilter
		want   int
	}{
		{"everything", core.TaskFilter{}, 33},
		{"one project", core.TaskFilter{ProjectKeys: []string{"infra"}}, 30},
		{"a priority", core.TaskFilter{Priorities: []core.Priority{core.PriorityHigh}}, 10},
		{"a priority excluded", core.TaskFilter{ProjectKeys: []string{"infra"}, Exclude: core.TaskExclude{Priorities: []core.Priority{core.PriorityHigh}}}, 20},
		{"nothing matches", core.TaskFilter{Statuses: []string{"done"}}, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.filter.Page.Limit = 5
			page, err := f.local.ListTasks(f.ctx, tc.filter)
			if err != nil {
				t.Fatal(err)
			}
			if page.Total != tc.want || page.TotalCapped {
				t.Errorf("total = %d capped %v, want %d exact", page.Total, page.TotalCapped, tc.want)
			}
			if len(page.Tasks) != min(5, tc.want) {
				t.Errorf("page holds %d tasks, want %d", len(page.Tasks), min(5, tc.want))
			}
		})
	}
}

func TestAProjectScopedReaderCountsOnlyItsProject(t *testing.T) {
	f := newClaimFixture(t, 0)
	web := seedNumbered(t, f, 4)
	scoped := *f.actor
	scoped.ProjectID = web.ID
	page, err := f.local.ListTasks(core.WithActor(context.Background(), &scoped), core.TaskFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 3 {
		t.Errorf("a reader scoped to web counts %d tasks, want 3", page.Total)
	}
}

func TestAPageNumberAnswersThatSliceOfTheWalk(t *testing.T) {
	f := newClaimFixture(t, 0)
	seedNumbered(t, f, 12)
	base := core.TaskFilter{ProjectKeys: []string{"infra"}, Page: core.Page{Limit: 5, Sort: core.SortSeq}}

	var walked []string
	walk := base
	for {
		page, err := f.local.ListTasks(f.ctx, walk)
		if err != nil {
			t.Fatal(err)
		}
		walked = append(walked, pageTitles(page)...)
		if page.NextCursor == "" {
			break
		}
		walk.Page.Cursor = page.NextCursor
	}

	cases := []struct {
		asked, number int
		want          []string
	}{
		{1, 1, walked[0:5]},
		{2, 2, walked[5:10]},
		{3, 3, walked[10:12]},
		{9, 3, walked[10:12]},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprint("page ", tc.asked), func(t *testing.T) {
			f2 := base
			f2.PageNumber = tc.asked
			page, err := f.local.ListTasks(f.ctx, f2)
			if err != nil {
				t.Fatal(err)
			}
			if got := pageTitles(page); !slices.Equal(got, tc.want) {
				t.Errorf("page %d = %v, want %v", tc.asked, got, tc.want)
			}
			if page.Number != tc.number || page.Total != 12 {
				t.Errorf("page %d answered number %d total %d, want %d and 12", tc.asked, page.Number, page.Total, tc.number)
			}
		})
	}
}

func TestAPageNumberHonoursStarsAndTheNextCursorContinuesIt(t *testing.T) {
	f := newClaimFixture(t, 0)
	seedNumbered(t, f, 7)
	all, err := f.local.ListTasks(f.ctx, core.TaskFilter{ProjectKeys: []string{"infra"}, Page: core.Page{Sort: core.SortSeq}})
	if err != nil {
		t.Fatal(err)
	}
	if err := f.local.StarTask(f.ctx, core.TaskRef{ID: all.Tasks[6].ID}); err != nil {
		t.Fatal(err)
	}
	base := core.TaskFilter{ProjectKeys: []string{"infra"}, Page: core.Page{Limit: 3, Sort: core.SortSeq}}
	base.PageNumber = 2
	two, err := f.local.ListTasks(f.ctx, base)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := pageTitles(two), []string{"t03", "t04", "t05"}; !slices.Equal(got, want) {
		t.Errorf("page 2 behind a starred t07 = %v, want %v", got, want)
	}
	base.PageNumber, base.Page.Cursor = 0, two.NextCursor
	three, err := f.local.ListTasks(f.ctx, base)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := pageTitles(three), []string{"t06"}; !slices.Equal(got, want) {
		t.Errorf("the page after page 2 = %v, want %v", got, want)
	}
}

func TestAPageNumberBesideACursorIsRefused(t *testing.T) {
	f := newClaimFixture(t, 0)
	seedNumbered(t, f, 4)
	first, err := f.local.ListTasks(f.ctx, core.TaskFilter{Page: core.Page{Limit: 2}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.local.ListTasks(f.ctx, core.TaskFilter{PageNumber: 2, Page: core.Page{Limit: 2, Cursor: first.NextCursor}})
	if core.KindOf(err) != core.KindInvalid {
		t.Errorf("page number with a cursor: err = %v, want invalid", err)
	}
}
