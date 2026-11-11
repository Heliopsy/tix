// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"strings"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/tasklink"
)

// WithPublicURL sets the address people reach the browser interface at,
// server.public_url. Every task the service returns then carries web_url, its
// screen under that address; without it no task does.
func WithPublicURL(base string) Option {
	return func(l *Local) { l.publicURL = strings.TrimRight(strings.TrimSpace(base), "/") }
}

// webBase is the address the caller's task links are built under, or "" when
// no public address is configured. A tenant served on a domain bound to it
// links under that domain, with the public address's scheme, port and path:
// the host a request arrived on when it is such a domain, else the tenant's
// own address. Every other tenant links under the public address.
func (l *Local) webBase(ctx context.Context) string {
	if l.publicURL == "" {
		return ""
	}
	if served, ok := core.ServedHostFrom(ctx); ok {
		if !served.Shared && served.Host != "" {
			return tasklink.Rehost(l.publicURL, served.Host)
		}
		return l.publicURL
	}
	scope, ok := core.TenantFrom(ctx)
	if !ok || scope.TenantID == "" {
		return l.publicURL
	}
	if addr, err := l.TenantAddress(ctx, scope.TenantID); err == nil && addr != "" {
		return tasklink.Rehost(l.publicURL, addr)
	}
	return l.publicURL
}

// linkTasks sets web_url on each task, when there is an address to build it
// under. It runs on values already read, after the transaction, so nothing it
// sets reaches an audit entry, an event or an export.
func (l *Local) linkTasks(ctx context.Context, tasks ...*core.Task) {
	if l.publicURL == "" {
		return
	}
	base := ""
	for _, t := range tasks {
		if t == nil || t.Ref == "" {
			continue
		}
		if base == "" {
			base = l.webBase(ctx)
		}
		t.WebURL = tasklink.URL(base, t.Ref)
	}
}

// linkInvite sets an issued invite's url, its path under the same address a
// task's web_url is built under, when there is one. A surface that knows the
// address it was reached at, the HTTP API or the browser, may set its own.
func (l *Local) linkInvite(ctx context.Context, issued *core.IssuedInvite) {
	if l.publicURL == "" || issued == nil || issued.Path == "" {
		return
	}
	issued.URL = strings.TrimRight(l.webBase(ctx), "/") + issued.Path
}

// linked returns t with web_url set, passing an error through.
func (l *Local) linked(ctx context.Context, t *core.Task, err error) (*core.Task, error) {
	if err == nil {
		l.linkTasks(ctx, t)
	}
	return t, err
}

// linkedClaim returns c with web_url set on its task.
func (l *Local) linkedClaim(ctx context.Context, c *core.Claim, err error) (*core.Claim, error) {
	if err == nil && c != nil {
		l.linkTasks(ctx, c.Task)
	}
	return c, err
}

// linkSlice sets web_url on every task in a slice.
func (l *Local) linkSlice(ctx context.Context, tasks []core.Task) {
	ptrs := make([]*core.Task, len(tasks))
	for i := range tasks {
		ptrs[i] = &tasks[i]
	}
	l.linkTasks(ctx, ptrs...)
}

// CreateTask creates a task; see core.Service.
func (l *Local) CreateTask(ctx context.Context, in core.CreateTaskInput) (*core.Task, error) {
	t, err := l.createTask(ctx, in)
	return l.linked(ctx, t, err)
}

// GetTask returns one task; see core.Service.
func (l *Local) GetTask(ctx context.Context, ref core.TaskRef) (*core.Task, error) {
	t, err := l.getTask(ctx, ref)
	return l.linked(ctx, t, err)
}

// ListTasks returns a page of tasks; see core.Service.
func (l *Local) ListTasks(ctx context.Context, f core.TaskFilter) (core.TaskPage, error) {
	page, err := l.listTasks(ctx, f)
	if err == nil {
		l.linkSlice(ctx, page.Tasks)
	}
	return page, err
}

// UpdateTask changes a task; see core.Service.
func (l *Local) UpdateTask(ctx context.Context, ref core.TaskRef, in core.UpdateTaskInput) (*core.Task, error) {
	t, err := l.updateTask(ctx, ref, in)
	return l.linked(ctx, t, err)
}

// TransitionTask moves a task to another state; see core.Service.
func (l *Local) TransitionTask(ctx context.Context, ref core.TaskRef, in core.TransitionInput) (*core.Task, error) {
	t, err := l.transitionTask(ctx, ref, in)
	return l.linked(ctx, t, err)
}

// TransitionRoute moves a task along a route of states; see core.Service.
func (l *Local) TransitionRoute(ctx context.Context, ref core.TaskRef, in core.RouteInput) (*core.RouteResult, error) {
	r, err := l.transitionRoute(ctx, ref, in)
	if err == nil && r != nil {
		l.linkTasks(ctx, r.Task)
	}
	return r, err
}

// RestoreTask brings a task back from the trash; see core.Service.
func (l *Local) RestoreTask(ctx context.Context, ref core.TaskRef) (*core.Task, error) {
	t, err := l.restoreTask(ctx, ref)
	return l.linked(ctx, t, err)
}

// TaskTree returns a task and its descendants; see core.Service.
func (l *Local) TaskTree(ctx context.Context, ref core.TaskRef, depth int) ([]core.Task, error) {
	tasks, err := l.taskTree(ctx, ref, depth)
	if err == nil {
		l.linkSlice(ctx, tasks)
	}
	return tasks, err
}

// CopyTask copies a task; see core.Service.
func (l *Local) CopyTask(ctx context.Context, ref core.TaskRef, in core.CopyTaskInput) (*core.Task, error) {
	t, err := l.copyTask(ctx, ref, in)
	return l.linked(ctx, t, err)
}

// MoveTask moves a task to another project; see core.Service.
func (l *Local) MoveTask(ctx context.Context, ref core.TaskRef, in core.MoveTaskInput) (*core.TaskMove, error) {
	out, err := l.moveTask(ctx, ref, in)
	if err == nil {
		l.linkTasks(ctx, &out.Task)
	}
	return out, err
}

// CheckTaskItem ticks or clears a checklist item; see core.Service.
func (l *Local) CheckTaskItem(ctx context.Context, ref core.TaskRef, in core.CheckItemInput) (*core.Task, error) {
	t, err := l.checkTaskItem(ctx, ref, in)
	return l.linked(ctx, t, err)
}

// SnoozeTask hides a task until a time; see core.Service.
func (l *Local) SnoozeTask(ctx context.Context, ref core.TaskRef, in core.SnoozeInput) (*core.Task, error) {
	t, err := l.snoozeTask(ctx, ref, in)
	return l.linked(ctx, t, err)
}

// UnsnoozeTask ends a snooze; see core.Service.
func (l *Local) UnsnoozeTask(ctx context.Context, ref core.TaskRef) (*core.Task, error) {
	t, err := l.unsnoozeTask(ctx, ref)
	return l.linked(ctx, t, err)
}

// UseTemplate creates a task from a template; see core.Service.
func (l *Local) UseTemplate(ctx context.Context, ref string, in core.UseTemplateInput) (*core.Task, error) {
	t, err := l.useTemplate(ctx, ref, in)
	return l.linked(ctx, t, err)
}

// ForceReclaim ends another worker's lease; see core.Service.
func (l *Local) ForceReclaim(ctx context.Context, ref core.TaskRef, in core.ForceReclaimInput) (*core.Task, error) {
	t, err := l.forceReclaim(ctx, ref, in)
	return l.linked(ctx, t, err)
}

// ClaimTask claims a task; see core.Service.
func (l *Local) ClaimTask(ctx context.Context, ref core.TaskRef, in core.ClaimInput) (*core.Claim, error) {
	c, err := l.claimTask(ctx, ref, in)
	return l.linkedClaim(ctx, c, err)
}

// ClaimNext claims the next task the queue offers; see core.Service.
func (l *Local) ClaimNext(ctx context.Context, in core.ClaimNextInput) (*core.Claim, error) {
	c, err := l.claimNext(ctx, in)
	return l.linkedClaim(ctx, c, err)
}

// RenewLease extends a held lease; see core.Service.
func (l *Local) RenewLease(ctx context.Context, ref core.TaskRef, token string, ttl core.Duration) (*core.Claim, error) {
	c, err := l.renewLease(ctx, ref, token, ttl)
	return l.linkedClaim(ctx, c, err)
}
