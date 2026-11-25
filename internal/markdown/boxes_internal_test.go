// SPDX-License-Identifier: AGPL-3.0-or-later

package markdown

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"
)

// boxSamples are texts where the full parse and the checklist parse could
// part ways if the second missed something that decides a checkbox.
var boxSamples = []string{
	"- [ ] one\n- [x] two\n- [X] three",
	"* [ ] star\n+ [x] plus\n1. [ ] numbered\n2) [x] paren",
	"- [ ]\n- [x]",
	"- [\t] tab\n- [ ]no space",
	"- [y] not a box\n- [] empty\n- [  ] two spaces",
	"- `[ ]` code\n- **[ ]** bold\n- [link](https://x.test)\n- [x](https://x.test)",
	"- [x]: https://example.test\n- [ ] after a definition",
	"[x]: https://example.test\n\n- [x] uses the label",
	"```\n- [ ] fenced\n```\n\n    - [ ] indented code\n\n- [ ] real",
	"> - [ ] quoted\n> - [x] quoted done",
	"- parent\n  - [ ] child\n    - [x] grandchild",
	"- [ ] loose\n\n- [x] loose too\n\n  second paragraph [ ] no",
	"- [ ] | a | b |\n  |---|---|\n  | 1 | 2 |",
	"| [ ] | b |\n|---|---|\n| - [ ] | x |",
	"- <span>[ ]</span> html\n- \\[ ] escaped",
	"Setext\n- [ ] after\n===\n",
	"- [ ] crlf\r\n- [x] crlf\r\n",
	"text [ ] inline [x] boxes that are not items",
	"- a\n[ ] lazy continuation",
	"1. [ ] one\n   - [x] nested in ordered",
	"<div>\n- [ ] html block\n</div>\n\n- [ ] after",
	"- ~~[ ]~~ struck\n- [ ] ~~struck text~~",
	"- [ ] https://example.test autolink",
}

func fullBoxes(src string) []bool {
	list := boxes(parse([]byte(src)))
	out := make([]bool, len(list))
	for i, box := range list {
		out[i] = checked(box)
	}
	return out
}

// TestTheChecklistParserFindsTheItemsTheRendererDraws holds the checklist
// parser to the renderer's: Render numbers the items SetItem ticks, so the two
// must agree on every text.
func TestTheChecklistParserFindsTheItemsTheRendererDraws(t *testing.T) {
	t.Parallel()
	samples := slices.Clone(boxSamples)
	if raw, err := os.ReadFile("../web/jstest/testdata/richtext-corpus.json"); err == nil {
		var corpus struct {
			Keep []struct{ In, Out string } `json:"keep"`
		}
		if err := json.Unmarshal(raw, &corpus); err != nil {
			t.Fatal(err)
		}
		for _, e := range corpus.Keep {
			samples = append(samples, e.In, e.Out)
		}
	}
	for _, src := range samples {
		if got, want := Items(src), fullBoxes(src); !slices.Equal(got, want) {
			t.Errorf("Items(%q) = %v, the renderer finds %v", src, got, want)
		}
	}
}

// FuzzTheChecklistParserFindsTheItemsTheRendererDraws is the same comparison
// over generated text.
func FuzzTheChecklistParserFindsTheItemsTheRendererDraws(f *testing.F) {
	for _, s := range boxSamples {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, src string) {
		if got, want := Items(src), fullBoxes(src); !slices.Equal(got, want) {
			t.Errorf("Items(%q) = %v, the renderer finds %v", src, got, want)
		}
	})
}

// parseBudget is what finding the checklist of a description may allocate,
// as a multiple of its length. The checklist parser allocates about 14 times
// the text; the full parse it replaced allocates about 22 times.
const parseBudget = 16

// TestCountingAChecklistCostsLittleMoreThanTheText guards the task listings'
// memory: every listed task's checklist is counted, and counting a page of
// imported descriptions was the largest allocation a screen made. A long text
// is parsed once; counting it again allocates nothing.
func TestCountingAChecklistCostsLittleMoreThanTheText(t *testing.T) {
	var b strings.Builder
	for i := 0; b.Len() < 50<<10; i++ {
		fmt.Fprintf(&b, "- [ ] step %d see [the doc](https://example.test/doc/%d), **bold** and `code`\n", i, i)
		b.WriteString("  A paragraph under the item that goes on for a while in plain words.\n")
	}
	boxed := b.String()
	links := strings.ReplaceAll(boxed, "[ ]", "--")
	allocs := func(f func()) int64 {
		return testing.Benchmark(func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				f()
			}
		}).AllocedBytesPerOp()
	}
	for _, c := range []struct {
		name   string
		count  func()
		budget int64
	}{
		{"parsing a checklist", func() { countBoxes(boxed) }, parseBudget * int64(len(boxed))},
		{"counting a checklist again", func() { Checklist(boxed) }, 1 << 10},
		{"counting links and no checklist", func() { Checklist(links) }, 1 << 10},
	} {
		got := allocs(c.count)
		t.Logf("%s of %d KB allocates %d KB", c.name, len(boxed)>>10, got>>10)
		if got > c.budget {
			t.Errorf("%s of %d KB allocates %d KB, over the %d KB budget", c.name, len(boxed)>>10, got>>10, c.budget>>10)
		}
	}
}

// TestTheCountCacheStaysBounded fills the cache past its size and checks it
// never holds more.
func TestTheCountCacheStaysBounded(t *testing.T) {
	c := countCache{m: map[uint64][2]int{}}
	for i := range uint64(countCacheSize * 3) {
		c.put(i, [2]int{1, 2})
		if len(c.m) > countCacheSize {
			t.Fatalf("the cache holds %d counts, over its %d", len(c.m), countCacheSize)
		}
	}
	if v, ok := c.get(countCacheSize*3 - 1); !ok || v != [2]int{1, 2} {
		t.Fatalf("the newest count is %v, %v", v, ok)
	}
}
