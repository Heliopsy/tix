// SPDX-License-Identifier: AGPL-3.0-or-later

package core

import (
	"context"
	"time"
)

// StatsService reports throughput, ageing and who closed what.
type StatsService interface {
	Stats(ctx context.Context, in StatsInput) (*Stats, error)
}

// Statistics defaults and bounds.
const (
	// DefaultStatsWindow is the window a caller who names none is given.
	DefaultStatsWindow = Duration(14 * 24 * time.Hour)
	// MaxStatsWindow bounds the window. The completions inside it are read as
	// rows, so an unbounded window is an unbounded read.
	MaxStatsWindow = Duration(3650 * 24 * time.Hour)
	// DefaultStatsTopActors is how many actors the leaderboard names.
	DefaultStatsTopActors = 5
	// MaxStatsTopActors bounds the leaderboard.
	MaxStatsTopActors = 100
	// DefaultStatsOldest is how many ageing tasks are reported.
	DefaultStatsOldest = 5
	// MaxStatsOldest bounds the ageing list.
	MaxStatsOldest = 100
)

// StatsLeaderboardMeasure names what TopActors counts. It travels with the
// figures so that no surface can show the leaderboard without saying what the
// number is: a ticket-closing count presented as a productivity measure is
// worse than no leaderboard at all.
const StatsLeaderboardMeasure = "tasks moved to a terminal state"

// StatsInput selects what one statistics read covers. The window always ends
// at the moment of the read.
type StatsInput struct {
	// ProjectRef narrows to one project. Empty covers the whole tenant.
	ProjectRef string `json:"project_ref,omitempty" yaml:"project_ref,omitempty"`
	// Window is how far back the window reaches. Ignored when Since is set.
	Window Duration `json:"window,omitempty" yaml:"window,omitempty"`
	// Since pins the start of the window to an instant instead of a length.
	Since time.Time `json:"since,omitempty" yaml:"since,omitempty"`
	// TopActors is how many actors the leaderboard names.
	TopActors int `json:"top_actors,omitempty" yaml:"top_actors,omitempty"`
	// Oldest is how many ageing tasks are reported.
	Oldest int `json:"oldest,omitempty" yaml:"oldest,omitempty"`
}

// Validate checks the input.
func (in StatsInput) Validate() error {
	if in.Window < 0 {
		return Invalid("statistics window must not be negative")
	}
	if in.Window > MaxStatsWindow {
		return Invalid("statistics window must be at most %s", MaxStatsWindow)
	}
	if in.TopActors < 0 || in.TopActors > MaxStatsTopActors {
		return Invalid("top actors must be between 0 and %d", MaxStatsTopActors)
	}
	if in.Oldest < 0 || in.Oldest > MaxStatsOldest {
		return Invalid("oldest must be between 0 and %d", MaxStatsOldest)
	}
	return nil
}

// StatsDay is one day of the completion series, keyed by its UTC date.
type StatsDay struct {
	Date      string `json:"date" yaml:"date"`
	Completed int    `json:"completed" yaml:"completed"`
}

// StatsDayLayout is the calendar day a completion is attributed to.
const StatsDayLayout = "2006-01-02"

// StatsCategory is how many tasks are presently in one state category.
type StatsCategory struct {
	Category StateCategory `json:"category" yaml:"category"`
	Count    int           `json:"count" yaml:"count"`
}

// StatsActor counts the tasks one actor moved to a terminal state. See
// StatsLeaderboardMeasure: the count is of moves, not of work done.
type StatsActor struct {
	ActorID string `json:"actor_id" yaml:"actor_id"`
	Handle  string `json:"handle,omitempty" yaml:"handle,omitempty"`
	Moved   int    `json:"moved" yaml:"moved"`
}

// StatsAgeing is one task that has not reached a terminal state, and how long
// it has been waiting.
type StatsAgeing struct {
	Ref       string    `json:"ref" yaml:"ref"`
	Title     string    `json:"title" yaml:"title"`
	Status    string    `json:"status" yaml:"status"`
	CreatedAt time.Time `json:"created_at" yaml:"created_at"`
	Age       Duration  `json:"age" yaml:"age"`
}

// Stats is one tenant's throughput, ageing and leaderboard over a window.
// Every count is zero and every list empty when nothing happened, which is a
// successful answer rather than an error.
type Stats struct {
	// ProjectID and ProjectKey are set only when the read named one project.
	ProjectID  string `json:"project_id,omitempty" yaml:"project_id,omitempty"`
	ProjectKey string `json:"project_key,omitempty" yaml:"project_key,omitempty"`

	Since time.Time `json:"since" yaml:"since"`
	Until time.Time `json:"until" yaml:"until"`

	Completed int        `json:"completed" yaml:"completed"`
	Created   int        `json:"created" yaml:"created"`
	PerDay    []StatsDay `json:"per_day" yaml:"per_day"`

	// MedianLeadTime and SlowestLeadTime cover creation to terminal state, over
	// the tasks that reached one inside the window.
	MedianLeadTime  Duration `json:"median_lead_time" yaml:"median_lead_time"`
	SlowestLeadTime Duration `json:"slowest_lead_time" yaml:"slowest_lead_time"`

	ByCategory []StatsCategory `json:"by_category" yaml:"by_category"`

	TopActors []StatsActor `json:"top_actors" yaml:"top_actors"`
	// LeaderboardMeasure states what TopActors counts, for every surface that
	// shows it.
	LeaderboardMeasure string `json:"leaderboard_measure" yaml:"leaderboard_measure"`

	Oldest []StatsAgeing `json:"oldest" yaml:"oldest"`
}
