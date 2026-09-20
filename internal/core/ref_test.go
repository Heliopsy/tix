package core

import "testing"

func TestParseTaskRefHumanForm(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		wantKey string
		wantSeq int64
	}{
		{"simple", "infra-42", "infra", 42},
		{"single digit", "web-1", "web", 1},
		{"large seq", "ops-999999", "ops", 999999},
		{"hyphenated project key splits at last hyphen", "web-api-7", "web-api", 7},
		{"doubly hyphenated key", "a-b-c-12", "a-b-c", 12},
		{"underscore in key", "my_proj-3", "my_proj", 3},
		{"key is lowercased", "INFRA-42", "infra", 42},
		{"surrounding space trimmed", "  infra-42  ", "infra", 42},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseTaskRef(tt.in)
			if err != nil {
				t.Fatalf("ParseTaskRef(%q) error = %v", tt.in, err)
			}
			if got.IsID() {
				t.Fatalf("ParseTaskRef(%q) parsed as an identifier, want human form", tt.in)
			}
			if got.ProjectKey != tt.wantKey || got.Seq != tt.wantSeq {
				t.Errorf("ParseTaskRef(%q) = {%q, %d}, want {%q, %d}",
					tt.in, got.ProjectKey, got.Seq, tt.wantKey, tt.wantSeq)
			}
			if !got.Valid() {
				t.Error("parsed ref should be valid")
			}
		})
	}
}

func TestParseTaskRefIDForm(t *testing.T) {
	for _, in := range []string{
		"01JBXR8GTM4K2VQZ", // typical generated identifier
		"abcdefgh",         // minimum length
		"system01",
	} {
		got, err := ParseTaskRef(in)
		if err != nil {
			t.Fatalf("ParseTaskRef(%q) error = %v", in, err)
		}
		if !got.IsID() {
			t.Errorf("ParseTaskRef(%q) should parse as an identifier", in)
		}
		if got.ID != in {
			t.Errorf("ParseTaskRef(%q).ID = %q", in, got.ID)
		}
	}
}

func TestParseTaskRefRejects(t *testing.T) {
	tests := []struct {
		name string
		in   string
	}{
		{"empty", ""},
		{"only space", "   "},
		{"seq zero", "infra-0"},
		{"negative seq", "infra--5"},
		{"leading hyphen", "-42"},
		{"trailing hyphen", "infra-"},
		{"project key starting with digit", "9infra-4"},
		{"too short for an identifier", "abc"},
		{"path traversal", "../../etc/passwd"},
		{"shell glob", "infra-*"},
		{"url", "https://example.com/t/1"},
		{"sql-ish", "infra-1; DROP TABLE tasks"},
		{"space inside", "in fra-1"},
		{"non-numeric suffix reads as id but is invalid", "infra-abc!"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, err := ParseTaskRef(tt.in); err == nil {
				t.Errorf("ParseTaskRef(%q) = %+v, want an error", tt.in, got)
			} else if !IsKind(err, KindInvalid) {
				t.Errorf("ParseTaskRef(%q) error kind = %q, want invalid", tt.in, KindOf(err))
			}
		})
	}
}

func TestTaskRefRoundTrip(t *testing.T) {
	for _, in := range []string{"infra-42", "web-api-7", "01JBXR8GTM4K2VQZ"} {
		ref, err := ParseTaskRef(in)
		if err != nil {
			t.Fatalf("ParseTaskRef(%q) error = %v", in, err)
		}
		again, err := ParseTaskRef(ref.String())
		if err != nil {
			t.Fatalf("reparsing %q error = %v", ref.String(), err)
		}
		if again != ref {
			t.Errorf("round trip of %q = %+v, want %+v", in, again, ref)
		}
	}
}

func TestTaskRefString(t *testing.T) {
	tests := []struct {
		ref  TaskRef
		want string
	}{
		{TaskRef{ID: "01JBXR8GTM4K2VQZ"}, "01JBXR8GTM4K2VQZ"},
		{TaskRef{ProjectKey: "infra", Seq: 42}, "infra-42"},
		{TaskRef{}, ""},
	}
	for _, tt := range tests {
		if got := tt.ref.String(); got != tt.want {
			t.Errorf("%+v.String() = %q, want %q", tt.ref, got, tt.want)
		}
	}
}

func TestTaskRefValid(t *testing.T) {
	tests := []struct {
		ref  TaskRef
		want bool
	}{
		{TaskRef{ID: "01JBXR8GTM4K2VQZ"}, true},
		{TaskRef{ProjectKey: "infra", Seq: 1}, true},
		{TaskRef{}, false},
		{TaskRef{ProjectKey: "infra"}, false},
		{TaskRef{Seq: 4}, false},
	}
	for _, tt := range tests {
		if got := tt.ref.Valid(); got != tt.want {
			t.Errorf("%+v.Valid() = %v, want %v", tt.ref, got, tt.want)
		}
	}
}

func TestMustParseTaskRef(t *testing.T) {
	if got := MustParseTaskRef("infra-42"); got.Seq != 42 {
		t.Errorf("MustParseTaskRef() = %+v", got)
	}
	defer func() {
		if recover() == nil {
			t.Error("MustParseTaskRef should panic on an invalid reference")
		}
	}()
	MustParseTaskRef("")
}

func TestValidateProjectKey(t *testing.T) {
	for _, ok := range []string{"infra", "web-api", "my_proj", "a", "A1"} {
		if err := ValidateProjectKey(ok); err != nil {
			t.Errorf("ValidateProjectKey(%q) = %v, want nil", ok, err)
		}
	}
	for _, bad := range []string{"", "9infra", "-infra", "in fra", "pro/ject", "ünicode", "infra-", "infra_"} {
		if err := ValidateProjectKey(bad); err == nil {
			t.Errorf("ValidateProjectKey(%q) = nil, want an error", bad)
		}
	}
	long := make([]byte, 65)
	for i := range long {
		long[i] = 'a'
	}
	if err := ValidateProjectKey(string(long)); err == nil {
		t.Error("an over-long project key must be rejected")
	}
}
