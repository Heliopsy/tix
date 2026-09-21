package output

import (
	"fmt"
	"io"

	"reflect"
	"regexp"
	"strconv"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"

	"github.com/heliopsy/tix/internal/core"
)

type tableFormatter struct{ mode Mode }

// Format renders data as a terminal table, falling back to reflection for unknown types.
func (t *tableFormatter) Format(w io.Writer, data any) error {
	p := NewPainter(t.mode, w)
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
		return renderTaskPage(w, p, v)
	case []core.Task:
		return renderRows(w, p, taskHeader, v, p.taskRow)
	case []*core.Task:
		return renderRows(w, p, taskHeader, deref(v), p.taskRow)
	case core.Task:
		return renderRows(w, p, taskHeader, []core.Task{v}, p.taskRow)
	case []core.Project:
		return renderRows(w, p, projectHeader, v, p.projectRow)
	case []*core.Project:
		return renderRows(w, p, projectHeader, deref(v), p.projectRow)
	case core.Project:
		return renderRows(w, p, projectHeader, []core.Project{v}, p.projectRow)
	case []core.Workflow:
		return renderRows(w, p, workflowHeader, v, p.workflowRow)
	case []*core.Workflow:
		return renderRows(w, p, workflowHeader, deref(v), p.workflowRow)
	case core.Workflow:
		return renderRows(w, p, workflowHeader, []core.Workflow{v}, p.workflowRow)
	case []core.Comment:
		return renderRows(w, p, commentHeader, v, commentRow)
	case []*core.Comment:
		return renderRows(w, p, commentHeader, deref(v), commentRow)
	case core.Comment:
		return renderRows(w, p, commentHeader, []core.Comment{v}, commentRow)
	case []core.APIToken:
		return renderRows(w, p, tokenHeader, v, tokenRow)
	case []*core.APIToken:
		return renderRows(w, p, tokenHeader, deref(v), tokenRow)
	case core.APIToken:
		return renderRows(w, p, tokenHeader, []core.APIToken{v}, tokenRow)
	case []core.AuditEntry:
		return renderRows(w, p, auditHeader, v, auditRow)
	case []*core.AuditEntry:
		return renderRows(w, p, auditHeader, deref(v), auditRow)
	case core.AuditEntry:
		return renderRows(w, p, auditHeader, []core.AuditEntry{v}, auditRow)
	case []core.WebhookEndpoint:
		return renderRows(w, p, endpointHeader, v, endpointRow)
	case []*core.WebhookEndpoint:
		return renderRows(w, p, endpointHeader, deref(v), endpointRow)
	case core.WebhookEndpoint:
		return renderRows(w, p, endpointHeader, []core.WebhookEndpoint{v}, endpointRow)
	case []core.Tenant:
		return renderRows(w, p, tenantHeader, v, tenantRow)
	case []*core.Tenant:
		return renderRows(w, p, tenantHeader, deref(v), tenantRow)
	case []core.User:
		return renderRows(w, p, userHeader, v, userRow)
	case core.User:
		return renderRows(w, p, userHeader, []core.User{v}, userRow)
	case *core.User:
		return renderRows(w, p, userHeader, []core.User{*v}, userRow)
	case core.IssuedToken:
		return renderRows(w, p, issuedHeader, []core.IssuedToken{v}, issuedRow)
	case *core.IssuedToken:
		return renderRows(w, p, issuedHeader, []core.IssuedToken{*v}, issuedRow)
	case core.Claim:
		return renderRows(w, p, claimHeader, []core.Claim{v}, p.claimRow)
	case *core.Claim:
		return renderRows(w, p, claimHeader, []core.Claim{*v}, p.claimRow)
	case []core.FieldDef:
		return renderRows(w, p, fieldHeader, v, fieldRow)
	case core.FieldDef:
		return renderRows(w, p, fieldHeader, []core.FieldDef{v}, fieldRow)
	case *core.FieldDef:
		return renderRows(w, p, fieldHeader, []core.FieldDef{*v}, fieldRow)
	case []core.Dependency:
		return renderRows(w, p, depHeader, v, depRow)
	case []core.WebhookDelivery:
		return renderRows(w, p, deliveryHeader, v, deliveryRow)
	case []core.Tag:
		return renderRows(w, p, tagHeader, v, tagRow)
	case core.Tag:
		return renderRows(w, p, tagHeader, []core.Tag{v}, tagRow)
	case []core.SyncSource:
		return renderRows(w, p, syncHeader, v, syncRow)
	case core.Tenant:
		return renderRows(w, p, tenantHeader, []core.Tenant{v}, tenantRow)
	}
	return renderReflected(w, p, data)
}

var (
	taskHeader     = table.Row{"REF", "TITLE", "STATUS", "PRIORITY", "ASSIGNEE", "TAGS", "DUE", "UPDATED", "BLOCKED"}
	projectHeader  = table.Row{"KEY", "NAME", "WORKFLOW", "COLOR", "ICON", "ARCHIVED", "CREATED", "UPDATED"}
	workflowHeader = table.Row{"KEY", "NAME", "INITIAL", "STATES", "TRANSITIONS", "BUILTIN", "UPDATED"}
	commentHeader  = table.Row{"ID", "TASK", "AUTHOR", "BODY", "CREATED"}
	tokenHeader    = table.Row{"ID", "NAME", "ACTOR", "SCOPES", "PROJECT", "CREATED", "EXPIRES", "LAST USED", "REVOKED"}
	auditHeader    = table.Row{"SEQ", "ACTION", "SUBJECT", "ACTOR", "SOURCE", "OCCURRED"}
	endpointHeader = table.Row{"ID", "URL", "EVENTS", "ACTIVE", "CREATED"}
	tenantHeader   = table.Row{"ID", "KEY", "NAME", "CREATED", "DELETED"}
	userHeader     = table.Row{"ID", "EMAIL", "NAME", "CREATED", "DISABLED"}
	issuedHeader   = table.Row{"ID", "NAME", "SCOPES", "TOKEN", "EXPIRES"}
	claimHeader    = table.Row{"REF", "TITLE", "STATUS", "LEASE EXPIRES", "TOKEN"}
	fieldHeader    = table.Row{"KEY", "LABEL", "TYPE", "REQUIRED", "INDEXED", "OPTIONS"}
	depHeader      = table.Row{"TASK", "DEPENDS ON", "CREATED"}
	deliveryHeader = table.Row{"ID", "ENDPOINT", "EVENT", "STATUS", "ATTEMPTS", "NEXT ATTEMPT", "CODE"}
	tagHeader      = table.Row{"ID", "NAME", "PROJECT", "COLOR"}
	syncHeader     = table.Row{"ID", "SYSTEM", "NAME", "CURSOR", "LAST RUN", "LAST STATUS"}
)

func userRow(u core.User) table.Row {
	return table.Row{u.ID, u.Email, truncate(u.DisplayName, 30),
		FormatCompact(u.CreatedAt), FormatCompactPtr(u.DisabledAt)}
}

func issuedRow(t core.IssuedToken) table.Row {
	return table.Row{t.ID, t.Name, joinScopes(t.Scopes), t.Token, FormatCompactPtr(t.ExpiresAt)}
}

func (p Painter) claimRow(c core.Claim) table.Row {
	var ref, title, status string
	if c.Task != nil {
		ref, title, status = p.Ref(c.Task.Ref), truncate(c.Task.Title, 40), p.Status(c.Task.Status)
	}
	return table.Row{ref, title, status, FormatCompact(c.LeaseExpiresAt), c.LeaseToken}
}

func fieldRow(f core.FieldDef) table.Row {
	return table.Row{f.Key, f.Label, string(f.Type), yesNo(f.Required), yesNo(f.Indexed),
		truncate(strings.Join(f.EnumOptions, ", "), 40)}
}

func depRow(d core.Dependency) table.Row {
	return table.Row{d.TaskID, d.DependsOn, FormatCompact(d.CreatedAt)}
}

func deliveryRow(d core.WebhookDelivery) table.Row {
	code := ""
	if d.LastStatusCode != 0 {
		code = strconv.Itoa(d.LastStatusCode)
	}
	return table.Row{d.ID, d.EndpointID, strconv.FormatInt(d.EventSeq, 10), string(d.Status),
		strconv.Itoa(d.Attempts), FormatCompact(d.NextAttemptAt), code}
}

func tagRow(t core.Tag) table.Row {
	return table.Row{t.ID, t.Name, t.ProjectID, t.Color}
}

func syncRow(s core.SyncSource) table.Row {
	return table.Row{s.ID, s.System, s.Name, truncate(s.Cursor, 24),
		FormatCompactPtr(s.LastRunAt), s.LastStatus}
}

func joinScopes(scopes []core.Scope) string {
	out := make([]string, len(scopes))
	for i, s := range scopes {
		out[i] = string(s)
	}
	return truncate(strings.Join(out, ", "), 40)
}

func (p Painter) taskRow(t core.Task) table.Row {
	return table.Row{
		p.Ref(t.Ref),
		truncate(t.Title, 60),
		p.Status(t.Status),
		p.Priority(t.Priority, priorityLabel(t.Priority)),
		t.AssigneeActorID,
		strings.Join(t.Tags, ", "),
		FormatCompactPtr(t.DueAt),
		FormatCompact(t.UpdatedAt),
		p.Blocked(t.Blocked, yesNo(t.Blocked)),
	}
}

func (p Painter) projectRow(project core.Project) table.Row {
	return table.Row{
		project.Key, truncate(project.Name, 40), project.WorkflowID,
		p.Swatch(project.Color), project.Icon,
		yesNo(project.Archived()), FormatCompact(project.CreatedAt), FormatCompact(project.UpdatedAt),
	}
}

func (p Painter) workflowRow(wf core.Workflow) table.Row {
	initial := wf.Definition.Initial
	if state, ok := wf.Definition.State(initial); ok {
		initial = p.StatusIn(state.Category, initial)
	}
	return table.Row{
		wf.Key, truncate(wf.Name, 40), initial,
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

func renderTaskPage(w io.Writer, p Painter, page core.TaskPage) error {
	if err := renderRows(w, p, taskHeader, page.Tasks, p.taskRow); err != nil {
		return err
	}
	if page.NextCursor == "" {
		return nil
	}
	if _, err := fmt.Fprintf(w, "%s %s\n", p.Muted("next cursor:"), page.NextCursor); err != nil {
		return fmt.Errorf("writing cursor: %w", err)
	}
	return nil
}

func renderRows[T any](w io.Writer, p Painter, header table.Row, items []T, row func(T) table.Row) error {
	tbl := newWriter()
	tbl.AppendHeader(paintHeader(p, header))
	for _, item := range items {
		tbl.AppendRow(row(item))
	}
	return emit(w, p, tbl)
}

func renderReflected(w io.Writer, p Painter, data any) error {
	rv := reflect.ValueOf(data)
	if rv.Kind() != reflect.Slice {
		if rv.Kind() == reflect.Struct {
			return renderStructs(w, p, reflect.Append(reflect.MakeSlice(reflect.SliceOf(rv.Type()), 0, 1), rv))
		}
		_, err := fmt.Fprintf(w, "%v\n", data)
		return err
	}
	rv = concreteSlice(rv)
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
	return renderStructs(w, p, rv)
}

func renderStructs(w io.Writer, p Painter, rv reflect.Value) error {
	elem := rv.Type().Elem()
	if elem.Kind() == reflect.Pointer {
		elem = elem.Elem()
	}
	tbl := newWriter()
	header := make(table.Row, 0, elem.NumField())
	for i := 0; i < elem.NumField(); i++ {
		if skipField(elem.Field(i)) {
			continue
		}
		header = append(header, elem.Field(i).Name)
	}
	tbl.AppendHeader(paintHeader(p, header))
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
			if skipField(elem.Field(f)) {
				continue
			}
			row = append(row, fmt.Sprintf("%v", item.Field(f).Interface()))
		}
		tbl.AppendRow(row)
	}
	return emit(w, p, tbl)
}

func newWriter() table.Writer {
	tbl := table.NewWriter()
	style := table.StyleLight
	// go-pretty decides its own colours from the real process environment,
	// which ignores the mode this invocation resolved, so every style is
	// applied by the Painter instead. go-pretty measures cell widths with
	// escape codes stripped, which is what keeps a coloured table's columns
	// identical to a plain one's.
	style.Color = table.ColorOptions{}
	tbl.SetStyle(style)
	return tbl
}

// emit writes a rendered table, dimming its rules when colour is on.
func emit(w io.Writer, p Painter, tbl table.Writer) error {
	out := tbl.Render()
	if p.Enabled() {
		out = dimRules(p, out)
	}
	if _, err := fmt.Fprintln(w, out); err != nil {
		return fmt.Errorf("writing table: %w", err)
	}
	return nil
}

// boxRun matches a run of box-drawing characters. The rules are dimmed after
// rendering rather than through the style, because go-pretty builds a
// horizontal rule by repeating its character and counts raw runes while doing
// it, which an escape sequence would throw off.
var boxRun = regexp.MustCompile(`[\x{2500}-\x{257F}]+`)

// dimRules mutes every rule so the data is what the eye lands on.
func dimRules(p Painter, rendered string) string {
	return boxRun.ReplaceAllStringFunc(rendered, p.Muted)
}

// paintHeader styles every header cell, leaving non-string cells alone.
func paintHeader(p Painter, header table.Row) table.Row {
	if !p.Enabled() {
		return header
	}
	out := make(table.Row, len(header))
	for i, cell := range header {
		if s, ok := cell.(string); ok {
			out[i] = p.Header(s)
			continue
		}
		out[i] = cell
	}
	return out
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

// skipField reports whether the reflection fallback must leave a field out.
// A field marked json:"-" is excluded from serialisation deliberately, which is
// how secrets are kept out of output; the fallback has to honour that too.
func skipField(f reflect.StructField) bool {
	if f.PkgPath != "" {
		return true
	}
	return strings.HasPrefix(f.Tag.Get("json"), "-") || strings.HasPrefix(f.Tag.Get("yaml"), "-")
}

// concreteSlice retypes a slice of interfaces to the concrete type its elements
// share, so a buffered stream renders real columns instead of falling through
// to a whole-struct %v that would ignore field tags.
func concreteSlice(rv reflect.Value) reflect.Value {
	if rv.Type().Elem().Kind() != reflect.Interface || rv.Len() == 0 {
		return rv
	}
	first := rv.Index(0).Elem()
	if !first.IsValid() {
		return rv
	}
	typed := reflect.MakeSlice(reflect.SliceOf(first.Type()), 0, rv.Len())
	for i := range rv.Len() {
		e := rv.Index(i).Elem()
		if !e.IsValid() || e.Type() != first.Type() {
			return rv
		}
		typed = reflect.Append(typed, e)
	}
	return typed
}
