package core

import (
	"encoding/json"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func testWorkflow() WorkflowDefinition {
	return WorkflowDefinition{
		Initial: "todo",
		States: []State{
			{Key: "todo", Label: "To do", Category: CategoryTodo},
			{Key: "doing", Label: "Doing", Category: CategoryInProgress, RevertOnLeaseExpiry: true, RevertTo: "todo"},
			{Key: "blocked", Label: "Blocked", Category: CategoryInProgress},
			{Key: "done", Label: "Done", Terminal: true, Category: CategoryDone},
			{Key: "cancelled", Label: "Cancelled", Terminal: true, Category: CategoryDone},
		},
		Transitions: []Transition{
			{From: "todo", To: "doing"},
			{From: "doing", To: "blocked"},
			{From: "blocked", To: "doing"},
			{From: "doing", To: "done"},
			{From: "todo", To: "cancelled", RequiresComment: true},
		},
	}
}

func TestWorkflowStateLookup(t *testing.T) {
	d := testWorkflow()

	s, ok := d.State("doing")
	if !ok || s.Label != "Doing" {
		t.Fatalf("State(doing) = %+v, %v", s, ok)
	}
	if _, ok := d.State("nope"); ok {
		t.Error("State() should report an unknown state as absent")
	}
	if !d.HasState("todo") || d.HasState("nope") {
		t.Error("HasState() disagrees with State()")
	}
}

func TestWorkflowTerminal(t *testing.T) {
	d := testWorkflow()

	if !d.IsTerminal("done") || !d.IsTerminal("cancelled") {
		t.Error("done and cancelled should be terminal")
	}
	if d.IsTerminal("doing") {
		t.Error("doing must not be terminal")
	}
	if d.IsTerminal("state-that-was-deleted") {
		t.Error("an unknown state must not be treated as terminal")
	}

	got := d.TerminalStates()
	if len(got) != 2 {
		t.Errorf("TerminalStates() = %v, want 2 entries", got)
	}
}

func TestWorkflowCanTransition(t *testing.T) {
	d := testWorkflow()

	if _, ok := d.CanTransition("todo", "doing"); !ok {
		t.Error("todo -> doing should be permitted")
	}
	if _, ok := d.CanTransition("todo", "done"); ok {
		t.Error("todo -> done is not defined and must be rejected")
	}
	if _, ok := d.CanTransition("done", "todo"); ok {
		t.Error("reverse transitions are not implied")
	}

	tr, ok := d.CanTransition("todo", "cancelled")
	if !ok || !tr.RequiresComment {
		t.Errorf("CanTransition(todo, cancelled) = %+v, %v; want RequiresComment", tr, ok)
	}
}

func TestTaskClaimedAtTime(t *testing.T) {
	now := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
	future := now.Add(time.Minute)
	past := now.Add(-time.Minute)

	tests := []struct {
		name string
		task Task
		want bool
	}{
		{"unclaimed", Task{}, false},
		{"live lease", Task{ClaimedByActorID: "a1", LeaseExpiresAt: &future}, true},
		{"expired lease is not a claim", Task{ClaimedByActorID: "a1", LeaseExpiresAt: &past}, false},
		{"holder with no expiry", Task{ClaimedByActorID: "a1"}, false},
		{"expiry with no holder", Task{LeaseExpiresAt: &future}, false},
		{"expiring exactly now", Task{ClaimedByActorID: "a1", LeaseExpiresAt: &now}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.task.ClaimedAtTime(now); got != tt.want {
				t.Errorf("ClaimedAtTime() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTaskDeleted(t *testing.T) {
	now := time.Now()
	if (Task{}).Deleted() {
		t.Error("a task with no deletion time is not deleted")
	}
	if !(Task{DeletedAt: &now}).Deleted() {
		t.Error("a task with a deletion time is deleted")
	}
}

func TestAPITokenActive(t *testing.T) {
	now := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
	future := now.Add(time.Hour)
	past := now.Add(-time.Hour)

	tests := []struct {
		name  string
		token APIToken
		want  bool
	}{
		{"no expiry", APIToken{}, true},
		{"not yet expired", APIToken{ExpiresAt: &future}, true},
		{"expired", APIToken{ExpiresAt: &past}, false},
		{"revoked", APIToken{RevokedAt: &past}, false},
		{"revoked beats a future expiry", APIToken{ExpiresAt: &future, RevokedAt: &past}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.token.Active(now); got != tt.want {
				t.Errorf("Active() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestWebhookSecretNeverMarshalled(t *testing.T) {
	e := WebhookEndpoint{ID: "w1", URL: "https://example.com", Secret: "super-secret-value"}
	b, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if contains(string(b), "super-secret-value") || contains(string(b), "Secret") {
		t.Errorf("webhook secret leaked into JSON: %s", b)
	}
}

func TestPriorityValid(t *testing.T) {
	for _, p := range []Priority{PriorityHighest, PriorityNormal, PriorityLowest} {
		if !p.Valid() {
			t.Errorf("%d should be valid", p)
		}
	}
	for _, p := range []Priority{0, -1, 6, 100} {
		if p.Valid() {
			t.Errorf("%d must be invalid", p)
		}
	}
	if PriorityHighest >= PriorityLowest {
		t.Error("PriorityHighest must be numerically lower than PriorityLowest")
	}
}

func TestFieldTypeValid(t *testing.T) {
	for _, ft := range []FieldType{
		FieldString, FieldText, FieldInt, FieldFloat, FieldBool,
		FieldDate, FieldDateTime, FieldEnum, FieldActor, FieldJSON,
	} {
		if !ft.Valid() {
			t.Errorf("%q should be valid", ft)
		}
	}
	if FieldType("blob").Valid() {
		t.Error("unknown field type must be invalid")
	}
}

func TestDomainVerified(t *testing.T) {
	now := time.Now()
	if (Domain{}).Verified() {
		t.Error("a domain with no verification time is unverified")
	}
	if !(Domain{VerifiedAt: &now}).Verified() {
		t.Error("a domain with a verification time is verified")
	}
}

func TestProjectArchived(t *testing.T) {
	now := time.Now()
	if (Project{}).Archived() {
		t.Error("a project with no archive time is not archived")
	}
	if !(Project{ArchivedAt: &now}).Archived() {
		t.Error("a project with an archive time is archived")
	}
}

func TestDefaultRetentionKeepsAuditLongest(t *testing.T) {
	r := DefaultRetention("t1")
	if r.AuditEntries <= r.Events {
		t.Errorf("audit retention %v must exceed event retention %v", r.AuditEntries, r.Events)
	}
	if r.TenantID != "t1" {
		t.Errorf("TenantID = %q, want t1", r.TenantID)
	}
	for name, d := range map[string]Duration{
		"events": r.Events, "audit": r.AuditEntries, "deliveries": r.WebhookDeliveries,
	} {
		if d <= 0 {
			t.Errorf("%s retention must be positive, got %v", name, d)
		}
	}
}

func TestDurationJSONRoundTrip(t *testing.T) {
	d := Duration(15 * time.Minute)

	b, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if string(b) != `"15m0s"` {
		t.Errorf("Marshal() = %s, want a duration string", b)
	}

	var got Duration
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if got != d {
		t.Errorf("round trip = %v, want %v", got, d)
	}
}

func TestDurationUnmarshalForms(t *testing.T) {
	tests := []struct {
		in      string
		want    time.Duration
		wantErr bool
	}{
		{`"15m"`, 15 * time.Minute, false},
		{`"2h30m"`, 2*time.Hour + 30*time.Minute, false},
		{`"0s"`, 0, false},
		{`900000000000`, 15 * time.Minute, false},
		{`"not a duration"`, 0, true},
		{`{}`, 0, true},
		{`"15"`, 0, true},
	}
	for _, tt := range tests {
		var got Duration
		err := json.Unmarshal([]byte(tt.in), &got)
		if tt.wantErr {
			if err == nil {
				t.Errorf("Unmarshal(%s) = nil error, want an error", tt.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("Unmarshal(%s) error = %v", tt.in, err)
			continue
		}
		if got.D() != tt.want {
			t.Errorf("Unmarshal(%s) = %v, want %v", tt.in, got.D(), tt.want)
		}
	}
}

func TestDurationString(t *testing.T) {
	if got := Duration(90 * time.Second).String(); got != "1m30s" {
		t.Errorf("String() = %q, want 1m30s", got)
	}
}

func TestDurationYAMLRoundTrip(t *testing.T) {
	type cfg struct {
		Lease Duration `yaml:"lease"`
	}

	out, err := yaml.Marshal(cfg{Lease: Duration(15 * time.Minute)})
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if !contains(string(out), "15m0s") {
		t.Errorf("Marshal() = %q, want a duration string", out)
	}

	var got cfg
	if err := yaml.Unmarshal(out, &got); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if got.Lease.D() != 15*time.Minute {
		t.Errorf("round trip = %v, want 15m", got.Lease.D())
	}
}

func TestDurationYAMLForms(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    time.Duration
		wantErr bool
	}{
		{"string form", "lease: 2h30m\n", 2*time.Hour + 30*time.Minute, false},
		{"quoted string", "lease: \"45s\"\n", 45 * time.Second, false},
		{"bare nanoseconds", "lease: 900000000000\n", 15 * time.Minute, false},
		{"unparseable", "lease: not-a-duration\n", 0, true},
		{"wrong shape", "lease: {a: 1}\n", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got struct {
				Lease Duration `yaml:"lease"`
			}
			err := yaml.Unmarshal([]byte(tt.in), &got)
			if tt.wantErr {
				if err == nil {
					t.Errorf("Unmarshal(%q) = nil error, want an error", tt.in)
				}
				return
			}
			if err != nil {
				t.Fatalf("Unmarshal(%q) error = %v", tt.in, err)
			}
			if got.Lease.D() != tt.want {
				t.Errorf("Unmarshal(%q) = %v, want %v", tt.in, got.Lease.D(), tt.want)
			}
		})
	}
}
