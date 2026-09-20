package output

import (
	"fmt"
	"io"
	"os"
	"reflect"
	"strconv"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/thereisnotime/tix/internal/core"
)

type tableFormatter struct{}

// Format renders data as a terminal table, falling back to reflection for unknown types.
func (t *tableFormatter) Format(w io.Writer, data any) error {
	if data == nil {
		return noResults(w)
	}
	if rv := reflect.ValueOf(data); rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return noResults(w)
		}
		return t.Format(w, rv.Elem().Interface())
	}

	switch v := data.(type) {
	case core.TaskPage:
		return renderTaskPage(w, v)
	case []core.Task:
		return renderRows(w, taskHeader, v, taskRow)
	case []*core.Task:
		return renderRows(w, taskHeader, deref(v), taskRow)
	case core.Task:
		return renderRows(w, taskHeader, []core.Task{v}, taskRow)
	case []core.Project:
		return renderRows(w, projectHeader, v, projectRow)
	case []*core.Project:
		return renderRows(w, projectHeader, deref(v), projectRow)
	case core.Project:
		return renderRows(w, projectHeader, []core.Project{v}, projectRow)
	case []core.Workflow:
		return renderRows(w, workflowHeader, v, workflowRow)
	case []*core.Workflow:
		return renderRows(w, workflowHeader, deref(v), workflowRow)
	case core.Workflow:
		return renderRows(w, workflowHeader, []core.Workflow{v}, workflowRow)
	case []core.Comment:
		return renderRows(w, commentHeader, v, commentRow)
	case []*core.Comment:
		return renderRows(w, commentHeader, deref(v), commentRow)
	case core.Comment:
		return renderRows(w, commentHeader, []core.Comment{v}, commentRow)
	case []core.APIToken:
		return renderRows(w, tokenHeader, v, tokenRow)
	case []*core.APIToken:
		return renderRows(w, tokenHeader, deref(v), tokenRow)
	case core.APIToken:
		return renderRows(w, tokenHeader, []core.APIToken{v}, tokenRow)
	case []core.AuditEntry:
		return renderRows(w, auditHeader, v, auditRow)
	case []*core.AuditEntry:
		return renderRows(w, auditHeader, deref(v), auditRow)
	case core.AuditEntry:
		return renderRows(w, auditHeader, []core.AuditEntry{v}, auditRow)
	case []core.WebhookEndpoint:
		return renderRows(w, endpointHeader, v, endpointRow)
	case []*core.WebhookEndpoint:
		return renderRows(w, endpointHeader, deref(v), endpointRow)
	case core.WebhookEndpoint:
		return renderRows(w, endpointHeader, []core.WebhookEndpoint{v}, endpointRow)
	case []core.Tenant:
		return renderRows(w, tenantHeader, v, tenantRow)
	case []*core.Tenant:
		return renderRows(w, tenantHeader, deref(v), tenantRow)
	case core.Tenant:
		return renderRows(w, tenantHeader, []core.Tenant{v}, tenantRow)
	}
	return renderReflected(w, data)
}

var (
	taskHeader     = table.Row{"REF", "TITLE", "STATUS", "PRIORITY", "ASSIGNEE", "LABELS", "DUE", "UPDATED", "BLOCKED"}
	projectHeader  = table.Row{"KEY", "NAME", "WORKFLOW", "ARCHIVED", "CREATED", "UPDATED"}
	workflowHeader = table.Row{"KEY", "NAME", "INITIAL", "STATES", "TRANSITIONS", "BUILTIN", "UPDATED"}
	commentHeader  = table.Row{"ID", "TASK", "AUTHOR", "BODY", "CREATED"}
	tokenHeader    = table.Row{"ID", "NAME", "ACTOR", "SCOPES", "PROJECT", "CREATED", "EXPIRES", "LAST USED", "REVOKED"}
	auditHeader    = table.Row{"SEQ", "ACTION", "SUBJECT", "ACTOR", "SOURCE", "OCCURRED"}
	endpointHeader = table.Row{"ID", "URL", "EVENTS", "ACTIVE", "CREATED"}
	tenantHeader   = table.Row{"ID", "KEY", "NAME", "CREATED", "DELETED"}
)

func taskRow(t core.Task) table.Row {
	return table.Row{
		t.Ref,
		truncate(t.Title, 60),
		t.Status,
		priorityLabel(t.Priority),
		t.AssigneeActorID,
		strings.Join(t.Labels, ", "),
		FormatCompactPtr(t.DueAt),
		FormatCompact(t.UpdatedAt),
		yesNo(t.Blocked),
	}
}

func projectRow(p core.Project) table.Row {
	return table.Row{
		p.Key, truncate(p.Name, 40), p.WorkflowID,
		yesNo(p.Archived()), FormatCompact(p.CreatedAt), FormatCompact(p.UpdatedAt),
	}
}

func workflowRow(wf core.Workflow) table.Row {
	return table.Row{
		wf.Key, truncate(wf.Name, 40), wf.Definition.Initial,
		strconv.Itoa(len(wf.Definition.States)),
		strconv.Itoa(len(wf.Definition.Transitions)),
		yesNo(wf.Builtin), FormatCompact(wf.UpdatedAt),
	}
}

func commentRow(c core.Comment) table.Row {
	return table.Row{c.ID, c.TaskID, c.AuthorActorID, truncate(c.Body, 60), FormatCompact(c.CreatedAt)}
}

func tokenRow(t core.APIToken) table.Row {
	scopes := make([]string, 0, len(t.Scopes))
	for _, s := range t.Scopes {
		scopes = append(scopes, string(s))
	}
	return table.Row{
		t.ID, t.Name, t.ActorID, strings.Join(scopes, ", "), t.ProjectID,
		FormatCompact(t.CreatedAt), FormatCompactPtr(t.ExpiresAt),
		FormatCompactPtr(t.LastUsedAt), FormatCompactPtr(t.RevokedAt),
	}
}

func auditRow(a core.AuditEntry) table.Row {
	subject := a.SubjectType
	if a.SubjectID != "" {
		subject = a.SubjectType + "/" + a.SubjectID
	}
	return table.Row{
		strconv.FormatInt(a.Seq, 10), a.Action, subject, a.ActorID,
		string(a.Source), FormatCompact(a.OccurredAt),
	}
}

func endpointRow(e core.WebhookEndpoint) table.Row {
	return table.Row{
		e.ID, truncate(e.URL, 60), strings.Join(e.EventTypes, ", "),
		yesNo(e.Active), FormatCompact(e.CreatedAt),
	}
}

func tenantRow(t core.Tenant) table.Row {
	return table.Row{t.ID, t.Key, truncate(t.Name, 40), FormatCompact(t.CreatedAt), FormatCompactPtr(t.DeletedAt)}
}

func renderTaskPage(w io.Writer, p core.TaskPage) error {
	if err := renderRows(w, taskHeader, p.Tasks, taskRow); err != nil {
		return err
	}
	if p.NextCursor == "" {
		return nil
	}
	if _, err := fmt.Fprintf(w, "next cursor: %s\n", p.NextCursor); err != nil {
		return fmt.Errorf("writing cursor: %w", err)
	}
	return nil
}

func renderRows[T any](w io.Writer, header table.Row, items []T, row func(T) table.Row) error {
	tbl := newWriter(w)
	tbl.AppendHeader(header)
	for _, item := range items {
		tbl.AppendRow(row(item))
	}
	tbl.Render()
	return nil
}

func renderReflected(w io.Writer, data any) error {
	rv := reflect.ValueOf(data)
	if rv.Kind() != reflect.Slice {
		if rv.Kind() == reflect.Struct {
			return renderStructs(w, reflect.Append(reflect.MakeSlice(reflect.SliceOf(rv.Type()), 0, 1), rv))
		}
		_, err := fmt.Fprintf(w, "%v\n", data)
		return err
	}
	elem := rv.Type().Elem()
	if elem.Kind() == reflect.Pointer {
		elem = elem.Elem()
	}
	if elem.Kind() != reflect.Struct {
		if rv.Len() == 0 {
			return noResults(w)
		}
		for i := 0; i < rv.Len(); i++ {
			if _, err := fmt.Fprintf(w, "%v\n", rv.Index(i).Interface()); err != nil {
				return err
			}
		}
		return nil
	}
	return renderStructs(w, rv)
}

func renderStructs(w io.Writer, rv reflect.Value) error {
	elem := rv.Type().Elem()
	if elem.Kind() == reflect.Pointer {
		elem = elem.Elem()
	}
	tbl := newWriter(w)
	header := make(table.Row, 0, elem.NumField())
	for i := 0; i < elem.NumField(); i++ {
		header = append(header, elem.Field(i).Name)
	}
	tbl.AppendHeader(header)
	for i := 0; i < rv.Len(); i++ {
		item := rv.Index(i)
		if item.Kind() == reflect.Pointer {
			if item.IsNil() {
				continue
			}
			item = item.Elem()
		}
		row := make(table.Row, 0, elem.NumField())
		for f := 0; f < elem.NumField(); f++ {
			row = append(row, fmt.Sprintf("%v", item.Field(f).Interface()))
		}
		tbl.AppendRow(row)
	}
	tbl.Render()
	return nil
}

func newWriter(w io.Writer) table.Writer {
	tbl := table.NewWriter()
	tbl.SetOutputMirror(w)
	style := table.StyleLight
	if !colorEnabled(w) {
		style.Color = table.ColorOptions{}
	} else {
		style.Color.Header = text.Colors{text.Bold}
	}
	tbl.SetStyle(style)
	return tbl
}

// colorEnabled reports whether ANSI colour may be written to w.
func colorEnabled(w io.Writer) bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

func noResults(w io.Writer) error {
	_, err := fmt.Fprintln(w, "No results.")
	return err
}

func deref[T any](items []*T) []T {
	out := make([]T, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		out = append(out, *item)
	}
	return out
}

func priorityLabel(p core.Priority) string {
	switch p {
	case core.PriorityHighest:
		return "highest"
	case core.PriorityHigh:
		return "high"
	case core.PriorityNormal:
		return "normal"
	case core.PriorityLow:
		return "low"
	case core.PriorityLowest:
		return "lowest"
	default:
		return strconv.Itoa(int(p))
	}
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

func truncate(s string, max int) string {
	s = strings.Join(strings.Fields(s), " ")
	if len([]rune(s)) <= max {
		return s
	}
	return string([]rune(s)[:max-1]) + "…"
}
