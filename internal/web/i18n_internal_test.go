// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"go/ast"
	"go/parser"
	gotoken "go/token"
	"html/template"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"
	"text/template/parse"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/i18n"
	"github.com/heliopsy/tix/internal/output"
)

// untranslatedExempt are templates the literal check does not read yet, each
// with the reason. Keep it short: an entry here is a screen a reader in
// another language sees in English.
var untranslatedExempt = map[string]string{}

// literalAttrs are the attributes whose values a reader sees or hears.
var literalAttrs = regexp.MustCompile(`\s(?:title|placeholder|aria-label|alt|data-rail-name|aria-valuetext|data-confirm)="([^"]*)"`)

var (
	templateComment = regexp.MustCompile(`(?s)\{\{-?\s*/\*.*?\*/\s*-?\}\}`)
	templateAction  = regexp.MustCompile(`(?s)\{\{.*?\}\}`)
	verbatimElement = regexp.MustCompile(`(?s)<(code|kbd|pre|script|style|samp)\b[^>]*>.*?</(?:code|kbd|pre|script|style|samp)>`)
	htmlComment     = regexp.MustCompile(`(?s)<!--.*?-->`)
	htmlTag         = regexp.MustCompile(`(?s)<[^>]*>`)
	htmlEntity      = regexp.MustCompile(`&#?[a-zA-Z0-9]+;`)
	twoLetters      = regexp.MustCompile(`\p{L}{2}`)
)

// untranslatedLiterals returns the words a template writes straight into a
// page instead of through the catalog: text between tags, and the value of
// every attribute a reader sees or hears. Code, keys and examples inside
// code, kbd, pre and samp are syntax rather than prose and are left alone.
func untranslatedLiterals(src string) []string {
	src = templateComment.ReplaceAllString(src, "")
	src = templateAction.ReplaceAllString(src, "\x00")
	src = htmlComment.ReplaceAllString(src, "")
	src = verbatimElement.ReplaceAllString(src, "\x00")
	var out []string
	keep := func(chunk string) {
		chunk = strings.TrimSpace(htmlEntity.ReplaceAllString(chunk, " "))
		if twoLetters.MatchString(chunk) {
			out = append(out, chunk)
		}
	}
	for _, m := range literalAttrs.FindAllStringSubmatch(src, -1) {
		for _, chunk := range strings.Split(m[1], "\x00") {
			keep(chunk)
		}
	}
	for _, chunk := range strings.Split(htmlTag.ReplaceAllString(src, "\x00"), "\x00") {
		keep(chunk)
	}
	return out
}

// TestTemplatesWriteNoWordsOutsideTheCatalog fails on any template that
// writes a word a reader sees without passing it through t, so a screen
// cannot grow an English-only string unnoticed.
func TestTemplatesWriteNoWordsOutsideTheCatalog(t *testing.T) {
	t.Parallel()
	for name, src := range templateSources(t) {
		if _, skip := untranslatedExempt[name]; skip {
			continue
		}
		for _, literal := range untranslatedLiterals(src) {
			t.Errorf("%s writes %q outside the catalog; wrap it in {{t \"...\"}}", name, literal)
		}
	}
}

// TestTheLiteralCheckSeesTextAndAttributes pins what the check above reads,
// so a regexp edit cannot quietly blind it.
func TestTheLiteralCheckSeesTextAndAttributes(t *testing.T) {
	t.Parallel()
	cases := []struct {
		src  string
		want []string
	}{
		{`<h1>Tasks</h1>`, []string{"Tasks"}},
		{`<h1>{{t "Tasks"}}</h1>`, nil},
		{`<button title="Star {{.Ref}}">x</button>`, []string{"Star"}},
		{`<a aria-label="{{if .S}}Unstar{{else}}Star{{end}}">`, []string{"Unstar", "Star"}},
		{`<p>Try <code>is:starred</code></p>`, []string{"Try"}},
		{`{{/* a comment */}}<p>&middot; v{{.V}}</p>`, nil},
		{`<input placeholder="{{t "Search"}}" class="wide">`, nil},
	}
	for _, tc := range cases {
		if got := untranslatedLiterals(tc.src); !slices.Equal(got, tc.want) {
			t.Errorf("%s: got %q, want %q", tc.src, got, tc.want)
		}
	}
}

// TestEveryLanguageCoversEveryKey checks each catalog against every key the
// interface can ask for: the strings templates pass to the translation
// helpers, the labels Go hands them, the flashes and titles handlers set, and
// the texts the scripts write. A missing entry would read in English; an
// entry nothing asks for is a translation left behind when its key changed.
func TestEveryLanguageCoversEveryKey(t *testing.T) {
	t.Parallel()
	keys := allKeys(t)
	if len(keys) < 100 {
		t.Fatalf("only %d keys found; the collector is not reading the templates", len(keys))
	}
	langs := i18n.Languages()
	if len(langs) < 2 {
		t.Fatalf("languages = %v, want English and at least one other", langs)
	}
	for _, lang := range langs[1:] {
		catalog := i18n.Catalog(lang)
		var missing, unused []string
		for key := range keys {
			if _, ok := catalog[key]; !ok {
				missing = append(missing, key)
			}
		}
		for key := range catalog {
			if _, ok := keys[key]; !ok {
				unused = append(unused, key)
			}
		}
		sort.Strings(missing)
		sort.Strings(unused)
		for _, key := range missing {
			t.Errorf("%s has no entry for %q (from %s)", lang, key, keys[key])
		}
		for _, key := range unused {
			t.Errorf("%s has an entry for %q, which nothing asks for", lang, key)
		}
	}
}

// TestTranslationsKeepTheirVerbsAndMarkup checks that every entry formats the
// same arguments as its key and carries the same tags, so a translation can
// neither drop a count nor smuggle markup into a th string.
func TestTranslationsKeepTheirVerbsAndMarkup(t *testing.T) {
	t.Parallel()
	verb := regexp.MustCompile(`%(?:\[\d+\])?[-+# 0]*\d*(?:\.\d+)?[a-zA-Z]`)
	tag := regexp.MustCompile(`</?[a-z]+[^>]*>`)
	for _, lang := range i18n.Languages()[1:] {
		for key, text := range i18n.Catalog(lang) {
			if strings.TrimSpace(text) == "" {
				t.Errorf("%s %q is blank", lang, key)
			}
			if a, b := len(verb.FindAllString(strings.ReplaceAll(key, "%%", ""), -1)),
				len(verb.FindAllString(strings.ReplaceAll(text, "%%", ""), -1)); a != b {
				t.Errorf("%s %q formats %d arguments, its key %d", lang, text, b, a)
			}
			want, got := tag.FindAllString(key, -1), tag.FindAllString(text, -1)
			sort.Strings(want)
			sort.Strings(got)
			if !slices.Equal(want, got) {
				t.Errorf("%s %q carries tags %q, its key %q", lang, text, got, want)
			}
		}
	}
}

// TestEveryScriptTextIsCarried reads every say("...") in the shipped scripts
// and fails on one the head would not carry a translation for.
func TestEveryScriptTextIsCarried(t *testing.T) {
	t.Parallel()
	said := regexp.MustCompile(`say\((?:[^()"]*\?\s*)?"([^"]+)"(?:\s*:\s*"([^"]+)")?\)`)
	found := 0
	for _, name := range assetNames(t, ".js") {
		src := readAsset(t, name)
		for _, m := range said.FindAllStringSubmatch(src, -1) {
			for _, text := range m[1:] {
				if text == "" {
					continue
				}
				found++
				if !slices.Contains(jsMessages, text) {
					t.Errorf("%s says %q, which jsMessages does not carry", name, text)
				}
			}
		}
	}
	if found < len(jsMessages) {
		t.Errorf("found %d say() texts, fewer than the %d jsMessages carries", found, len(jsMessages))
	}
}

// templateSources reads every template, keyed by file name.
func templateSources(t *testing.T) map[string]string {
	t.Helper()
	entries, err := fs.ReadDir(templateFS, "templates")
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for _, e := range entries {
		b, err := fs.ReadFile(templateFS, "templates/"+e.Name())
		if err != nil {
			t.Fatal(err)
		}
		out[e.Name()] = string(b)
	}
	return out
}

func assetNames(t *testing.T, ext string) []string {
	t.Helper()
	entries, err := os.ReadDir("assets")
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ext && e.Name() != "htmx.min.js" {
			out = append(out, e.Name())
		}
	}
	return out
}

func readAsset(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("assets", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// allKeys is every key the interface can ask a catalog for, mapped to where
// it was found.
func allKeys(t *testing.T) map[string]string {
	t.Helper()
	keys := map[string]string{}
	for name, src := range templateSources(t) {
		if _, skip := untranslatedExempt[name]; skip {
			continue
		}
		for _, k := range templateKeys(t, name, src) {
			keys[k] = name
		}
	}
	for _, k := range goKeys() {
		keys[k] = "a Go label"
	}
	for _, k := range handlerKeys(t) {
		keys[k] = "a handler"
	}
	for _, k := range jsMessages {
		keys[k] = "a script"
	}
	return keys
}

// keyArgs is how many leading string arguments of each helper are keys.
var keyArgs = map[string]int{"t": 1, "th": 1, "tn": 2, "tip": 2, "savedNext": 1}

// keyTemplates are partials that translate the string they are handed.
var keyTemplates = map[string]bool{"field-info": true}

// templateKeys walks one template's parse trees for the literal strings it
// hands to the translation helpers and to partials that translate.
func templateKeys(t *testing.T, name, src string) []string {
	t.Helper()
	tmpl, err := template.New(name).Funcs(funcs(output.TimeStyle{})).Funcs(translators(i18n.English)).Parse(src)
	if err != nil {
		t.Fatalf("parsing %s: %v", name, err)
	}
	var out []string
	var walk func(parse.Node)
	walkPipe := func(p *parse.PipeNode) {
		if p == nil {
			return
		}
		for _, cmd := range p.Cmds {
			walk(cmd)
		}
	}
	walk = func(n parse.Node) {
		switch n := n.(type) {
		case *parse.ListNode:
			if n == nil {
				return
			}
			for _, c := range n.Nodes {
				walk(c)
			}
		case *parse.ActionNode:
			walkPipe(n.Pipe)
		case *parse.IfNode:
			walkPipe(n.Pipe)
			walk(n.List)
			walk(n.ElseList)
		case *parse.RangeNode:
			walkPipe(n.Pipe)
			walk(n.List)
			walk(n.ElseList)
		case *parse.WithNode:
			walkPipe(n.Pipe)
			walk(n.List)
			walk(n.ElseList)
		case *parse.TemplateNode:
			if keyTemplates[n.Name] && n.Pipe != nil && len(n.Pipe.Cmds) == 1 {
				if s, ok := n.Pipe.Cmds[0].Args[0].(*parse.StringNode); ok {
					out = append(out, s.Text)
				}
			}
			walkPipe(n.Pipe)
		case *parse.PipeNode:
			walkPipe(n)
		case *parse.CommandNode:
			if id, ok := n.Args[0].(*parse.IdentifierNode); ok {
				if id.Ident == "tc" && len(n.Args) == 3 {
					ctx, ok1 := n.Args[1].(*parse.StringNode)
					text, ok2 := n.Args[2].(*parse.StringNode)
					if ok1 && ok2 {
						out = append(out, ctx.Text+i18n.Context+text.Text)
					}
				}
				for i := 1; i <= keyArgs[id.Ident] && i < len(n.Args); i++ {
					if s, ok := n.Args[i].(*parse.StringNode); ok {
						out = append(out, s.Text)
					}
				}
			}
			for _, a := range n.Args {
				walk(a)
			}
		}
	}
	for _, d := range tmpl.Templates() {
		if d.Tree != nil {
			walk(d.Tree.Root)
		}
	}
	return out
}

// labelSets are further sets of fixed labels Go code hands templates,
// registered beside the screens that translate them.
var labelSets []func() []string

// goKeys are the fixed labels Go code hands templates, which a template
// translates with t rather than naming as a literal.
func goKeys() []string {
	var out []string
	for _, set := range labelSets {
		out = append(out, set()...)
	}
	for _, kind := range core.Kinds {
		out = append(out, headingFor(kind))
		if hint := hintFor(kind); hint != "" {
			out = append(out, hint)
		}
	}
	out = append(out, "internal error", "pruning removed records", "dry run: nothing was removed",
		"%s saved", "Columns on %s", "Server default", "Browser default")
	for _, v := range categoryLabels {
		out = append(out, v)
	}
	return out
}

// renderers are the functions whose literal string arguments reach a page:
// a flash, a page title, a pager's unit. The index is the argument that carries it.
var renderers = map[string][]int{
	"redirect":        {3},
	"redirectTo":      {4},
	"afterTransition": {3},
	"render":          {3},
	"renderStatus":    {4, 5},
	"renderFragment":  {4},
	"newView":         {2},
	"newPager":        {4},
	"savedNext":       {0},
}

// handlerKeys reads this package's Go source for the literal flashes and
// titles its handlers pass to the functions above.
func handlerKeys(t *testing.T) []string {
	t.Helper()
	fset := gotoken.NewFileSet()
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			var name string
			switch fn := call.Fun.(type) {
			case *ast.Ident:
				name = fn.Name
			case *ast.SelectorExpr:
				name = fn.Sel.Name
			}
			for _, i := range renderers[name] {
				if i >= len(call.Args) {
					continue
				}
				lit, ok := call.Args[i].(*ast.BasicLit)
				if !ok || lit.Kind != gotoken.STRING {
					continue
				}
				if s, err := strconv.Unquote(lit.Value); err == nil && s != "" {
					out = append(out, s)
				}
			}
			return true
		})
	}
	return out
}
