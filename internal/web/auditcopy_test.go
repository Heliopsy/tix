// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"encoding/json"
	"html"
	"regexp"
	"strconv"
	"testing"
)

var rawCopyTag = regexp.MustCompile(`data-dlg-open="raw-(\d+)"[^>]*>.*?</a><button type="button" class="btn link small rawcopy" data-copy-text="([^"]*)"`)

// Every audit row carries its entry as the API's JSON, pretty-printed, on a
// copy button beside Raw, on each screen that draws audit rows.
func TestEveryAuditRowCopiesItsEntryAsJSON(t *testing.T) {
	t.Parallel()
	b, ref := rawFixture(t)
	for _, tc := range []struct{ name, path string }{
		{"activity", "/activity"},
		{"task history", "/tasks/" + ref},
		{"actor", "/actors/alice"},
	} {
		m := rawCopyTag.FindStringSubmatch(b.page(tc.path))
		if m == nil {
			t.Errorf("%s: the first Raw control has no copy button beside it", tc.name)
			continue
		}
		text := html.UnescapeString(m[2])
		var entry struct {
			Seq    int64  `json:"seq"`
			Action string `json:"action"`
		}
		if err := json.Unmarshal([]byte(text), &entry); err != nil {
			t.Errorf("%s: the copied text is not one entry: %v\n%s", tc.name, err, text)
			continue
		}
		if strconv.FormatInt(entry.Seq, 10) != m[1] || entry.Action == "" {
			t.Errorf("%s: copies entry %d (%q), want the row's own, %s", tc.name, entry.Seq, entry.Action, m[1])
		}
		if !regexp.MustCompile(`\n  "seq": `).MatchString(text) {
			t.Errorf("%s: the copied JSON is not pretty-printed:\n%s", tc.name, text)
		}
	}
}
