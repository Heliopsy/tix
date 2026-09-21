package sql

import (
	stdsql "database/sql"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
)

func TestTimeTextIsLexicographicallyOrdered(t *testing.T) {
	base := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	cases := []time.Time{
		base,
		base.Add(time.Nanosecond),
		base.Add(time.Millisecond),
		base.Add(time.Second),
		base.Add(24 * time.Hour),
	}
	for i := 1; i < len(cases); i++ {
		prev, next := TimeText(cases[i-1]), TimeText(cases[i])
		if prev >= next {
			t.Fatalf("%q is not lexicographically before %q", prev, next)
		}
	}
	offset := time.FixedZone("plus", 3600)
	if TimeText(base.In(offset)) != TimeText(base) {
		t.Fatal("TimeText must normalize to UTC")
	}
}

func TestNullTimeText(t *testing.T) {
	if got := NullTimeText(nil); got != nil {
		t.Fatalf("nil time = %v, want nil", got)
	}
	var zero time.Time
	if got := NullTimeText(&zero); got != nil {
		t.Fatalf("zero time = %v, want nil", got)
	}
	at := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	if got := NullTimeText(&at); got != TimeText(at) {
		t.Fatalf("time = %v, want %q", got, TimeText(at))
	}
}

func TestParseTime(t *testing.T) {
	at := time.Date(2026, 1, 2, 3, 4, 5, 123456789, time.UTC)
	cases := []struct {
		name  string
		input string
		want  time.Time
		fails bool
	}{
		{name: "empty", input: ""},
		{name: "storage layout", input: TimeText(at), want: at},
		{name: "rfc3339 nano", input: at.Format(time.RFC3339Nano), want: at},
		{name: "rfc3339", input: "2026-01-02T03:04:05Z", want: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)},
		{name: "garbage", input: "not a time", fails: true},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseTime(tc.input)
			if tc.fails {
				if !core.IsKind(err, core.KindInternal) {
					t.Fatalf("error = %v, want internal", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseTime: %v", err)
			}
			if !got.Equal(tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestScanTimeAndScanNullTime(t *testing.T) {
	at := time.Date(2026, 5, 6, 7, 8, 9, 0, time.UTC)

	got, err := ScanTime(stdsql.NullString{})
	if err != nil || !got.IsZero() {
		t.Fatalf("ScanTime(null) = %v, %v", got, err)
	}
	got, err = ScanTime(stdsql.NullString{String: TimeText(at), Valid: true})
	if err != nil || !got.Equal(at) {
		t.Fatalf("ScanTime = %v, %v", got, err)
	}
	if _, err := ScanTime(stdsql.NullString{String: "bad", Valid: true}); err == nil {
		t.Fatal("ScanTime should reject a malformed timestamp")
	}

	if p, err := ScanNullTime(stdsql.NullString{}); err != nil || p != nil {
		t.Fatalf("ScanNullTime(null) = %v, %v", p, err)
	}
	if p, err := ScanNullTime(stdsql.NullString{String: "", Valid: true}); err != nil || p != nil {
		t.Fatalf("ScanNullTime(empty) = %v, %v", p, err)
	}
	p, err := ScanNullTime(stdsql.NullString{String: TimeText(at), Valid: true})
	if err != nil || p == nil || !p.Equal(at) {
		t.Fatalf("ScanNullTime = %v, %v", p, err)
	}
	if _, err := ScanNullTime(stdsql.NullString{String: "bad", Valid: true}); err == nil {
		t.Fatal("ScanNullTime should reject a malformed timestamp")
	}
}

func TestTextAndNullText(t *testing.T) {
	if Text(stdsql.NullString{}) != "" {
		t.Fatal("Text(null) should be empty")
	}
	if Text(stdsql.NullString{String: "x", Valid: true}) != "x" {
		t.Fatal("Text lost the value")
	}
	if NullText("") != nil {
		t.Fatal("NullText(\"\") should be nil")
	}
	if NullText("x") != "x" {
		t.Fatal("NullText lost the value")
	}
}

func TestBoolHelpers(t *testing.T) {
	if !Bool(1) || Bool(0) {
		t.Fatal("Bool is wrong")
	}
	if BoolInt(true) != 1 || BoolInt(false) != 0 {
		t.Fatal("BoolInt is wrong")
	}
}

func TestJSONHelpers(t *testing.T) {
	if got, err := JSONText(nil, "{}"); err != nil || got != "{}" {
		t.Fatalf("JSONText(nil) = %q, %v", got, err)
	}
	if got, err := JSONText(map[string]any(nil), "{}"); err != nil || got != "{}" {
		t.Fatalf("JSONText(nil map) = %q, %v", got, err)
	}
	got, err := JSONText([]string{"a"}, "[]")
	if err != nil || got != `["a"]` {
		t.Fatalf("JSONText = %q, %v", got, err)
	}
	if _, err := JSONText(make(chan int), "{}"); err == nil {
		t.Fatal("JSONText should reject an unmarshalable value")
	}

	var out []string
	if err := ParseJSON("", &out); err != nil || out != nil {
		t.Fatalf("ParseJSON(empty) = %v, %v", out, err)
	}
	if err := ParseJSON("null", &out); err != nil || out != nil {
		t.Fatalf("ParseJSON(null) = %v, %v", out, err)
	}
	if err := ParseJSON(`["a","b"]`, &out); err != nil || len(out) != 2 {
		t.Fatalf("ParseJSON = %v, %v", out, err)
	}
	if err := ParseJSON("{", &out); err == nil {
		t.Fatal("ParseJSON should reject malformed json")
	}
}

func TestDependencyPathQueryShape(t *testing.T) {
	if _, _, err := DependencyPathQuery(SQLite, core.TenantScope{}, "a", "b"); !core.IsKind(err, core.KindInvalid) {
		t.Fatalf("unscoped = %v, want invalid", err)
	}
	scope := core.TenantScope{TenantID: "t1"}
	q, args, err := DependencyPathQuery(SQLite, scope, "a", "b")
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	if len(args) != 4 {
		t.Fatalf("args = %d, want 4", len(args))
	}
	if q[:17] != "WITH RECURSIVE re" {
		t.Fatalf("query = %q", q)
	}
	pg, _, err := DependencyPathQuery(Postgres, scope, "a", "b")
	if err != nil {
		t.Fatalf("postgres: %v", err)
	}
	if pg == q {
		t.Fatal("postgres placeholders were not rewritten")
	}
}
