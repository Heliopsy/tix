// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"context"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

// viaChip matches one rendered channel: its kind, its hover text and its
// words, so an assertion reads the chip and not the page around it.
var viaChip = regexp.MustCompile(`(?s)<span class="via via-([a-z-]+)"(?: title="([^"]*)")?>.*?<span>([^<]*)</span></span>`)

// channelCell is the history table's channel column, one row's cell.
var channelCell = regexp.MustCompile(`(?s)<td data-label="Channel">(.*?)</td>`)

type chip struct{ kind, title, label string }

func chipsIn(region string) []chip {
	var out []chip
	for _, m := range viaChip.FindAllStringSubmatch(region, -1) {
		out = append(out, chip{m[1], html.UnescapeString(m[2]), html.UnescapeString(m[3])})
	}
	return out
}

func findChip(chips []chip, label string) (chip, bool) {
	for _, c := range chips {
		if c.label == label {
			return c, true
		}
	}
	return chip{}, false
}

// seedChannels writes one task over each of three channels, as the entry
// points would have recorded them.
func seedChannels(t *testing.T, f *fixture) map[string]string {
	t.Helper()
	refs := map[string]string{}
	for _, tc := range []struct {
		title string
		via   core.Via
	}{
		{"filed over ssh", core.Via{Kind: core.ViaSSH, KeyFingerprint: "SHA256:abcdef", ClientAddr: "192.0.2.20"}},
		{"filed by ci", core.Via{Kind: core.ViaAPI, TokenID: "tok-1", TokenName: "ci-agent", ClientAddr: "192.0.2.30", UserAgent: "curl/8"}},
		{"filed by the agent", core.Via{Kind: core.ViaMCP, TokenID: "tok-2", TokenName: "mcp-agent"}},
	} {
		ctx := core.WithVia(core.WithActor(context.Background(), &f.actorA), tc.via)
		task, err := f.svc.CreateTask(ctx, core.CreateTaskInput{ProjectRef: "infra", Title: tc.title})
		if err != nil {
			t.Fatalf("creating %q: %v", tc.title, err)
		}
		refs[tc.title] = task.Ref
	}
	return refs
}

func TestTheActivityFeedShowsEachChannelWithItsDetails(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	seedChannels(t, f)
	b := f.as("alice")
	b.createTask("infra", "filed in the browser")

	feed := between(t, b.page("/activity"), `id="live-feed"`, `</ul>`)
	chips := chipsIn(feed)
	for _, want := range []struct{ label, kind, detail string }{
		{"via SSH", "ssh", "SSH key: SHA256:abcdef"},
		{"via token ci-agent", "api", "Address: 192.0.2.30"},
		{"via MCP (token mcp-agent)", "mcp", "Token ID: tok-2"},
		{"via web", "web", ""},
	} {
		c, ok := findChip(chips, want.label)
		if !ok {
			t.Errorf("no %q chip in the feed; chips: %+v", want.label, chips)
			continue
		}
		if c.kind != want.kind || !strings.Contains(c.title, want.detail) {
			t.Errorf("%q chip = %+v, want kind %s with %q in its hover", want.label, c, want.kind, want.detail)
		}
	}
	if !strings.Contains(feed, `href="/activity?token=ci-agent&amp;via=api"`) {
		t.Errorf("the ci-agent chip does not link to its channel and token:\n%s", feed)
	}
	var raws strings.Builder
	for _, m := range regexp.MustCompile(`class="btn link small rawopen" href="([^"#]+)`).FindAllStringSubmatch(feed, -1) {
		raws.WriteString(between(t, b.page(html.UnescapeString(m[1])), `<dialog id="raw-`, `</dialog>`))
	}
	if !strings.Contains(raws.String(), "<dt>Channel</dt>") || !strings.Contains(raws.String(), "SHA256:abcdef</dd>") {
		t.Errorf("the raw dialogs do not list the channel and its details:\n%s", raws.String())
	}
}

func TestTheActivityFeedFiltersByChannelAndToken(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	seedChannels(t, f)
	b := f.as("alice")

	for _, tc := range []struct {
		query string
		keep  string
		drop  []string
		chip  string
	}{
		{"via=ssh", "via SSH", []string{"via token ci-agent", "via MCP (token mcp-agent)"}, "via SSH"},
		{"token=ci-agent", "via token ci-agent", []string{"via SSH", "via MCP (token mcp-agent)"}, "token ci-agent"},
		{"via=mcp&token=ci-agent", "", []string{"via SSH", "via token ci-agent", "via MCP (token mcp-agent)"}, "via MCP"},
	} {
		t.Run(tc.query, func(t *testing.T) {
			page := b.page("/activity?" + tc.query)
			chips := chipsIn(between(t, page, `id="live-feed"`, `</ul>`))
			if _, ok := findChip(chips, tc.keep); tc.keep != "" && !ok {
				t.Errorf("%s dropped %q: %+v", tc.query, tc.keep, chips)
			}
			for _, d := range tc.drop {
				if _, ok := findChip(chips, d); ok {
					t.Errorf("%s kept %q", tc.query, d)
				}
			}
			if active := activeChips(t, page); !strings.Contains(active, tc.chip) {
				t.Errorf("%s shows no %q chip:\n%s", tc.query, tc.chip, active)
			}
		})
	}

	form := between(t, b.page("/activity?via=ssh"), `<form method="get" action="/activity"`, `</form>`)
	if !strings.Contains(form, `<option value="ssh" selected>SSH</option>`) {
		t.Errorf("the channel control does not show the channel being filtered by:\n%s", form)
	}
	if !strings.Contains(form, `name="token"`) {
		t.Errorf("the filter has no token box:\n%s", form)
	}
	resp := b.get("/activity?via=" + url.QueryEscape("telepathy"))
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest || !strings.Contains(string(body), "unknown access channel") {
		t.Errorf("a made-up channel = %d, want it refused by name:\n%s", resp.StatusCode, body)
	}
}

func TestATaskHistoryShowsTheChannelColumn(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	refs := seedChannels(t, f)
	b := f.as("alice")

	region := historyRegion(t, b.page("/tasks/"+refs["filed by ci"]))
	if !strings.Contains(region, "<th>Channel</th>") {
		t.Errorf("the history table has no channel column:\n%s", region)
	}
	var cells []chip
	for _, m := range channelCell.FindAllStringSubmatch(region, -1) {
		cells = append(cells, chipsIn(m[1])...)
	}
	c, ok := findChip(cells, "via token ci-agent")
	if !ok || !strings.Contains(c.title, "User agent: curl/8") {
		t.Errorf("the channel column shows %+v, want the ci-agent token with its user agent in the hover", cells)
	}
	var linked bool
	for _, m := range channelCell.FindAllStringSubmatch(region, -1) {
		if strings.Contains(m[1], `<a class="via-link" href="/activity?token=ci-agent&amp;via=api"`) {
			linked = true
		}
	}
	if !linked {
		t.Errorf("the channel cell does not link to the feed narrowed to the ci-agent token:\n%s", region)
	}
}

// An entry written before channels were recorded says so, rather than
// claiming a channel nobody recorded.
func TestAnOldEntryReadsAsAnUnknownChannel(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	ctx := core.WithActor(context.Background(), &f.actorA)
	if _, err := f.svc.CreateTask(ctx, core.CreateTaskInput{ProjectRef: "infra", Title: "from before"}); err != nil {
		t.Fatal(err)
	}
	feed := between(t, f.as("alice").page("/activity"), `id="live-feed"`, `</ul>`)
	if c, ok := findChip(chipsIn(feed), "via unknown channel"); !ok || c.kind != "unknown" {
		t.Errorf("an entry with no channel shows %+v", chipsIn(feed))
	}
}
