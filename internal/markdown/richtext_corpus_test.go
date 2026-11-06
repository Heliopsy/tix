// SPDX-License-Identifier: AGPL-3.0-or-later

package markdown_test

import (
	"encoding/json"
	"os"
	"slices"
	"testing"

	"github.com/heliopsy/tix/internal/markdown"
)

// richCorpus is the web description editor's round-trip corpus. Each kept
// entry pairs a text with what the rich view writes back for it, and the
// node suite (internal/web/jstest/richtext.test.mjs) proves the pairing.
type richCorpus struct {
	Keep []struct {
		Name string `json:"name"`
		In   string `json:"in"`
		Out  string `json:"out"`
	} `json:"keep"`
}

// TestTheRichEditorsMarkdownRendersAsTheOriginal closes the loop the node
// suite cannot: its check renders with markdown-it, and the page renders with
// goldmark. A text the rich editor rewrites must draw the same HTML here, and
// carry the same checklist, or an edit would change what readers see.
func TestTheRichEditorsMarkdownRendersAsTheOriginal(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile("../web/jstest/testdata/richtext-corpus.json")
	if err != nil {
		t.Fatal(err)
	}
	var corpus richCorpus
	if err := json.Unmarshal(raw, &corpus); err != nil {
		t.Fatal(err)
	}
	if len(corpus.Keep) < 20 {
		t.Fatalf("the corpus holds only %d entries", len(corpus.Keep))
	}
	opt := markdown.Options{
		TaskHref:   func(ref string) string { return "/tasks/" + ref },
		HandleHref: func(handle string) string { return "/users/" + handle },
	}
	for _, tt := range corpus.Keep {
		t.Run(tt.Name, func(t *testing.T) {
			t.Parallel()
			if got, want := markdown.Render(tt.Out, opt), markdown.Render(tt.In, opt); got != want {
				t.Errorf("rewritten text renders differently\n in: %q\nout: %q\nwant %s\n got %s", tt.In, tt.Out, want, got)
			}
			if got, want := markdown.Items(tt.Out), markdown.Items(tt.In); !slices.Equal(got, want) {
				t.Errorf("checklist of %q = %v, want %v", tt.Out, got, want)
			}
		})
	}
}
