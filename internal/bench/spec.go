// SPDX-License-Identifier: AGPL-3.0-or-later

// Package bench builds large task fixtures and measures the operations that
// decide whether tix holds up at a million tasks.
package bench

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// Spec describes the shape of a generated dataset.
type Spec struct {
	Name     string
	Tenants  int
	Projects int
	Tasks    int
	Tags     int
	// DepRatio gives one dependency edge per DepRatio tasks. Zero adds none.
	DepRatio int
	// TagRatio attaches a tag to one task per TagRatio tasks. Zero attaches none.
	TagRatio int
	// Batch is how many tasks share one write transaction.
	Batch int
}

// Fixture profiles. Small is what `go test ./...` builds; Full is the design
// target and is opt-in because seeding it takes minutes.
var (
	// Small seeds in well under a second and still pages deeply enough for a
	// forgotten index to show.
	Small = Spec{Name: "small", Tenants: 2, Projects: 6, Tasks: 3000, Tags: 8, DepRatio: 20, TagRatio: 5, Batch: 1500}
	// Medium is the size the committed budgets were measured at.
	Medium = Spec{Name: "medium", Tenants: 3, Projects: 20, Tasks: 100_000, Tags: 16, DepRatio: 20, TagRatio: 5, Batch: 5000}
	// Full is the design target: a million tasks over fifty projects and five tenants.
	Full = Spec{Name: "full", Tenants: 5, Projects: 50, Tasks: 1_000_000, Tags: 32, DepRatio: 20, TagRatio: 5, Batch: 10_000}
)

// SizeEnv names the variable that chooses a profile: small, medium, full, or a
// task count. Unset means Small, so the default test run stays fast.
const SizeEnv = "TIX_BENCH_SIZE"

// PostgresEnv names the DSN variable. Unset skips every Postgres run, matching
// the convention the engine suites already use.
const PostgresEnv = "TIX_TEST_POSTGRES_DSN"

// EngineEnv restricts the run to one engine, "sqlite" or "postgres". Unset runs
// every engine that is available.
const EngineEnv = "TIX_BENCH_ENGINE"

// SpecFromEnv resolves the profile named by SizeEnv, defaulting to Small.
func SpecFromEnv() Spec {
	switch v := strings.ToLower(strings.TrimSpace(os.Getenv(SizeEnv))); v {
	case "", "small":
		return Small
	case "medium":
		return Medium
	case "full", "1m":
		return Full
	default:
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			return Small
		}
		s := Full
		s.Name = v
		s.Tasks = n
		if n < 50_000 {
			s.Batch = 2000
		}
		return s
	}
}

// Statuses are the workflow states every seeded project uses.
var Statuses = []string{"todo", "doing", "review", "done", "cancelled"}

// TerminalStatuses are the states that unblock a dependent task.
var TerminalStatuses = []string{"done", "cancelled"}

// Epoch is the creation time of the first seeded task. Each later task is one
// Step older, so (created_at, id) is a total order and a cursor for any depth
// can be computed without walking there.
var Epoch = time.Date(2024, time.January, 1, 0, 0, 0, 0, time.UTC)

// Step separates the creation times of consecutive tasks within a tenant.
const Step = time.Second
