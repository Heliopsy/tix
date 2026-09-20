package core

import (
	"strings"
	"testing"
)

func TestCreateTaskInputValidate(t *testing.T) {
	tests := []struct {
		name    string
		in      CreateTaskInput
		wantErr bool
	}{
		{"title only is enough", CreateTaskInput{Title: "buy milk"}, false},
		{"with priority", CreateTaskInput{Title: "x", Priority: PriorityHigh}, false},
		{"zero priority means unset", CreateTaskInput{Title: "x", Priority: 0}, false},
		{"missing title", CreateTaskInput{}, true},
		{"out of range priority", CreateTaskInput{Title: "x", Priority: 99}, true},
		{"over-long title", CreateTaskInput{Title: strings.Repeat("a", MaxTitleLength+1)}, true},
		{"title at the limit", CreateTaskInput{Title: strings.Repeat("a", MaxTitleLength)}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.in.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestWorkflowInputValidate(t *testing.T) {
	good := WorkflowDefinition{
		Initial: "todo",
		States: []State{
			{Key: "todo"},
			{Key: "done", Terminal: true},
		},
		Transitions: []Transition{{From: "todo", To: "done"}},
	}

	if err := (WorkflowInput{Key: "default", Definition: good}).Validate(); err != nil {
		t.Fatalf("a valid workflow was rejected: %v", err)
	}

	tests := []struct {
		name string
		in   WorkflowInput
	}{
		{"missing key", WorkflowInput{Definition: good}},
		{"no states", WorkflowInput{Key: "k", Definition: WorkflowDefinition{Initial: "todo"}}},
		{
			"no initial state",
			WorkflowInput{Key: "k", Definition: WorkflowDefinition{
				States: []State{{Key: "todo"}, {Key: "done", Terminal: true}},
			}},
		},
		{
			"initial state not defined",
			WorkflowInput{Key: "k", Definition: WorkflowDefinition{
				Initial: "nope",
				States:  []State{{Key: "todo"}, {Key: "done", Terminal: true}},
			}},
		},
		{
			"state without a key",
			WorkflowInput{Key: "k", Definition: WorkflowDefinition{
				Initial: "todo",
				States:  []State{{Key: "todo"}, {Key: ""}, {Key: "done", Terminal: true}},
			}},
		},
		{
			"duplicate state",
			WorkflowInput{Key: "k", Definition: WorkflowDefinition{
				Initial: "todo",
				States:  []State{{Key: "todo"}, {Key: "todo"}, {Key: "done", Terminal: true}},
			}},
		},
		{
			"transition to an undefined state",
			WorkflowInput{Key: "k", Definition: WorkflowDefinition{
				Initial:     "todo",
				States:      []State{{Key: "todo"}, {Key: "done", Terminal: true}},
				Transitions: []Transition{{From: "todo", To: "nowhere"}},
			}},
		},
		{
			"transition from an undefined state",
			WorkflowInput{Key: "k", Definition: WorkflowDefinition{
				Initial:     "todo",
				States:      []State{{Key: "todo"}, {Key: "done", Terminal: true}},
				Transitions: []Transition{{From: "ghost", To: "done"}},
			}},
		},
		{
			"self transition",
			WorkflowInput{Key: "k", Definition: WorkflowDefinition{
				Initial:     "todo",
				States:      []State{{Key: "todo"}, {Key: "done", Terminal: true}},
				Transitions: []Transition{{From: "todo", To: "todo"}},
			}},
		},
		{
			"revert target undefined",
			WorkflowInput{Key: "k", Definition: WorkflowDefinition{
				Initial: "todo",
				States:  []State{{Key: "todo"}, {Key: "done", Terminal: true, RevertTo: "ghost"}},
			}},
		},
		{
			"no terminal state",
			WorkflowInput{Key: "k", Definition: WorkflowDefinition{
				Initial: "todo",
				States:  []State{{Key: "todo"}, {Key: "doing"}},
			}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.in.Validate(); err == nil {
				t.Error("expected an error")
			}
		})
	}
}

func TestFieldDefInputValidate(t *testing.T) {
	tests := []struct {
		name    string
		in      FieldDefInput
		wantErr bool
	}{
		{"string field", FieldDefInput{Key: "team", Type: FieldString}, false},
		{"enum with options", FieldDefInput{Key: "size", Type: FieldEnum, EnumOptions: []string{"s", "m"}}, false},
		{"missing key", FieldDefInput{Type: FieldString}, true},
		{"unknown type", FieldDefInput{Key: "k", Type: "blob"}, true},
		{"enum without options", FieldDefInput{Key: "size", Type: FieldEnum}, true},
		{"options on a non-enum", FieldDefInput{Key: "k", Type: FieldString, EnumOptions: []string{"a"}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.in.Validate(); (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestImportInputRequiresExplicitMode(t *testing.T) {
	if err := (ImportInput{}).Validate(); err == nil {
		t.Error("an import with no mode must be rejected")
	}
	for _, m := range []ImportMode{ImportMerge, ImportReplace} {
		if err := (ImportInput{Mode: m}).Validate(); err != nil {
			t.Errorf("mode %q should be accepted, got %v", m, err)
		}
	}
	if err := (ImportInput{Mode: "wipe"}).Validate(); err == nil {
		t.Error("an unknown mode must be rejected")
	}
}

func TestArtifactInputValidate(t *testing.T) {
	if err := (ArtifactInput{Kind: ArtifactResult}).Validate(); err != nil {
		t.Errorf("a result artifact should be accepted, got %v", err)
	}
	if err := (ArtifactInput{}).Validate(); err == nil {
		t.Error("an artifact with no kind must be rejected")
	}
	big := ArtifactInput{Kind: ArtifactFile, Blob: make([]byte, MaxBlobSize+1)}
	if err := big.Validate(); err == nil {
		t.Error("an over-large inline blob must be rejected")
	}
	atLimit := ArtifactInput{Kind: ArtifactFile, Blob: make([]byte, MaxBlobSize)}
	if err := atLimit.Validate(); err != nil {
		t.Errorf("a blob at the limit should be accepted, got %v", err)
	}
}

func TestCreateUserInputValidate(t *testing.T) {
	tests := []struct {
		name    string
		in      CreateUserInput
		wantErr bool
	}{
		{"email only", CreateUserInput{Email: "a@example.com"}, false},
		{"with a long enough password", CreateUserInput{Email: "a@example.com", Password: strings.Repeat("x", MinPasswordLength)}, false},
		{"with a role", CreateUserInput{Email: "a@example.com", Role: RoleMember}, false},
		{"missing email", CreateUserInput{}, true},
		{"short password", CreateUserInput{Email: "a@example.com", Password: "short"}, true},
		{"unknown role", CreateUserInput{Email: "a@example.com", Role: "owner"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.in.Validate(); (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCreateTokenInputValidate(t *testing.T) {
	ok := CreateTokenInput{Name: "agent", Scopes: []Scope{ScopeTaskClaim}}
	if err := ok.Validate(); err != nil {
		t.Errorf("Validate() = %v, want nil", err)
	}
	if err := (CreateTokenInput{Name: "agent"}).Validate(); err == nil {
		t.Error("a token with no scopes must be rejected")
	}
	if err := (CreateTokenInput{Scopes: []Scope{ScopeTaskRead}}).Validate(); err == nil {
		t.Error("a token with no name must be rejected")
	}
}

func TestSyncSourceInputValidate(t *testing.T) {
	for _, system := range []string{SystemGeneric, SystemJira, SystemOpenProject} {
		in := SyncSourceInput{System: system, Name: "main"}
		if err := in.Validate(); err != nil {
			t.Errorf("system %q should be accepted, got %v", system, err)
		}
	}
	if err := (SyncSourceInput{System: "trello", Name: "x"}).Validate(); err == nil {
		t.Error("an unsupported system must be rejected")
	}
	if err := (SyncSourceInput{System: SystemJira}).Validate(); err == nil {
		t.Error("a source with no name must be rejected")
	}
}

func TestCreateProjectInputValidate(t *testing.T) {
	if err := (CreateProjectInput{Key: "infra", Name: "Infrastructure"}).Validate(); err != nil {
		t.Errorf("Validate() = %v, want nil", err)
	}
	if err := (CreateProjectInput{Key: "9bad", Name: "x"}).Validate(); err == nil {
		t.Error("an invalid project key must be rejected")
	}
	if err := (CreateProjectInput{Key: "infra"}).Validate(); err == nil {
		t.Error("a project with no name must be rejected")
	}
}

func TestCreateTenantInputValidate(t *testing.T) {
	if err := (CreateTenantInput{Key: "acme", Name: "Acme"}).Validate(); err != nil {
		t.Errorf("Validate() = %v, want nil", err)
	}
	if err := (CreateTenantInput{Key: "-bad", Name: "x"}).Validate(); err == nil {
		t.Error("an invalid tenant key must be rejected")
	}
	if err := (CreateTenantInput{Key: "acme"}).Validate(); err == nil {
		t.Error("a tenant with no name must be rejected")
	}
}

func TestTransitionInputValidate(t *testing.T) {
	if err := (TransitionInput{To: "done"}).Validate(); err != nil {
		t.Errorf("Validate() = %v, want nil", err)
	}
	if err := (TransitionInput{}).Validate(); err == nil {
		t.Error("a transition with no target must be rejected")
	}
}

func TestWebhookInputValidate(t *testing.T) {
	if err := (WebhookInput{URL: "https://example.com/hook"}).Validate(); err != nil {
		t.Errorf("Validate() = %v, want nil", err)
	}
	if err := (WebhookInput{}).Validate(); err == nil {
		t.Error("a webhook with no url must be rejected")
	}
}

// UpdateTaskInput uses pointers so clearing a field is distinguishable from
func TestUpdateTaskInputDistinguishesUnsetFromZero(t *testing.T) {
	var unset UpdateTaskInput
	if unset.Title != nil {
		t.Error("an unset title must be nil")
	}

	empty := ""
	clear := UpdateTaskInput{Title: &empty}
	if clear.Title == nil || *clear.Title != "" {
		t.Error("an explicitly empty title must be distinguishable from unset")
	}
}
