// SPDX-License-Identifier: AGPL-3.0-or-later

package docsmd_test

import (
	"fmt"
	"maps"
	"path"
	"slices"
	"testing"

	"github.com/heliopsy/tix/internal/docsmd"
)

// exampleFiles are the programs docs/examples.md walks through, with the fence
// language each is shown under so GitHub highlights it.
var exampleFiles = map[string]string{
	"examples/sh/tix-example.sh":         "sh",
	"examples/python/tix_example.py":     "python",
	"examples/typescript/tix-example.ts": "ts",
	"examples/go/main.go":                "go",
}

// TestTheExamplesPageShowsTheExampleFiles holds every snippet on the examples
// page to the region of the file it names, so the page cannot show code the
// smoke suite never ran. It also fails on a region the page leaves out and on
// a step one language has and another does not.
func TestTheExamplesPageShowsTheExampleFiles(t *testing.T) {
	page, err := docsmd.Read("docs/examples.md")
	if err != nil {
		t.Fatal(err)
	}
	snippets, err := docsmd.Snippets(page)
	if err != nil {
		t.Fatalf("docs/examples.md: %v", err)
	}
	regions := map[string]map[string]string{}
	var steps []string
	for _, file := range slices.Sorted(maps.Keys(exampleFiles)) {
		src, err := docsmd.Read(file)
		if err != nil {
			t.Fatal(err)
		}
		bodies, order, err := docsmd.Regions(src)
		if err != nil {
			t.Fatalf("%s: %v", file, err)
		}
		if steps == nil {
			steps = order
		} else if !slices.Equal(order, steps) {
			t.Errorf("%s marks the regions %q, want the same steps as the others: %q", file, order, steps)
		}
		regions[file] = bodies
	}

	shown := map[string]bool{}
	for _, s := range snippets {
		where := fmt.Sprintf("docs/examples.md:%d", s.Line)
		lang, known := exampleFiles[s.Path]
		if !known {
			t.Errorf("%s: %s is not one of the example files", where, s.Path)
			continue
		}
		if s.Lang != lang {
			t.Errorf("%s: %s is fenced as %q, want %q", where, path.Base(s.Path), s.Lang, lang)
		}
		want, ok := regions[s.Path][s.Region]
		if !ok {
			t.Errorf("%s: %s has no region %q", where, s.Path, s.Region)
			continue
		}
		if s.Body != want {
			t.Errorf("%s: the snippet of %s#%s has drifted from the file\npage:\n%s\nfile:\n%s",
				where, s.Path, s.Region, s.Body, want)
		}
		shown[s.Path+"#"+s.Region] = true
	}
	for file, bodies := range regions {
		for name := range bodies {
			if !shown[file+"#"+name] {
				t.Errorf("docs/examples.md never shows %s#%s", file, name)
			}
		}
	}
}
