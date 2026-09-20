package cmd

import (
	"strings"

	"github.com/spf13/cobra"
	"github.com/thereisnotime/tix/internal/core"
)

// newTaskCmd builds the task command group.
func newTaskCmd(g *globals) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "task",
		Short:   "Create, list and change tasks",
		GroupID: "work",
		Args:    noArgs,
		RunE:    helpRunner,
	}
	cmd.AddCommand(
		taskAddCmd(g), taskLsCmd(g), taskShowCmd(g), taskEditCmd(g),
		taskMvCmd(g), taskRmCmd(g), taskRestoreCmd(g), taskTreeCmd(g),
	)
	return cmd
}

// helpRunner prints help for a group that was invoked without a subcommand.
func helpRunner(cmd *cobra.Command, _ []string) error { return cmd.Help() }

// defaultProject returns the configured default project key. It is consulted
// only when --project was not given, so the flag always wins and the resolver
// decides between environment, .env, configuration file and the default.
func (g *globals) defaultProject() (string, error) {
	resolved, err := g.resolve()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(resolved.Config.Project), nil
}

// projectFallback returns the configured default project, or "" when the
// command was given an explicit --project.
func (g *globals) projectFallback(cmd *cobra.Command) (string, error) {
	if cmd.Flags().Changed("project") {
		return "", nil
	}
	return g.defaultProject()
}

func taskAddCmd(g *globals) *cobra.Command {
	var (
		project, bodyFlag, status, priority, assignee, parent, due string
		tags, depends, fields                                      []string
		dryRun                                                     bool
	)
	cmd := &cobra.Command{
		Use:     "add TITLE...",
		Short:   "Create a task",
		Long:    "Create a task. Only a title is required; the default project and workflow are used.\n\nExit codes: 2 invalid input, 3 unknown project or parent, 5 permission denied.",
		Example: "  tix task add \"buy milk\"\n  tix task add \"ship release\" -p infra --priority high --tag ops",
		Args:    minArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			fallback, err := g.projectFallback(cmd)
			if err != nil {
				return err
			}
			if fallback != "" {
				project = fallback
			}
			title := strings.Join(args, " ")
			if len(args) == 1 && args[0] == StdinMarker {
				read, err := readAll(cmd.InOrStdin())
				if err != nil {
					return err
				}
				title = strings.TrimSpace(read)
			}
			text, err := body(cmd, bodyFlag)
			if err != nil {
				return err
			}
			prio, err := parsePriority(priority)
			if err != nil {
				return err
			}
			dueAt, err := parseTime(due)
			if err != nil {
				return err
			}
			custom, err := parseFields(fields)
			if err != nil {
				return err
			}
			in := core.CreateTaskInput{
				ProjectRef: project, Title: strings.TrimSpace(title), Body: text,
				Status: status, Priority: prio, Tags: tags,
				AssigneeActorID: assignee, ParentRef: parent, DueAt: dueAt,
				CustomFields: custom, DependsOn: depends,
			}
			if err := in.Validate(); err != nil {
				return err
			}
			if dryRun {
				return g.render(cmd, newPlan("task.create", in.Title,
					map[string]any{"project": project, "status": status, "tags": tags}))
			}
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			task, err := conn.Service.CreateTask(ctx, in)
			if err != nil {
				return err
			}
			return g.render(cmd, task)
		},
	}
	f := cmd.Flags()
	f.StringVarP(&project, "project", "p", "", "project key the task belongs to")
	f.StringVar(&bodyFlag, "body", "", "task body, or - to read standard input")
	f.StringVar(&status, "status", "", "initial status")
	f.StringVar(&priority, "priority", "", "priority name or number")
	f.StringVar(&assignee, "assignee", "", "actor the task is assigned to")
	f.StringVar(&parent, "parent", "", "parent task reference")
	f.StringVar(&due, "due", "", "due date")
	f.StringSliceVarP(&tags, "tag", "l", nil, "tag to attach, repeatable")
	f.StringSliceVar(&depends, "depends-on", nil, "task this one depends on, repeatable")
	f.StringSliceVar(&fields, "field", nil, "custom field as key=value, repeatable")
	f.BoolVar(&dryRun, "dry-run", false, "report what would be created without writing")
	registerCompletions(g, cmd, "project", "status", "tag")
	return cmd
}

func taskLsCmd(g *globals) *cobra.Command {
	var (
		projects, statuses, tags, assignees []string
		query, cursor, sort                 string
		limit                               int
		desc, all, deleted                  bool
		claimed, unclaimed, blocked         bool
	)
	cmd := &cobra.Command{
		Use:     "ls",
		Aliases: []string{"list"},
		Short:   "List tasks",
		Long:    "List tasks. Results stream, so large listings are never buffered.\n\nExit codes: 2 invalid filter, 5 permission denied.",
		Example: "  tix task ls\n  tix task ls --status todo -o ndjson\n  tix task ls -p infra --tag ops --all",
		Args:    noArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			fallback, err := g.projectFallback(cmd)
			if err != nil {
				return err
			}
			if fallback != "" {
				projects = []string{fallback}
			}
			filter := core.TaskFilter{
				ProjectKeys: projects, Statuses: statuses, Tags: tags,
				AssigneeIDs: assignees, Query: query, IncludeDeleted: deleted,
				Page: core.Page{Limit: limit, Cursor: cursor, Sort: sort, Direction: core.Ascending},
			}
			if desc {
				filter.Page.Direction = core.Descending
			}
			switch {
			case claimed && unclaimed:
				return usagef(cmd, "--claimed and --unclaimed are mutually exclusive")
			case claimed:
				filter.Claimed = core.Yes
			case unclaimed:
				filter.Claimed = core.No
			}
			if blocked {
				filter.Blocked = core.Yes
			}
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			out := newList[core.Task](g, cmd)
			for {
				page, err := conn.Service.ListTasks(ctx, filter)
				if err != nil {
					return out.fail(err)
				}
				for _, task := range page.Tasks {
					if err := out.Write(task); err != nil {
						return out.fail(err)
					}
				}
				if !all || page.NextCursor == "" {
					if page.NextCursor != "" {
						g.diag(cmd, "more results available; next cursor %s", page.NextCursor)
					}
					return out.Close()
				}
				filter.Page.Cursor = page.NextCursor
			}
		},
	}
	f := cmd.Flags()
	f.StringSliceVarP(&projects, "project", "p", nil, "restrict to project keys")
	f.StringSliceVarP(&statuses, "status", "s", nil, "restrict to statuses")
	f.StringSliceVarP(&tags, "tag", "l", nil, "restrict to tags")
	f.StringSliceVar(&assignees, "assignee", nil, "restrict to assignees")
	f.StringVar(&query, "query", "", "match title and body text")
	f.StringVar(&cursor, "cursor", "", "continue from a previous page")
	f.StringVar(&sort, "sort", core.SortCreatedAt, "sort field")
	f.IntVar(&limit, "limit", core.DefaultPageLimit, "maximum records per page")
	f.BoolVar(&desc, "desc", false, "sort descending")
	f.BoolVar(&all, "all", false, "follow cursors until every page is read")
	f.BoolVar(&deleted, "include-deleted", false, "include soft-deleted tasks")
	f.BoolVar(&claimed, "claimed", false, "only tasks currently claimed")
	f.BoolVar(&unclaimed, "unclaimed", false, "only tasks not currently claimed")
	f.BoolVar(&blocked, "blocked", false, "only tasks blocked by a dependency")
	_ = cmd.RegisterFlagCompletionFunc("sort", fixedCompletion(core.TaskSortFields))
	registerCompletions(g, cmd, "project", "status", "tag")
	return cmd
}

func taskShowCmd(g *globals) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "show REF",
		Short:   "Show one task",
		Long:    "Show one task by ULID or by project reference such as infra-42.\n\nExit codes: 3 unknown reference, 5 permission denied.",
		Example: "  tix task show default-1\n  tix task show default-1 -o yaml",
		Args:    exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ref, err := parseRef(args[0])
			if err != nil {
				return err
			}
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			task, err := conn.Service.GetTask(ctx, ref)
			if err != nil {
				return err
			}
			return g.render(cmd, task)
		},
		ValidArgsFunction: g.completeTaskRefs,
	}
	return cmd
}

func taskEditCmd(g *globals) *cobra.Command {
	var (
		title, bodyFlag, priority, assignee, parent, due string
		tags, fields                                     []string
		version                                          int
		dryRun                                           bool
	)
	cmd := &cobra.Command{
		Use:     "edit REF",
		Short:   "Change a task",
		Long:    "Change a task's attributes. Only the flags you pass are applied.\n\nExit codes: 3 unknown reference, 4 version conflict, 5 permission denied.",
		Example: "  tix task edit default-1 --title \"buy oat milk\"\n  tix task edit default-1 --priority high --dry-run",
		Args:    exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ref, err := parseRef(args[0])
			if err != nil {
				return err
			}
			in := core.UpdateTaskInput{Version: version}
			if cmd.Flags().Changed("title") {
				in.Title = &title
			}
			if cmd.Flags().Changed("body") {
				text, err := body(cmd, bodyFlag)
				if err != nil {
					return err
				}
				in.Body = &text
			}
			if cmd.Flags().Changed("priority") {
				prio, err := parsePriority(priority)
				if err != nil {
					return err
				}
				in.Priority = &prio
			}
			if cmd.Flags().Changed("assignee") {
				in.AssigneeActorID = &assignee
			}
			if cmd.Flags().Changed("parent") {
				in.ParentRef = &parent
			}
			if cmd.Flags().Changed("due") {
				dueAt, err := parseTime(due)
				if err != nil {
					return err
				}
				in.DueAt = &dueAt
			}
			if cmd.Flags().Changed("tag") {
				in.Tags = &tags
			}
			custom, err := parseFields(fields)
			if err != nil {
				return err
			}
			in.CustomFields = custom

			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			if dryRun {
				if _, err := conn.Service.GetTask(ctx, ref); err != nil {
					return err
				}
				return g.render(cmd, newPlan("task.update", ref.String(), nil))
			}
			task, err := conn.Service.UpdateTask(ctx, ref, in)
			if err != nil {
				return err
			}
			return g.render(cmd, task)
		},
		ValidArgsFunction: g.completeTaskRefs,
	}
	f := cmd.Flags()
	f.StringVar(&title, "title", "", "new title")
	f.StringVar(&bodyFlag, "body", "", "new body, or - to read standard input")
	f.StringVar(&priority, "priority", "", "new priority")
	f.StringVar(&assignee, "assignee", "", "new assignee actor id")
	f.StringVar(&parent, "parent", "", "new parent reference")
	f.StringVar(&due, "due", "", "new due date")
	f.StringSliceVarP(&tags, "tag", "l", nil, "replace the tag set")
	f.StringSliceVar(&fields, "field", nil, "custom field as key=value, repeatable")
	f.IntVar(&version, "version", 0, "expected version for optimistic locking")
	f.BoolVar(&dryRun, "dry-run", false, "report what would change without writing")
	registerCompletions(g, cmd, "tag")
	return cmd
}

func taskMvCmd(g *globals) *cobra.Command {
	var (
		comment, leaseToken string
		fields              []string
		dryRun              bool
	)
	cmd := &cobra.Command{
		Use:     "mv REF... STATUS",
		Aliases: []string{"transition"},
		Short:   "Move tasks to a status",
		Long:    "Move one or more tasks to a status. Pass - as a reference to read references from standard input.\n\nExit codes: 3 unknown reference, 4 lease expired, 6 illegal transition.",
		Example: "  tix task mv default-1 doing\n  tix task ls -o ndjson | jq -r .ref | tix task mv - done",
		Args:    minArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			status := args[len(args)-1]
			refs, err := readRefs(cmd, args[:len(args)-1])
			if err != nil {
				return err
			}
			custom, err := parseFields(fields)
			if err != nil {
				return err
			}
			in := core.TransitionInput{To: status, Comment: comment, LeaseToken: leaseToken, CustomFields: custom}
			if err := in.Validate(); err != nil {
				return err
			}
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			if dryRun {
				return g.dryBulk(cmd, conn, ctx, refs, "task.transition",
					map[string]any{"to": status})
			}
			return g.bulk(cmd, refs, func(ref core.TaskRef) error {
				_, err := conn.Service.TransitionTask(ctx, ref, in)
				return err
			})
		},
		ValidArgsFunction: g.completeTaskRefs,
	}
	f := cmd.Flags()
	f.StringVar(&comment, "comment", "", "comment recorded with the transition")
	f.StringVar(&leaseToken, "lease-token", "", "lease token held on the task")
	f.StringSliceVar(&fields, "field", nil, "custom field as key=value, repeatable")
	f.BoolVar(&dryRun, "dry-run", false, "report what would change without writing")
	registerCompletions(g, cmd, "status")
	return cmd
}

func taskRmCmd(g *globals) *cobra.Command {
	var hard, cascade, dryRun bool
	cmd := &cobra.Command{
		Use:     "rm REF...",
		Aliases: []string{"delete"},
		Short:   "Delete tasks",
		Long:    "Soft delete tasks, or remove them permanently with --hard.\n\nA task with subtasks is refused unless --cascade is given.\n\nExit codes: 3 unknown reference, 5 permission denied, 6 the task has subtasks.",
		Example: "  tix task rm default-1\n  tix task rm default-1 --hard\n  tix task rm default-1 --cascade",
		Args:    minArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			refs, err := readRefs(cmd, args)
			if err != nil {
				return err
			}
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			if dryRun {
				return g.dryBulk(cmd, conn, ctx, refs, "task.delete",
					map[string]any{"hard": hard, "cascade": cascade})
			}
			in := core.DeleteTaskInput{Hard: hard, Cascade: cascade}
			return g.bulk(cmd, refs, func(ref core.TaskRef) error {
				return conn.Service.DeleteTask(ctx, ref, in)
			})
		},
		ValidArgsFunction: g.completeTaskRefs,
	}
	cmd.Flags().BoolVar(&hard, "hard", false, "delete permanently instead of soft deleting")
	cmd.Flags().BoolVar(&cascade, "cascade", false, "also delete the task's subtasks")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "report what would be deleted without writing")
	return cmd
}

func taskRestoreCmd(g *globals) *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:     "restore REF...",
		Short:   "Restore soft-deleted tasks",
		Long:    "Restore tasks that were soft deleted.\n\nExit codes: 3 unknown reference, 5 permission denied.",
		Example: "  tix task restore default-1",
		Args:    minArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			refs, err := readRefs(cmd, args)
			if err != nil {
				return err
			}
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			if dryRun {
				return g.render(cmd, newPlan("task.restore", refsString(refs), nil))
			}
			return g.bulk(cmd, refs, func(ref core.TaskRef) error {
				_, err := conn.Service.RestoreTask(ctx, ref)
				return err
			})
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "report what would be restored without writing")
	return cmd
}

func taskTreeCmd(g *globals) *cobra.Command {
	var depth int
	cmd := &cobra.Command{
		Use:     "tree REF",
		Short:   "Show a task and its descendants",
		Long:    "Show a task with its subtree.\n\nExit codes: 3 unknown reference, 5 permission denied.",
		Example: "  tix task tree default-1 --depth 2",
		Args:    exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ref, err := parseRef(args[0])
			if err != nil {
				return err
			}
			conn, ctx, err := g.dial(cmd)
			if err != nil {
				return err
			}
			tasks, err := conn.Service.TaskTree(ctx, ref, depth)
			if err != nil {
				return err
			}
			return g.render(cmd, tasks)
		},
		ValidArgsFunction: g.completeTaskRefs,
	}
	cmd.Flags().IntVar(&depth, "depth", 0, "maximum depth, zero for unlimited")
	return cmd
}

// refsString renders a reference list for a dry-run plan.
func refsString(refs []core.TaskRef) string {
	out := make([]string, 0, len(refs))
	for _, ref := range refs {
		out = append(out, ref.String())
	}
	return strings.Join(out, ",")
}
