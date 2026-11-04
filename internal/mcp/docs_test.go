// SPDX-License-Identifier: AGPL-3.0-or-later

package mcp

import (
	"net/http"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/docsmd"
)

const docsPage = "docs/mcp.md"

func docsSection(t *testing.T, heading string) string {
	t.Helper()
	page, err := docsmd.Read(docsPage)
	if err != nil {
		t.Fatal(err)
	}
	section, err := docsmd.Section(page, heading)
	if err != nil {
		t.Fatalf("%s: %v", docsPage, err)
	}
	return section
}

func sameSet(t *testing.T, what string, got, want []string) {
	t.Helper()
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Errorf("%s lists %s as %v, the code says %v", docsPage, what, got, want)
	}
}

func TestDocsListTheOperationsNoToolCanSpeak(t *testing.T) {
	rows, err := docsmd.Table(docsSection(t, "## What becomes a tool"), "Left out", "Because it")
	if err != nil {
		t.Fatal(err)
	}
	var documented []string
	for _, cell := range docsmd.Column(rows, 0) {
		documented = append(documented, docsmd.Codes(cell)...)
	}
	var want []string
	for _, tool := range shippedPlan(t, Options{}) {
		if tool.Op.Unsupported != "" {
			want = append(want, tool.Name)
		}
	}
	sameSet(t, "the operations left out for not speaking JSON", documented, want)
}

func TestDocsNameEveryDestructiveToolThatIsNotADelete(t *testing.T) {
	method := map[string]string{}
	var want []string
	for _, tool := range shippedPlan(t, Options{}) {
		method[tool.Name] = tool.Op.Method
		if tool.Destructive && tool.Op.Method != http.MethodDelete {
			want = append(want, tool.Name)
		}
	}
	var documented []string
	for _, code := range docsmd.Codes(docsSection(t, "## Scopes and destructive tools")) {
		if m, isTool := method[code]; isTool && m != http.MethodDelete {
			documented = append(documented, code)
		}
	}
	sameSet(t, "the destructive tools that are not deletes", documented, want)
}

func TestDocsDescribeEveryOverlayKey(t *testing.T) {
	rows, err := docsmd.Table(docsSection(t, "## The overlay"), "Key", "Meaning")
	if err != nil {
		t.Fatal(err)
	}
	var documented []string
	for _, cell := range docsmd.Column(rows, 0) {
		documented = append(documented, docsmd.Codes(cell)...)
	}
	var want []string
	entry := reflect.TypeOf(Entry{})
	for i := range entry.NumField() {
		tag, _, _ := strings.Cut(entry.Field(i).Tag.Get("yaml"), ",")
		want = append(want, tag)
	}
	sameSet(t, "the overlay keys", documented, want)
}
