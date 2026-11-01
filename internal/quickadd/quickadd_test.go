// SPDX-License-Identifier: AGPL-3.0-or-later

package quickadd

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
)

// sunday is the present every parse below is answered at: Sunday 1 November
// 2026, mid-morning UTC.
var sunday = time.Date(2026, 11, 1, 10, 30, 0, 0, time.UTC)

// day is midnight UTC of a calendar day.
func day(y int, m time.Month, d int) *time.Time {
	t := time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	return &t
}

// want is what a line should parse to. Unset fields are expected unset.
type want struct {
	title    string
	priority core.Priority
	tags     []string
	assignee string
	due      *time.Time
	project  string
	parent   string
	status   string
}

func TestParse(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		line string
		want want
	}{
		{"plain title", "buy milk", want{title: "buy milk"}},
		{"the example line", "Fix login !high #auth @raj due:fri +web ^infra-3",
			want{title: "Fix login", priority: core.PriorityHigh, tags: []string{"auth"}, assignee: "raj",
				due: day(2026, 11, 6), project: "web", parent: "infra-3"}},
		{"status", "Fix login status:doing", want{title: "Fix login", status: "doing"}},
		{"long forms", "Fix login priority:low tag:ops assignee:lena project:infra parent:infra-9",
			want{title: "Fix login", priority: core.PriorityLow, tags: []string{"ops"}, assignee: "lena",
				project: "infra", parent: "infra-9"}},
		{"key is case-insensitive", "Fix DUE:tomorrow Project:Web",
			want{title: "Fix", due: day(2026, 11, 2), project: "web"}},
		{"tokens anywhere", "!lowest Fix #a the #b login",
			want{title: "Fix the login", priority: core.PriorityLowest, tags: []string{"a", "b"}}},
		{"repeated tag once", "x #a #a", want{title: "x", tags: []string{"a"}}},
		{"same priority twice is fine", "x !high !2", want{title: "x", priority: core.PriorityHigh}},

		{"priority highest", "x !highest", want{title: "x", priority: core.PriorityHighest}},
		{"priority normal", "x !normal", want{title: "x", priority: core.PriorityNormal}},
		{"priority low", "x !low", want{title: "x", priority: core.PriorityLow}},
		{"priority by number", "x !1", want{title: "x", priority: core.PriorityHighest}},
		{"priority five", "x !5", want{title: "x", priority: core.PriorityLowest}},
		{"priority name case", "x !HIGH", want{title: "x", priority: core.PriorityHigh}},
		{"priority out of range is text", "x !7", want{title: "x !7"}},
		{"priority signed number is text", "x !+3", want{title: "x !+3"}},
		{"priority two digits is text", "x !03", want{title: "x !03"}},
		{"unknown priority word is text", "read the !important memo", want{title: "read the !important memo"}},
		{"bare bang is text", "wow !", want{title: "wow !"}},
		{"trailing bang is text", "ship it!", want{title: "ship it!"}},

		{"Bulgarian title", "Поправи входа !high", want{title: "Поправи входа", priority: core.PriorityHigh}},
		{"Bulgarian tag", "Поправи входа #сигурност", want{title: "Поправи входа", tags: []string{"сигурност"}}},
		{"Bulgarian handle", "Обади се @иван", want{title: "Обади се", assignee: "иван"}},
		{"emoji title", "🚀 launch #release", want{title: "🚀 launch", tags: []string{"release"}}},
		{"CJK title", "修复登录 #認証", want{title: "修复登录", tags: []string{"認証"}}},
		{"tag with joiners", "x #front-end/css", want{title: "x", tags: []string{"front-end/css"}}},
		{"tag with digits", "x #v2", want{title: "x", tags: []string{"v2"}}},

		{"email stays", "mail a@b.c today", want{title: "mail a@b.c today"}},
		{"C sharp stays", "learn C# basics", want{title: "learn C# basics"}},
		{"F sharp and C sharp", "port C# to F#", want{title: "port C# to F#"}},
		{"issue number stays", "fix #42 now", want{title: "fix #42 now"}},
		{"hash alone stays", "item # 3", want{title: "item # 3"}},
		{"double hash stays", "## heading", want{title: "## heading"}},
		{"hash in URL stays", "see https://example.com/page#section", want{title: "see https://example.com/page#section"}},
		{"hash path in URL stays", "open https://x.io/#/settings", want{title: "open https://x.io/#/settings"}},
		{"URL key is not a key", "read http://x.io", want{title: "read http://x.io"}},
		{"tag with trailing dot stays", "about #news.", want{title: "about #news."}},
		{"tag with comma stays", "about #news,", want{title: "about #news,"}},
		{"plus one stays", "+1 on this", want{title: "+1 on this"}},
		{"plus alone stays", "a + b", want{title: "a + b"}},
		{"plus with bad key stays", "x +web-", want{title: "x +web-"}},
		{"at alone stays", "meet @ noon", want{title: "meet @ noon"}},
		{"handle with trailing dot stays", "ask @raj.", want{title: "ask @raj."}},
		{"handle with dot inside", "ask @raj.k", want{title: "ask", assignee: "raj.k"}},
		{"npm scope stays", "bump @types/node", want{title: "bump @types/node"}},
		{"caret alone stays", "2 ^ 3", want{title: "2 ^ 3"}},
		{"smiley stays", "nice ^_^", want{title: "nice ^_^"}},
		{"caret without number stays", "under ^infra", want{title: "under ^infra"}},
		{"unknown key stays", "ratio 16:9 and note:x", want{title: "ratio 16:9 and note:x"}},
		{"empty keyed value stays", "due: soon", want{title: "due: soon"}},
		{"time of day stays", "standup at 9:30", want{title: "standup at 9:30"}},

		{"backslash keeps a tag", `tag posts \#news`, want{title: "tag posts #news"}},
		{"backslash keeps a handle", `thank \@raj`, want{title: "thank @raj"}},
		{"backslash keeps a priority", `say \!high`, want{title: "say !high"}},
		{"backslash keeps a key", `explain \due:fri`, want{title: "explain due:fri"}},
		{"backslash keeps a project", `\+web`, want{title: "+web"}},
		{"backslash elsewhere stays", `clean C:\temp`, want{title: `clean C:\temp`}},
		{"lone backslash stays", `a \ b`, want{title: `a \ b`}},
		{"quotes keep tokens", `Rename "#main @ops" to trunk`, want{title: `Rename "#main @ops" to trunk`}},
		{"quoted single word", `explain "!high" #docs`, want{title: `explain "!high"`, tags: []string{"docs"}}},
		{"Bulgarian quotes", `Прочети „#новини @иван“ !low`, want{title: `Прочети „#новини @иван“`, priority: core.PriorityLow}},
		{"curly quotes", `say “due:fri” now`, want{title: `say “due:fri” now`}},
		{"guillemets", `lire «#info» #fr`, want{title: `lire «#info»`, tags: []string{"fr"}}},
		{"unclosed quote is text", `say "hi #there`, want{title: `say "hi`, tags: []string{"there"}}},
		{"apostrophe is text", `don't #panic`, want{title: "don't", tags: []string{"panic"}}},

		{"whitespace collapses", "  Fix \t login   #a  ", want{title: "Fix login", tags: []string{"a"}}},

		{"due today", "x due:today", want{title: "x", due: day(2026, 11, 1)}},
		{"due tomorrow", "x due:tomorrow", want{title: "x", due: day(2026, 11, 2)}},
		{"due weekday short", "x due:mon", want{title: "x", due: day(2026, 11, 2)}},
		{"due weekday full", "x due:Saturday", want{title: "x", due: day(2026, 11, 7)}},
		{"due same weekday is next week", "x due:sun", want{title: "x", due: day(2026, 11, 8)}},
		{"due days", "x due:+3d", want{title: "x", due: day(2026, 11, 4)}},
		{"due weeks", "x due:+2w", want{title: "x", due: day(2026, 11, 15)}},
		{"due zero days", "x due:+0d", want{title: "x", due: day(2026, 11, 1)}},
		{"due across a year", "x due:+61d", want{title: "x", due: day(2027, 1, 1)}},
		{"due date", "x due:2026-11-20", want{title: "x", due: day(2026, 11, 20)}},
		{"due timestamp", "x due:2026-11-20T15:04:05Z",
			want{title: "x", due: ptr(time.Date(2026, 11, 20, 15, 4, 5, 0, time.UTC))}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := Parse(tt.line, sunday)
			if err != nil {
				t.Fatalf("Parse(%q) refused: %v", tt.line, err)
			}
			check(t, tt.line, got, tt.want)
		})
	}
}

// check compares a result field by field so a failure names the field.
func check(t *testing.T, line string, got Result, w want) {
	t.Helper()
	if got.Title != w.title {
		t.Errorf("%q: title %q, want %q", line, got.Title, w.title)
	}
	if got.Priority != w.priority {
		t.Errorf("%q: priority %d, want %d", line, got.Priority, w.priority)
	}
	if !slices.Equal(got.Tags, w.tags) {
		t.Errorf("%q: tags %q, want %q", line, got.Tags, w.tags)
	}
	if got.Assignee != w.assignee {
		t.Errorf("%q: assignee %q, want %q", line, got.Assignee, w.assignee)
	}
	if (got.Due == nil) != (w.due == nil) || got.Due != nil && !got.Due.Equal(*w.due) {
		t.Errorf("%q: due %v, want %v", line, got.Due, w.due)
	}
	if got.Project != w.project {
		t.Errorf("%q: project %q, want %q", line, got.Project, w.project)
	}
	if got.Parent != w.parent {
		t.Errorf("%q: parent %q, want %q", line, got.Parent, w.parent)
	}
	if got.Status != w.status {
		t.Errorf("%q: status %q, want %q", line, got.Status, w.status)
	}
}

func ptr(t time.Time) *time.Time { return &t }

func TestParseRefuses(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, line, mention string
	}{
		{"empty line", "", "title is required"},
		{"blank line", "   ", "title is required"},
		{"only tokens", "#auth !high", "#auth"},
		{"two priorities", "x !high !low", "!low"},
		{"two assignees", "x @raj @lena", "@lena"},
		{"two projects", "x +web project:infra", "project:infra"},
		{"two due dates", "x due:fri due:mon", "due:mon"},
		{"two parents", "x ^infra-1 ^infra-2", "^infra-2"},
		{"two statuses", "x status:todo status:doing", "status:doing"},
		{"bad due word", "Ship it due:someday", "someday"},
		{"bad due offset unit", "x due:+3y", "+3y"},
		{"due offset too far", "x due:+99999d", "+99999d"},
		{"bad due date", "x due:2026-13-01", "2026-13-01"},
		{"bad keyed priority", "x priority:urgent", "urgent"},
		{"bad keyed project", "x project:1web", "1web"},
		{"bad keyed parent", "x parent:nope", "nope"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := Parse(tt.line, sunday)
			if err == nil {
				t.Fatalf("Parse(%q) accepted a line it should refuse", tt.line)
			}
			if !core.IsKind(err, core.KindInvalid) {
				t.Errorf("Parse(%q) refused as %v, want invalid input", tt.line, core.KindOf(err))
			}
			if !strings.Contains(err.Error(), tt.mention) {
				t.Errorf("Parse(%q) = %q, which does not name %q", tt.line, err, tt.mention)
			}
			if core.FieldOf(err) != "title" {
				t.Errorf("Parse(%q) attributed the refusal to %q, not to the line", tt.line, core.FieldOf(err))
			}
		})
	}
}

// The calendar a weekday is counted in is the present's own. Late on Saturday
// in Tokyo is still Saturday morning in UTC, and "tomorrow" there is Sunday.
func TestRelativeDatesFollowThePresentsZone(t *testing.T) {
	t.Parallel()
	tokyo := time.FixedZone("JST", 9*3600)
	late := time.Date(2026, 11, 7, 23, 0, 0, 0, tokyo)
	got, err := Parse("x due:tomorrow", late)
	if err != nil {
		t.Fatal(err)
	}
	if want := day(2026, 11, 8); !got.Due.Equal(*want) {
		t.Errorf("tomorrow from late Saturday in Tokyo is %v, want %v", got.Due, want)
	}
	utc, err := Parse("x due:tomorrow", late.UTC())
	if err != nil {
		t.Fatal(err)
	}
	if want := day(2026, 11, 8); !utc.Due.Equal(*want) {
		t.Errorf("tomorrow from Saturday 14:00 UTC is %v, want %v", utc.Due, want)
	}
	early := time.Date(2026, 11, 8, 1, 0, 0, 0, tokyo)
	got, err = Parse("x due:today", early)
	if err != nil {
		t.Fatal(err)
	}
	if want := day(2026, 11, 8); !got.Due.Equal(*want) {
		t.Errorf("today early Sunday in Tokyo is %v, want %v though UTC is still Saturday", got.Due, want)
	}
}

func TestTokensAreRecordedInOrder(t *testing.T) {
	t.Parallel()
	got, err := Parse("Fix !1 #auth @raj due:+3d +web ^infra-3 status:doing", sunday)
	if err != nil {
		t.Fatal(err)
	}
	want := []Token{
		{Kind: KindPriority, Raw: "!1", Value: "highest"},
		{Kind: KindTag, Raw: "#auth", Value: "auth"},
		{Kind: KindAssignee, Raw: "@raj", Value: "raj"},
		{Kind: KindDue, Raw: "due:+3d", Value: "2026-11-04"},
		{Kind: KindProject, Raw: "+web", Value: "web"},
		{Kind: KindParent, Raw: "^infra-3", Value: "infra-3"},
		{Kind: KindStatus, Raw: "status:doing", Value: "doing"},
	}
	if len(got.Tokens) != len(want) {
		t.Fatalf("tokens %+v, want %+v", got.Tokens, want)
	}
	for i, tok := range got.Tokens {
		if tok.Kind != want[i].Kind || tok.Raw != want[i].Raw || tok.Value != want[i].Value {
			t.Errorf("token %d is %+v, want %+v", i, tok, want[i])
		}
	}
}

func TestApply(t *testing.T) {
	t.Parallel()
	due := day(2026, 11, 20)
	other := day(2026, 12, 1)
	tests := []struct {
		name    string
		line    string
		in      core.CreateTaskInput
		want    core.CreateTaskInput
		refused string
	}{
		{"fills an empty input", "Fix !high #a @raj due:2026-11-20 +web ^web-1 status:doing", core.CreateTaskInput{},
			core.CreateTaskInput{Title: "Fix", Priority: core.PriorityHigh, Tags: []string{"a"}, AssigneeActorID: "raj",
				DueAt: due, ProjectRef: "web", ParentRef: "web-1", Status: "doing"}, ""},
		{"keeps the caller's other fields", "Fix", core.CreateTaskInput{Body: "notes", ProjectRef: "infra", Priority: core.PriorityLow},
			core.CreateTaskInput{Title: "Fix", Body: "notes", ProjectRef: "infra", Priority: core.PriorityLow}, ""},
		{"combines tags", "Fix #a #b", core.CreateTaskInput{Tags: []string{"b", "c"}},
			core.CreateTaskInput{Title: "Fix", Tags: []string{"b", "c", "a"}}, ""},
		{"agreeing values pass", "Fix !high +WEB due:2026-11-20", core.CreateTaskInput{Priority: core.PriorityHigh, ProjectRef: "web", DueAt: due},
			core.CreateTaskInput{Title: "Fix", Priority: core.PriorityHigh, ProjectRef: "web", DueAt: due}, ""},
		{"priority contradiction", "Fix !high", core.CreateTaskInput{Priority: core.PriorityLow}, core.CreateTaskInput{}, "priority"},
		{"project contradiction", "Fix +web", core.CreateTaskInput{ProjectRef: "infra"}, core.CreateTaskInput{}, "project_ref"},
		{"assignee contradiction", "Fix @raj", core.CreateTaskInput{AssigneeActorID: "lena"}, core.CreateTaskInput{}, "assignee_actor_id"},
		{"parent contradiction", "Fix ^web-1", core.CreateTaskInput{ParentRef: "web-2"}, core.CreateTaskInput{}, "parent_ref"},
		{"status contradiction", "Fix status:doing", core.CreateTaskInput{Status: "todo"}, core.CreateTaskInput{}, "status"},
		{"due contradiction", "Fix due:2026-11-20", core.CreateTaskInput{DueAt: other}, core.CreateTaskInput{}, "due_at"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r, err := Parse(tt.line, sunday)
			if err != nil {
				t.Fatal(err)
			}
			got, err := r.Apply(tt.in)
			if tt.refused != "" {
				if err == nil || core.FieldOf(err) != tt.refused || !core.IsKind(err, core.KindInvalid) {
					t.Fatalf("Apply = %v, want an invalid refusal on %s", err, tt.refused)
				}
				return
			}
			if err != nil {
				t.Fatalf("Apply refused: %v", err)
			}
			if got.Title != tt.want.Title || got.Body != tt.want.Body || got.Priority != tt.want.Priority ||
				!slices.Equal(got.Tags, tt.want.Tags) || got.AssigneeActorID != tt.want.AssigneeActorID ||
				got.ProjectRef != tt.want.ProjectRef || got.ParentRef != tt.want.ParentRef || got.Status != tt.want.Status ||
				(got.DueAt == nil) != (tt.want.DueAt == nil) || got.DueAt != nil && !got.DueAt.Equal(*tt.want.DueAt) {
				t.Errorf("Apply = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestFormatDue(t *testing.T) {
	t.Parallel()
	if got := FormatDue(*day(2026, 11, 20)); got != "2026-11-20" {
		t.Errorf("a day formats as %q", got)
	}
	if got := FormatDue(time.Date(2026, 11, 20, 9, 0, 0, 0, time.UTC)); got != "2026-11-20T09:00:00Z" {
		t.Errorf("an instant formats as %q", got)
	}
}

func TestSyntaxHintNamesEveryForm(t *testing.T) {
	t.Parallel()
	hint := SyntaxHint()
	for _, form := range []string{"!high", "#tag", "@handle", "due:", "+project", "^", "status:", `\`, "quotes"} {
		if !strings.Contains(hint, form) {
			t.Errorf("the hint %q does not mention %s", hint, form)
		}
	}
}

func TestPriorityNameCoversTheScale(t *testing.T) {
	t.Parallel()
	for p := core.PriorityHighest; p <= core.PriorityLowest; p++ {
		r, err := Parse("x !"+PriorityName(p), sunday)
		if err != nil || r.Priority != p {
			t.Errorf("!%s parses to %d (%v), want %d", PriorityName(p), r.Priority, err, p)
		}
	}
	if PriorityName(0) != "" {
		t.Errorf("an unset priority has the name %q", PriorityName(0))
	}
}
