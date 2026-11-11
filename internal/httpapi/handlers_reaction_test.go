// SPDX-License-Identifier: AGPL-3.0-or-later

package httpapi_test

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/client"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/httpapi"
	"github.com/heliopsy/tix/internal/wire"
)

func reactionPath(id, reaction string) string {
	return strings.NewReplacer("{id}", id, "{reaction}", url.PathEscape(reaction)).Replace(wire.RouteCommentReaction)
}

func newReactedComment(t *testing.T, f *apiFixture) core.Comment {
	t.Helper()
	task := f.createTask("reacted")
	resp := f.call(http.MethodPost, "/api/v1/tasks/"+task.Ref+"/comments", httpapi.CommentRequest{Body: "ship it"})
	mustStatus(t, resp, http.StatusCreated)
	var c core.Comment
	decodeBody(t, resp, &c)
	return c
}

func TestReactionEndpointsToggleAndAnswerTheComment(t *testing.T) {
	f := newFixture(t)
	c := newReactedComment(t, f)

	steps := []struct {
		name, method, reaction string
		status                 int
		want                   string
	}{
		{"react", http.MethodPut, "heart", http.StatusOK, "heart 1 true"},
		{"react again", http.MethodPut, "heart", http.StatusOK, "heart 1 true"},
		{"react by emoji", http.MethodPut, "\U0001F680", http.StatusOK, "heart 1 true,rocket 1 true"},
		{"unreact", http.MethodDelete, "heart", http.StatusOK, "rocket 1 true"},
		{"unreact again", http.MethodDelete, "heart", http.StatusOK, "rocket 1 true"},
		{"unknown reaction", http.MethodPut, "party_parrot", http.StatusBadRequest, ""},
	}
	for _, st := range steps {
		resp := f.call(st.method, reactionPath(c.ID, st.reaction), nil)
		mustStatus(t, resp, st.status)
		if st.status != http.StatusOK {
			_ = resp.Body.Close()
			continue
		}
		var got core.Comment
		decodeBody(t, resp, &got)
		parts := make([]string, 0, len(got.Reactions))
		for _, r := range got.Reactions {
			parts = append(parts, string(r.Reaction)+" "+string(rune('0'+r.Count))+" "+map[bool]string{true: "true", false: "false"}[r.Reacted])
		}
		if s := strings.Join(parts, ","); s != st.want || got.ID != c.ID {
			t.Fatalf("%s: comment %s reactions %q, want %q", st.name, got.ID, s, st.want)
		}
	}

	resp := f.do(http.MethodPut, reactionPath(c.ID, "heart"), f.hostB, f.tokenB, nil)
	mustStatus(t, resp, http.StatusNotFound)
	_ = resp.Body.Close()
	resp = f.call(http.MethodPut, reactionPath("01JJJJJJJJJJJJJJJJJJJJJJJJ", "heart"), nil)
	mustStatus(t, resp, http.StatusNotFound)
	_ = resp.Body.Close()
}

func TestReactionSurvivesTheClientRoundTrip(t *testing.T) {
	f := newFixtureWith(t, func(fx *apiFixture, cfg *httpapi.Config) {
		cfg.DefaultTenantID = fx.tenantA.ID
	})
	c := newReactedComment(t, f)
	remote, err := client.New(f.server.URL, f.tokenA)
	if err != nil {
		t.Fatalf("building client: %v", err)
	}
	t.Cleanup(func() { _ = remote.Close() })
	ctx := context.Background()
	got, err := remote.AddCommentReaction(ctx, c.ID, core.ReactionEyes)
	if err != nil || len(got.Reactions) != 1 || got.Reactions[0].Emoji != core.ReactionEyes.Emoji() || !got.Reactions[0].Reacted {
		t.Fatalf("react = %+v, %v", got, err)
	}
	listed, err := remote.ListComments(ctx, core.TaskRef{ID: c.TaskID})
	if err != nil || len(listed) != 1 || len(listed[0].Reactions) != 1 {
		t.Fatalf("listed = %+v, %v", listed, err)
	}
	got, err = remote.RemoveCommentReaction(ctx, c.ID, core.ReactionEyes)
	if err != nil || len(got.Reactions) != 0 {
		t.Fatalf("unreact = %+v, %v", got, err)
	}
}
