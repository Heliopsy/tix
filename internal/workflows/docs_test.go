// SPDX-License-Identifier: AGPL-3.0-or-later

package workflows_test

import (
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/docsmd"
	"github.com/heliopsy/tix/internal/workflows"
)

// TestTheDocsDescribeEveryTemplate keeps the Templates section of
// docs/workflows.md in step with the files: one heading per template, naming
// it and its key, whose body states its purpose, every state and every field.
func TestTheDocsDescribeEveryTemplate(t *testing.T) {
	t.Parallel()
	md, err := docsmd.Read("docs/workflows.md")
	if err != nil {
		t.Fatalf("reading the page under test: %v", err)
	}
	all, err := workflows.Templates()
	if err != nil {
		t.Fatalf("loading templates: %v", err)
	}
	for _, tpl := range all {
		heading := "### " + tpl.Name + " (`" + tpl.Key + "`)"
		body, err := docsmd.Section(md, heading)
		if err != nil {
			t.Errorf("docs/workflows.md: %v", err)
			continue
		}
		if !strings.Contains(body, tpl.Purpose) {
			t.Errorf("%s: the docs do not state its purpose %q", tpl.Key, tpl.Purpose)
		}
		for _, s := range tpl.Definition.States {
			if !strings.Contains(body, "`"+s.Key+"`") {
				t.Errorf("%s: the docs do not name state %q", tpl.Key, s.Key)
			}
		}
		for _, f := range tpl.Fields {
			if !strings.Contains(body, "`"+f.Key+"`") {
				t.Errorf("%s: the docs do not name field %q", tpl.Key, f.Key)
			}
		}
	}
}
