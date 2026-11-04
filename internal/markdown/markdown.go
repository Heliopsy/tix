// SPDX-License-Identifier: AGPL-3.0-or-later

// Package markdown turns task descriptions and comments into safe HTML and
// reads and ticks the checklist a description carries.
//
// The dialect is CommonMark with the GitHub tables, task lists, autolinks and
// strikethrough. Raw HTML is never passed through: it is shown as the text that
// was typed. A link survives only when it is http, https or mailto, and an
// image is never loaded: it becomes a link to its address, so a description
// cannot make every reader's browser fetch from an origin of its choosing. The
// output carries no inline style or script, so it holds under a strict content
// security policy.
package markdown

import (
	"bytes"
	"html/template"
	"io"
	"net/url"
	"strconv"
	"strings"

	"github.com/yuin/goldmark/v2/ast"
	"github.com/yuin/goldmark/v2/extension"
	"github.com/yuin/goldmark/v2/parser"
	"github.com/yuin/goldmark/v2/renderer"
	"github.com/yuin/goldmark/v2/renderer/html"
	"github.com/yuin/goldmark/v2/util"
)

// Rel is the relationship every rendered link carries.
const Rel = "noopener noreferrer nofollow"

// Options say how a rendering links references and draws checklist items.
type Options struct {
	// TaskHref returns the address of the task a reference such as infra-8
	// names, or "" to leave the reference as text.
	TaskHref func(ref string) string
	// HandleHref returns the address an @handle links to, or "" to leave it
	// as text.
	HandleHref func(handle string) string
	// Interactive draws checklist items as submit buttons named check or
	// uncheck whose value is the item's number, for a form around the
	// rendering to post. Otherwise they are disabled checkboxes.
	Interactive bool
	// IDPrefix leads the id of each interactive item, so two renderings on
	// one page do not collide.
	IDPrefix string
}

// md is the one parser. Render builds a renderer per call because numbering
// checklist items is state.
var md = parser.New(parser.WithExtensions(
	extension.NewTableParser(), extension.NewStrikethroughParser(),
	extension.NewTaskListItemParser(), extension.NewLinkifyParser(),
))

func parse(src []byte) ast.Node {
	return md.Parse(src)
}

// Render returns the HTML for src. It never fails: a rendering error leaves
// the source escaped in a paragraph rather than losing the text.
func Render(src string, opt Options) template.HTML {
	if strings.TrimSpace(src) == "" {
		return ""
	}
	source := []byte(src)
	doc := parse(source)
	linkRefs(doc, source, opt)
	s := &safeRenderer{opt: opt}
	// Cell alignment goes in an attribute, not a style, which a policy
	// without unsafe-inline styles would refuse.
	r := html.New(
		html.WithHardWraps(),
		html.WithExtensions(
			extension.NewTableHTMLRenderer(extension.WithTableCellAlignMethod(extension.TableCellAlignAttribute)),
			extension.NewStrikethroughHTMLRenderer(),
		),
		html.WithNodeRenderers(s.renderers()),
	)
	var buf bytes.Buffer
	if err := r.Render(&buf, source, doc); err != nil {
		return template.HTML("<p>" + template.HTMLEscapeString(src) + "</p>") // #nosec G203 -- escaped above
	}
	return template.HTML(buf.String()) // #nosec G203 -- every node is rendered by safeRenderer or goldmark without unsafe
}

// SafeHref returns dest as an attribute-ready address when it is an absolute
// http, https or mailto URL, and reports false for anything else.
func SafeHref(dest []byte) (string, bool) {
	escaped := urlEscape(dest)
	u, err := url.Parse(escaped)
	if err != nil {
		return "", false
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https":
		if u.Host == "" {
			return "", false
		}
	case "mailto":
		if u.Opaque == "" {
			return "", false
		}
	default:
		return "", false
	}
	return escaped, true
}

// urlEscape percent-encodes dest. goldmark's URLEscape also writes & as
// &amp;, which is HTML escaping; that is undone here so the address parses as
// a URL and is escaped once, where it is written into the attribute.
func urlEscape(dest []byte) string {
	return strings.ReplaceAll(string(util.URLEscape(dest)), "&amp;", "&")
}

// safeRenderer draws the nodes whose default rendering is unsafe or wrong for
// tix: raw HTML, links, images, checklist items and headings.
type safeRenderer struct {
	opt   Options
	items int
}

// renderers claims the node kinds this renderer draws.
func (r *safeRenderer) renderers() map[ast.NodeKind]html.NodeRenderer {
	return map[ast.NodeKind]html.NodeRenderer{
		ast.KindRawHTML:   html.NodeRendererFunc(r.rawHTML),
		ast.KindHTMLBlock: html.NodeRendererFunc(r.htmlBlock),
		ast.KindLink:      html.NodeRendererFunc(r.link),
		ast.KindAutoLink:  html.NodeRendererFunc(r.autoLink),
		ast.KindImage:     html.NodeRendererFunc(r.image),
		ast.KindHeading:   html.NodeRendererFunc(r.heading),
		ast.KindListItem:  html.NodeRendererFunc(r.listItem),
		ast.KindParagraph: html.NodeRendererFunc(r.paragraph),
		kindRef:           html.NodeRendererFunc(r.ref),
	}
}

func (r *safeRenderer) rawHTML(w io.Writer, src []byte, node ast.Node, entering bool, _ renderer.Context) (ast.WalkStatus, error) {
	if entering {
		_, _ = w.Write(util.EscapeHTML(node.(*ast.RawHTML).Value.Bytes(src)))
	}
	return ast.WalkSkipChildren, nil
}

func (r *safeRenderer) htmlBlock(w io.Writer, src []byte, node ast.Node, entering bool, _ renderer.Context) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	n := node.(*ast.HTMLBlock)
	_, _ = io.WriteString(w, `<p class="md-html">`)
	_, _ = w.Write(util.EscapeHTML(bytes.TrimRight(n.Value.Bytes(src), "\r\n")))
	_, _ = io.WriteString(w, "</p>\n")
	return ast.WalkSkipChildren, nil
}

func writeAnchor(w io.Writer, href string, class string) {
	_, _ = io.WriteString(w, `<a href="`)
	_, _ = w.Write(util.EscapeHTML([]byte(href)))
	_, _ = io.WriteString(w, `" rel="`+Rel+`"`)
	if class != "" {
		_, _ = io.WriteString(w, ` class="`+class+`"`)
	}
	_, _ = io.WriteString(w, ">")
}

func (r *safeRenderer) link(w io.Writer, src []byte, node ast.Node, entering bool, _ renderer.Context) (ast.WalkStatus, error) {
	n := node.(*ast.Link)
	href, ok := SafeHref([]byte(n.Destination.Value(src)))
	if !ok {
		return ast.WalkContinue, nil
	}
	if entering {
		writeAnchor(w, href, "")
	} else {
		_, _ = io.WriteString(w, "</a>")
	}
	return ast.WalkContinue, nil
}

func (r *safeRenderer) autoLink(w io.Writer, src []byte, node ast.Node, entering bool, _ renderer.Context) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	n := node.(*ast.AutoLink)
	dest, label := n.Destination.Value(src), n.Label.Value(src)
	if !strings.Contains(dest, ":") {
		dest = "https://" + dest
	}
	href, ok := SafeHref([]byte(dest))
	if ok {
		writeAnchor(w, href, "")
	}
	_, _ = w.Write(util.EscapeHTML([]byte(label)))
	if ok {
		_, _ = io.WriteString(w, "</a>")
	}
	return ast.WalkContinue, nil
}

// image never loads the picture. It links to the address, labelled by the alt
// text, and inside a link it is only the alt text, since anchors do not nest.
func (r *safeRenderer) image(w io.Writer, src []byte, node ast.Node, entering bool, _ renderer.Context) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkSkipChildren, nil
	}
	n := node.(*ast.Image)
	alt := plainText(n, src)
	href, ok := SafeHref([]byte(n.Destination.Value(src)))
	if ok && !insideLink(n) {
		if alt == "" {
			alt = href
		}
		writeAnchor(w, href, "md-image")
		_, _ = w.Write(util.EscapeHTML([]byte(alt)))
		_, _ = io.WriteString(w, "</a>")
		return ast.WalkSkipChildren, nil
	}
	_, _ = w.Write(util.EscapeHTML([]byte(alt)))
	return ast.WalkSkipChildren, nil
}

func insideLink(n ast.Node) bool {
	for p := n.Parent(); p != nil; p = p.Parent() {
		if p.Kind() == ast.KindLink {
			return true
		}
	}
	return false
}

// heading sits two levels down, under the h2 of the card the text is in.
func (r *safeRenderer) heading(w io.Writer, _ []byte, node ast.Node, entering bool, _ renderer.Context) (ast.WalkStatus, error) {
	tag := "h" + strconv.Itoa(min(node.(*ast.Heading).Level+2, 6))
	if entering {
		_, _ = io.WriteString(w, "<"+tag+">")
	} else {
		_, _ = io.WriteString(w, "</"+tag+">\n")
	}
	return ast.WalkContinue, nil
}

func (r *safeRenderer) listItem(w io.Writer, _ []byte, node ast.Node, entering bool, _ renderer.Context) (ast.WalkStatus, error) {
	if !entering {
		_, _ = io.WriteString(w, "</li>\n")
		return ast.WalkContinue, nil
	}
	if extension.IsTask(node) {
		_, _ = io.WriteString(w, `<li class="task-item">`)
	} else {
		_, _ = io.WriteString(w, "<li>")
	}
	if fc := node.FirstChild(); fc != nil {
		if p, ok := fc.(*ast.Paragraph); !ok || !html.IsInTightBlock(p) {
			_, _ = io.WriteString(w, "\n")
		}
	}
	return ast.WalkContinue, nil
}

// paragraph is goldmark's paragraph with the checklist box drawn at the start
// of the one that opens a checklist item.
func (r *safeRenderer) paragraph(w io.Writer, src []byte, node ast.Node, entering bool, _ renderer.Context) (ast.WalkStatus, error) {
	tight := html.IsInTightBlock(node)
	if !entering {
		switch {
		case !tight:
			_, _ = io.WriteString(w, "</p>\n")
		case node.NextSibling() != nil && node.FirstChild() != nil:
			_, _ = io.WriteString(w, "\n")
		}
		return ast.WalkContinue, nil
	}
	if !tight {
		_, _ = io.WriteString(w, "<p>")
	}
	if item := node.Parent(); item != nil && item.FirstChild() == node {
		if status, ok := extension.TaskStatusOf(item); ok {
			r.checkBox(w, src, node, status == extension.TaskStatusCompleted)
		}
	}
	return ast.WalkContinue, nil
}

func (r *safeRenderer) checkBox(w io.Writer, src []byte, para ast.Node, checked bool) {
	r.items++
	label := itemLabel(para, src)
	if !r.opt.Interactive {
		_, _ = io.WriteString(w, `<input type="checkbox" disabled`)
		if checked {
			_, _ = io.WriteString(w, " checked")
		}
		_, _ = io.WriteString(w, ` aria-label="`)
		_, _ = w.Write(util.EscapeHTML([]byte(label)))
		_, _ = io.WriteString(w, `"> `)
		return
	}
	name, state := "check", "false"
	if checked {
		name, state = "uncheck", "true"
	}
	num := strconv.Itoa(r.items)
	_, _ = io.WriteString(w, `<button type="submit" class="check" id="`)
	_, _ = w.Write(util.EscapeHTML([]byte(r.opt.IDPrefix)))
	_, _ = io.WriteString(w, `check-`+num+`" role="checkbox" aria-checked="`+state+
		`" name="`+name+`" value="`+num+`" aria-label="`)
	_, _ = w.Write(util.EscapeHTML([]byte(label)))
	_, _ = io.WriteString(w, `"></button> `)
}

// itemLabel is the text of a checklist item, which names its checkbox.
func itemLabel(para ast.Node, src []byte) string {
	label := strings.Join(strings.Fields(plainText(para, src)), " ")
	if r := []rune(label); len(r) > 200 {
		label = string(r[:200])
	}
	return label
}

func plainText(n ast.Node, src []byte) string {
	var b strings.Builder
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		b.WriteString(nodeText(c, src))
	}
	return b.String()
}

func nodeText(n ast.Node, src []byte) string {
	switch v := n.(type) {
	case *ast.Text:
		s := v.Value.Value(src)
		if v.SoftLineBreak() || v.HardLineBreak() {
			s += " "
		}
		return s
	case *ast.CodeSpan:
		return v.Value.Value(src)
	case *ast.AutoLink:
		return v.Label.Value(src)
	case *refNode:
		return v.label
	}
	return plainText(n, src)
}
