package cmd

import (
	"context"
	"time"

	"github.com/spf13/cobra"
	"github.com/thereisnotime/tix/internal/core"
)

// completionTimeout bounds every dynamic lookup so a shell never hangs on an
// unreachable target.
const completionTimeout = 2 * time.Second

// fixedCompletion offers a static list of candidates.
func fixedCompletion(values []string) func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return values, cobra.ShellCompDirectiveNoFileComp
	}
}

// registerCompletions attaches dynamic completion to the named flags.
func registerCompletions(g *globals, cmd *cobra.Command, names ...string) {
	for _, name := range names {
		switch name {
		case "project":
			_ = cmd.RegisterFlagCompletionFunc(name, flagCompletion(g.completeProjectKeys))
		case "tag":
			_ = cmd.RegisterFlagCompletionFunc(name, flagCompletion(g.completeLabels))
		case "status":
			_ = cmd.RegisterFlagCompletionFunc(name, flagCompletion(g.completeStatuses))
		}
	}
}

// flagCompletion adapts an argument completion function to a flag.
func flagCompletion(fn func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective)) func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return fn
}

// completeTaskRefs offers existing task references.
func (g *globals) completeTaskRefs(cmd *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	return g.candidates(cmd, func(ctx context.Context, svc core.Service) ([]string, error) {
		page, err := svc.ListTasks(ctx, core.TaskFilter{Page: core.Page{Limit: core.MaxPageLimit}})
		if err != nil {
			return nil, err
		}
		out := make([]string, 0, len(page.Tasks))
		for _, task := range page.Tasks {
			out = append(out, task.Ref)
		}
		return out, nil
	})
}

// completeProjectKeys offers existing project keys.
func (g *globals) completeProjectKeys(cmd *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	return g.candidates(cmd, func(ctx context.Context, svc core.Service) ([]string, error) {
		projects, _, err := svc.ListProjects(ctx, core.ProjectFilter{Page: core.Page{Limit: core.MaxPageLimit}})
		if err != nil {
			return nil, err
		}
		out := make([]string, 0, len(projects))
		for _, p := range projects {
			out = append(out, p.Key)
		}
		return out, nil
	})
}

// completeLabels offers existing tag names.
func (g *globals) completeLabels(cmd *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	return g.candidates(cmd, func(ctx context.Context, svc core.Service) ([]string, error) {
		tags, err := svc.ListTags(ctx)
		if err != nil {
			return nil, err
		}
		out := make([]string, 0, len(tags))
		for _, l := range tags {
			out = append(out, l.Name)
		}
		return out, nil
	})
}

// completeStatuses offers the states defined by the configured workflows.
func (g *globals) completeStatuses(cmd *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	return g.candidates(cmd, func(ctx context.Context, svc core.Service) ([]string, error) {
		workflows, err := svc.ListWorkflows(ctx)
		if err != nil {
			return nil, err
		}
		seen := map[string]bool{}
		var out []string
		for _, wf := range workflows {
			for _, state := range wf.Definition.States {
				if !seen[state.Key] {
					seen[state.Key] = true
					out = append(out, state.Key)
				}
			}
		}
		return out, nil
	})
}

// candidates runs a dynamic lookup under a deadline, offering nothing on failure.
func (g *globals) candidates(cmd *cobra.Command, fn func(context.Context, core.Service) ([]string, error)) ([]string, cobra.ShellCompDirective) {
	conn, ctx, err := g.dial(cmd)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp | cobra.ShellCompDirectiveError
	}
	ctx, cancel := context.WithTimeout(ctx, completionTimeout)
	defer cancel()
	values, err := fn(ctx, conn.Service)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp | cobra.ShellCompDirectiveError
	}
	return values, cobra.ShellCompDirectiveNoFileComp
}
