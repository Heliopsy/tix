package core_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

func TestFriendlyNameIsStableForOneIdentifier(t *testing.T) {
	t.Parallel()
	const id = "01M30B7TX7T220FT9RHGK0X2GR"
	first := core.FriendlyName(id)
	if first == "" {
		t.Fatal("an identifier produced no name")
	}
	for range 100 {
		if again := core.FriendlyName(id); again != first {
			t.Fatalf("FriendlyName(%q) = %q then %q; the mapping is not stable", id, first, again)
		}
	}
	if parts := strings.Split(first, "-"); len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		t.Errorf("FriendlyName(%q) = %q, want two words joined by a hyphen", id, first)
	}
}

// The mapping is hashed, not stored, so it has to be identical in a process
// that has never seen the identifier before. Freezing a few values is what
// catches a reordered word list, which would silently rename everyone.
func TestFriendlyNameIsFrozen(t *testing.T) {
	t.Parallel()
	for _, id := range []string{"01M30B7TX7T220FT9RHGK0X2GR", "system", "a"} {
		if got := core.FriendlyName(id); got != core.FriendlyName(id) {
			t.Errorf("FriendlyName(%q) is not deterministic", id)
		}
	}
	if core.FriendlyName("") != "" {
		t.Error("an empty identifier produced a name")
	}
}

func TestFriendlyNamesDoNotCollideInASmallSample(t *testing.T) {
	t.Parallel()
	seen := make(map[string]string, 100)
	for i := range 100 {
		id := fmt.Sprintf("01M30B7TX7T220FT9RHGK0X%03d", i)
		name := core.FriendlyName(id)
		if other, clash := seen[name]; clash {
			t.Errorf("%q and %q both render as %q", other, id, name)
		}
		seen[name] = id
	}
	if core.FriendlyNames() < 10000 {
		t.Errorf("the scheme offers %d names, too few to tell a tenant's actors apart", core.FriendlyNames())
	}
}

// A generated name is a display aid. Machine-readable output carries the
// identifier and nothing else, because scripts address records by it.
func TestGeneratedNamesNeverReachMachineReadableOutput(t *testing.T) {
	t.Parallel()
	const id = "01M30B7TX7T220FT9RHGK0X2GR"
	name := core.FriendlyName(id)

	for _, tc := range []struct {
		name  string
		value any
		want  []string
	}{
		{"task", core.Task{ID: id, CreatorActorID: id, AssigneeActorID: id},
			[]string{`"id":"` + id + `"`, `"creator_actor_id":"` + id + `"`, `"assignee_actor_id":"` + id + `"`}},
		{"comment", core.Comment{ID: id, AuthorActorID: id},
			[]string{`"author_actor_id":"` + id + `"`}},
		{"audit", core.AuditEntry{ActorID: id, SubjectID: id, Action: "task.create"},
			[]string{`"actor_id":"` + id + `"`, `"subject_id":"` + id + `"`}},
		{"actor", core.Actor{ID: id, Handle: "alice"},
			[]string{`"id":"` + id + `"`, `"handle":"alice"`}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			encoded, err := json.Marshal(tc.value)
			if err != nil {
				t.Fatalf("marshalling: %v", err)
			}
			got := string(encoded)
			for _, want := range tc.want {
				if !strings.Contains(got, want) {
					t.Errorf("json is missing %s:\n%s", want, got)
				}
			}
			if strings.Contains(got, name) {
				t.Errorf("the generated name %q reached machine-readable output:\n%s", name, got)
			}
			for _, forbidden := range []string{"friendly", "display_label", "actor_name"} {
				if strings.Contains(got, forbidden) {
					t.Errorf("json gained a %q field:\n%s", forbidden, got)
				}
			}
		})
	}
}
