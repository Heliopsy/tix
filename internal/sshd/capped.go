package sshd

import (
	"context"

	"github.com/heliopsy/tix/internal/core"
)

// capped is the service a session is given: the real one, with a ceiling on
// how many tasks a single sandbox may hold. Without it one visitor in a loop
// grows the shared database without bound, which is the difference between a
// demo and an availability problem.
//
// It is a wrapper rather than a rule in the service because the ceiling is a
// property of this listener, not of tix: no other caller wants it.
type capped struct {
	core.Service
	limit int
}

// CreateTask refuses once the sandbox is full.
//
// The count comes from the same tenant-scoped listing every other reader uses,
// so it can never see another sandbox's rows, and asking for exactly the limit
// is enough to answer the only question there is.
func (c capped) CreateTask(ctx context.Context, in core.CreateTaskInput) (*core.Task, error) {
	page, err := c.ListTasks(ctx, core.TaskFilter{
		IncludeDeleted: true,
		Page:           core.Page{Limit: c.limit},
	})
	if err != nil {
		return nil, err
	}
	if len(page.Tasks) >= c.limit {
		return nil, core.Precondition(
			"this sandbox holds its limit of %d tasks; delete one before adding another", c.limit)
	}
	return c.Service.CreateTask(ctx, in)
}
