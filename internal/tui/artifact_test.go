// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"slices"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/capability"
	"github.com/heliopsy/tix/internal/core"
)

// TestTheArtifactFormOffersEveryKindTheServiceAccepts keeps the picker and the
// contract together: a kind the form omits is unreachable, and one it invents
// is refused on apply.
func TestTheArtifactFormOffersEveryKindTheServiceAccepts(t *testing.T) {
	options := ArtifactKindOptions()
	if len(options) != len(core.ArtifactKinds) {
		t.Fatalf("the form offers %d kinds for %d the service has", len(options), len(core.ArtifactKinds))
	}
	for _, kind := range core.ArtifactKinds {
		if !slices.Contains(options, string(kind)) {
			t.Errorf("the form cannot record a %q artifact", kind)
		}
		if !kind.Valid() {
			t.Errorf("the form would offer %q, which the service does not accept", kind)
		}
	}
}

// artifactForm opens the artifact input the way a reader does: the key, the
// name, then the form that classifies it.
func artifactForm(t *testing.T, name string) (Model, *fakeService) {
	t.Helper()
	m := boardModel(t)
	svc := newFakeService()
	m.svc = svc
	m, cmd := m.reduce(pressKey("O"))
	if m.prompt != promptArtifact {
		t.Fatalf("O opened prompt %v", m.prompt)
	}
	m.input.SetValue(name)
	m, _ = m.reduce(pressKey("enter"))
	if !m.form.Open() {
		t.Fatalf("naming an artifact opened no form; err = %q cmd = %v", m.err, cmd)
	}
	return m, svc
}

func TestRecordingAnArtifactGathersANameAndAKind(t *testing.T) {
	m, svc := artifactForm(t, "run.log")
	frame := m.Frame()

	block := formBlock(t, frame, "artifact run.log")
	if !strings.Contains(block, ArtifactNote) {
		t.Errorf("the form does not say what it cannot record:\n%s", block)
	}
	if row := formRow(t, frame, "artifact run.log", "kind"); !strings.Contains(row, "result") {
		t.Errorf("the kind row opens on %q rather than on the default kind", row)
	}

	// One step to the second kind the service declares, which is what proves
	// the row is a question rather than a fixed label.
	m, _ = m.reduce(pressKey("right"))
	if row := formRow(t, m.Frame(), "artifact run.log", "kind"); !strings.Contains(row, string(core.ArtifactLog)) {
		t.Fatalf("cycling the kind left it on %q", row)
	}

	next, cmd := m.reduce(pressKey("enter"))
	if cmd == nil {
		t.Fatalf("applying the form asked for nothing; err = %q", next.err)
	}
	next, _ = next.reduce(run(t, cmd))
	if len(svc.artifacts) != 1 {
		t.Fatalf("artifacts = %+v", svc.artifacts)
	}
	got := svc.artifacts[0]
	if got.Kind != core.ArtifactLog || got.Name != "run.log" {
		t.Fatalf("recorded %+v, want the name the prompt took and the kind the form chose", got)
	}
	if err := got.Validate(); err != nil {
		t.Fatalf("the form gathered an input the service refuses: %v", err)
	}
	task, _ := TaskAt(m.columns, m.sel)
	if !strings.Contains(next.status, "run.log") || !strings.Contains(next.status, task.Ref) {
		t.Errorf("the status bar says %q", next.status)
	}
}

func TestAnUnnamedArtifactIsNotRecorded(t *testing.T) {
	m := boardModel(t)
	svc := newFakeService()
	m.svc = svc
	m, _ = m.reduce(pressKey("O"))
	m.input.SetValue("   ")
	next, cmd := m.reduce(pressKey("enter"))
	if next.form.Open() {
		t.Fatal("a blank name opened the kind form")
	}
	if cmd != nil {
		next.reduce(run(t, cmd))
	}
	if len(svc.artifacts) != 0 {
		t.Fatalf("a blank name recorded %+v", svc.artifacts)
	}
}

// TestTheArtifactKeyIsNeitherOfferedNorSentWithoutArtifactWrite watches both
// halves of the gate: the help overlay drops the key and the keystroke sends
// nothing.
func TestTheArtifactKeyIsNeitherOfferedNorSentWithoutArtifactWrite(t *testing.T) {
	m := boardModel(t)
	svc := newFakeService()
	m.svc = svc
	m.actor = &core.Actor{ID: "u-viewer", TenantID: "t", Kind: core.ActorUser, Role: core.RoleViewer}
	m.allowed = resolveActions(m.actor)
	if m.mayPerform("PutArtifact") {
		t.Fatal("a viewer holds artifact:write, so this test proves nothing")
	}

	help := m.keys.ViewHelp(viewBoard, m.permits())
	if slices.ContainsFunc(help, func(e HelpEntry) bool { return e.Keys == "O" }) {
		t.Errorf("a reader who may not record artifacts is told about O: %+v", help)
	}
	next, cmd := m.reduce(pressKey("O"))
	if next.prompt != promptNone {
		t.Fatal("a reader who may not record artifacts was asked for a name")
	}
	if cmd != nil {
		next.reduce(run(t, cmd))
	}
	if len(svc.artifacts) != 0 {
		t.Fatalf("a reader who may not record artifacts recorded %+v", svc.artifacts)
	}
}

func TestEverySchemeRecordsAnArtifact(t *testing.T) {
	for _, scheme := range Schemes() {
		t.Run(string(scheme), func(t *testing.T) {
			m := boardModel(t)
			svc := newFakeService()
			m.svc = svc
			m = m.installScheme(string(scheme))
			if m.err != "" {
				t.Fatalf("installing %s: %s", scheme, m.err)
			}
			m, _ = m.reduce(keyMsgFor(m.keys.Artifact.Keys()[0]))
			if m.prompt != promptArtifact {
				t.Fatalf("%s: the artifact key opened prompt %v", scheme, m.prompt)
			}
			m.input.SetValue("result.json")
			m, _ = m.reduce(keyMsgFor(m.keys.Accept.Keys()[0]))
			if !m.form.Open() {
				t.Fatalf("%s: the name opened no form; err = %q", scheme, m.err)
			}
			m, cmd := m.reduce(keyMsgFor(m.keys.Accept.Keys()[0]))
			if cmd == nil {
				t.Fatalf("%s: applying the form asked for nothing", scheme)
			}
			m.reduce(run(t, cmd))
			if len(svc.artifacts) != 1 || svc.artifacts[0].Name != "result.json" {
				t.Fatalf("%s: artifacts = %+v", scheme, svc.artifacts)
			}
		})
	}
}

// TestTheRegistryRecordsWhatTheArtifactFormCannotGather keeps the binding
// honest: the terminal records an artifact, and it does not record all of one.
func TestTheRegistryRecordsWhatTheArtifactFormCannotGather(t *testing.T) {
	op, ok := capability.ByMethod("PutArtifact")
	if !ok {
		t.Fatal("the registry declares no artifact put")
	}
	if op.TUI != viewName(viewDetail) {
		t.Fatalf("the artifact put is bound to view %q", op.TUI)
	}
	limit, found := op.LimitationFor(capability.SurfaceTUI)
	if !found {
		t.Fatal("the terminal binding claims the whole operation, and the form gathers no payload")
	}
	for _, part := range []string{"payload", "blob"} {
		if !strings.Contains(limit.Reason, part) {
			t.Errorf("the recorded shortfall does not name the %s: %q", part, limit.Reason)
		}
	}
}
