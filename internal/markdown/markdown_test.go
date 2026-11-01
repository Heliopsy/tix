// SPDX-License-Identifier: AGPL-3.0-or-later

package markdown_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/markdown"
)

// xssCorpus is text a hostile writer could put in a description or comment.
// Every entry must render without a live tag, handler or dangerous address.
var xssCorpus = []string{
	`<script>alert(1)</script>`,
	`<img src=x onerror=alert(1)>`,
	`inline <b onclick="alert(1)">bold</b> html`,
	`<svg/onload=alert(1)>`,
	`<iframe src="https://evil.example"></iframe>`,
	`<a href="javascript:alert(1)">x</a>`,
	`[x](javascript:alert(1))`,
	`[x](JaVaScRiPt:alert(1))`,
	`[x](  javascript:alert(1) )`,
	`[x](java&#x09;script:alert(1))`,
	`[x](java%0ascript:alert(1))`,
	`[x](vbscript:msgbox(1))`,
	`[x](data:text/html;base64,PHNjcmlwdD5hbGVydCgxKTwvc2NyaXB0Pg==)`,
	`[x](file:///etc/passwd)`,
	`[x](//evil.example/path)`,
	`<javascript:alert(1)>`,
	`![x](javascript:alert(1))`,
	`![x](data:image/svg+xml;base64,PHN2Zz48L3N2Zz4=)`,
	`![x" onerror="alert(1)](https://example.com/a.png)`,
	`[x](https://example.com/"onmouseover="alert(1))`,
	`[x](https://example.com "title\" onmouseover=\"alert(1)")`,
	`[x" onclick="alert(1)](https://example.com)`,
	"```js\" onload=\"alert(1)\n<script>alert(1)</script>\n```",
	"`<script>alert(1)</script>`",
	`<style>body{display:none}</style>`,
	`<!-- comment --><p style="color:red">x</p>`,
	`[ref]: javascript:alert(1)` + "\n\n[ref]",
	`| a |` + "\n|---|\n| <script>alert(1)</script> |",
	`- [ ] <img src=x onerror=alert(1)>`,
	`@"><script>alert(1)</script>`,
	`infra-1"><script>alert(1)</script>`,
	`www.example.com/"onmouseover="alert(1)`,
	`https://example.com/<script>alert(1)</script>`,
}

var (
	liveTag    = regexp.MustCompile(`(?i)<\s*(script|img|svg|iframe|style|object|embed|form|input[^>]*type="(?:text|hidden)|math|base|meta|link)\b`)
	handler    = regexp.MustCompile(`(?i)<[^>]*\son[a-z]+\s*=`)
	styleAttr  = regexp.MustCompile(`(?i)<[^>]*\sstyle\s*=`)
	hrefAttr   = regexp.MustCompile(`href="([^"]*)"`)
	safeScheme = regexp.MustCompile(`^(https?://|mailto:|/tasks)`)
)

func renderings(src string) []string {
	opt := markdown.Options{
		TaskHref:   func(ref string) string { return "/tasks/" + ref },
		HandleHref: func(h string) string { return "/tasks?q=assignee:" + h },
	}
	static := string(markdown.Render(src, opt))
	opt.Interactive = true
	return []string{static, string(markdown.Render(src, opt))}
}

func TestHostileTextRendersInert(t *testing.T) {
	t.Parallel()
	for _, src := range xssCorpus {
		for _, out := range renderings(src) {
			if m := liveTag.FindString(out); m != "" {
				t.Errorf("%q rendered a live tag %q:\n%s", src, m, out)
			}
			if handler.MatchString(out) {
				t.Errorf("%q rendered an event handler:\n%s", src, out)
			}
			if styleAttr.MatchString(out) {
				t.Errorf("%q rendered a style attribute, which a strict policy refuses:\n%s", src, out)
			}
			for _, m := range hrefAttr.FindAllStringSubmatch(out, -1) {
				if !safeScheme.MatchString(m[1]) {
					t.Errorf("%q rendered the address %q:\n%s", src, m[1], out)
				}
			}
		}
	}
}

func TestRawHTMLIsShownAsTyped(t *testing.T) {
	t.Parallel()
	out := string(markdown.Render("say <b>hi</b>\n\n<div>block</div>", markdown.Options{}))
	for _, want := range []string{"say &lt;b&gt;hi&lt;/b&gt;", "&lt;div&gt;block&lt;/div&gt;"} {
		if !strings.Contains(out, want) {
			t.Errorf("raw HTML is not shown as text; want %q in:\n%s", want, out)
		}
	}
}

func TestOnlyWebAddressesBecomeLinks(t *testing.T) {
	t.Parallel()
	cases := []struct {
		src, href string
	}{
		{"[a](https://example.com/x?y=1)", `href="https://example.com/x?y=1"`},
		{"[a](http://example.com)", `href="http://example.com"`},
		{"[a](mailto:ops@example.com)", `href="mailto:ops@example.com"`},
		{"see https://example.com/p", `href="https://example.com/p"`},
		{"mail ops@example.com", `href="mailto:ops@example.com"`},
		{"[a](javascript:alert(1))", ""},
		{"[a](data:text/html,x)", ""},
		{"[a](/relative)", ""},
		{"[a](ftp://example.com)", ""},
	}
	for _, c := range cases {
		out := string(markdown.Render(c.src, markdown.Options{}))
		if c.href == "" {
			if strings.Contains(out, "<a ") {
				t.Errorf("%q became a link:\n%s", c.src, out)
			}
			continue
		}
		if !strings.Contains(out, "<a "+c.href+` rel="noopener noreferrer nofollow">`) {
			t.Errorf("%q: want a link with %s and the rel, got:\n%s", c.src, c.href, out)
		}
		if strings.Contains(out, "target=") {
			t.Errorf("%q: a link opens where the app's links open, with no target:\n%s", c.src, out)
		}
	}
}

func TestAnImageIsALinkNeverAFetch(t *testing.T) {
	t.Parallel()
	out := string(markdown.Render("![the chart](https://example.com/c.png)", markdown.Options{}))
	if strings.Contains(out, "<img") {
		t.Fatalf("an image was drawn, which makes every reader fetch from its origin:\n%s", out)
	}
	if !strings.Contains(out, `<a href="https://example.com/c.png" rel="noopener noreferrer nofollow" class="md-image">the chart</a>`) {
		t.Fatalf("an image is not a link to its address labelled by its alt text:\n%s", out)
	}
}

func TestTheGitHubExtensionsRender(t *testing.T) {
	t.Parallel()
	src := "| a | b |\n|:--|--:|\n| 1 | 2 |\n\n~~gone~~\n\n```go\nfmt.Println(1 < 2)\n```\n\n- [x] done\n- [ ] open\n"
	out := string(markdown.Render(src, markdown.Options{}))
	for _, want := range []string{
		`<th align="left">a</th>`, `<td align="right">2</td>`, "<del>gone</del>",
		`<pre><code class="language-go">fmt.Println(1 &lt; 2)`,
		`<li class="task-item"><input type="checkbox" disabled checked aria-label="done">`,
		`<input type="checkbox" disabled aria-label="open">`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("want %q in:\n%s", want, out)
		}
	}
}

func TestALineBreakIsKept(t *testing.T) {
	t.Parallel()
	out := string(markdown.Render("one\ntwo", markdown.Options{}))
	if !strings.Contains(out, "one<br>") {
		t.Fatalf("a line break typed in plain text was folded away:\n%s", out)
	}
}

func TestHeadingsSitUnderTheCardHeading(t *testing.T) {
	t.Parallel()
	out := string(markdown.Render("# one\n\n#### four", markdown.Options{}))
	if !strings.Contains(out, "<h3>one</h3>") || !strings.Contains(out, "<h6>four</h6>") {
		t.Fatalf("headings are not moved below the card's h2:\n%s", out)
	}
}

func TestReferencesAndHandlesLinkWhereTheyResolve(t *testing.T) {
	t.Parallel()
	opt := markdown.Options{
		TaskHref: func(ref string) string {
			if strings.HasPrefix(ref, "infra-") || strings.HasPrefix(ref, "web-app-") {
				return "/tasks/" + ref
			}
			return ""
		},
		HandleHref: func(h string) string {
			if h == "alice" {
				return "/tasks?q=assignee:alice"
			}
			return ""
		},
	}
	out := string(markdown.Render("see infra-8, web-app-3 and covid-19; ask @alice not @bob, "+
		"mail x@alice.com, `infra-9` [infra-10](https://e.com)", opt))
	for _, want := range []string{
		`<a href="/tasks/infra-8" class="taskref">infra-8</a>`,
		`<a href="/tasks/web-app-3" class="taskref">web-app-3</a>`,
		`<a href="/tasks?q=assignee:alice" class="mention">@alice</a>`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("want %q in:\n%s", want, out)
		}
	}
	for _, not := range []string{`/tasks/covid-19`, `assignee:bob`, `/tasks/infra-9`, `/tasks/infra-10`, `class="mention">@alice.com`} {
		if strings.Contains(out, not) {
			t.Errorf("%q should not be linked:\n%s", not, out)
		}
	}
}

func TestInteractiveItemsAreNumberedButtons(t *testing.T) {
	t.Parallel()
	src := "- [ ] first\n- [x] second\n  - [ ] third *em*\n"
	out := string(markdown.Render(src, markdown.Options{Interactive: true, IDPrefix: "d-"}))
	for _, want := range []string{
		`<button type="submit" class="check" id="d-check-1" role="checkbox" aria-checked="false" name="check" value="1" aria-label="first"></button>`,
		`<button type="submit" class="check" id="d-check-2" role="checkbox" aria-checked="true" name="uncheck" value="2" aria-label="second"></button>`,
		`id="d-check-3" role="checkbox" aria-checked="false" name="check" value="3" aria-label="third em"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("want %q in:\n%s", want, out)
		}
	}
}
