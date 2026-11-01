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
	"net/url"
	"strconv"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	extast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
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

// tableAlign keeps cell alignment out of style attributes, which a policy
// without unsafe-inline styles would refuse.
var tableAlign = extension.WithTableCellAlignMethod(extension.TableCellAlignAttribute)

// md is the one parser. Its renderer is never used: Render builds one per call
// because numbering checklist items is state.
var md = goldmark.New(goldmark.WithExtensions(
	extension.NewTable(tableAlign), extension.Strikethrough, extension.TaskList, extension.Linkify,
))

func parse(src []byte) ast.Node {
	return md.Parser().Parse(text.NewReader(src))
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
	r := renderer.NewRenderer(renderer.WithNodeRenderers(
		util.Prioritized(html.NewRenderer(html.WithHardWraps()), 1000),
		util.Prioritized(extension.NewTableHTMLRenderer(tableAlign), 500),
		util.Prioritized(extension.NewStrikethroughHTMLRenderer(), 500),
		util.Prioritized(&safeRenderer{opt: opt}, 100),
	))
	var buf bytes.Buffer
	if err := r.Render(&buf, source, doc); err != nil {
		return template.HTML("<p>" + template.HTMLEscapeString(src) + "</p>") // #nosec G203 -- escaped above
	}
	return template.HTML(buf.String()) // #nosec G203 -- every node is rendered by safeRenderer or goldmark without unsafe
}

// SafeHref returns dest as an attribute-ready address when it is an absolute
// http, https or mailto URL, and reports false for anything else.
func SafeHref(dest []byte) (string, bool) {
	escaped := string(util.URLEscape(dest, true))
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

// safeRenderer draws the nodes whose default rendering is unsafe or wrong for
// tix: raw HTML, links, images, checklist items and headings.
type safeRenderer struct {
	opt   Options
	items int
}

// RegisterFuncs claims the node kinds this renderer draws.
func (r *safeRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindRawHTML, r.rawHTML)
	reg.Register(ast.KindHTMLBlock, r.htmlBlock)
	reg.Register(ast.KindLink, r.link)
	reg.Register(ast.KindAutoLink, r.autoLink)
	reg.Register(ast.KindImage, r.image)
	reg.Register(ast.KindHeading, r.heading)
	reg.Register(ast.KindListItem, r.listItem)
	reg.Register(extast.KindTaskCheckBox, r.checkBox)
	reg.Register(kindRef, r.ref)
}

func (r *safeRenderer) rawHTML(w util.BufWriter, src []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkSkipChildren, nil
	}
	segs := node.(*ast.RawHTML).Segments
	for i := range segs.Len() {
		seg := segs.At(i)
		_, _ = w.Write(util.EscapeHTML(seg.Value(src)))
	}
	return ast.WalkSkipChildren, nil
}

func (r *safeRenderer) htmlBlock(w util.BufWriter, src []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	n := node.(*ast.HTMLBlock)
	if !entering {
		return ast.WalkContinue, nil
	}
	var buf bytes.Buffer
	for i := range n.Lines().Len() {
		line := n.Lines().At(i)
		buf.Write(line.Value(src))
	}
	if n.HasClosure() {
		buf.Write(n.ClosureLine.Value(src))
	}
	_, _ = w.WriteString(`<p class="md-html">`)
	_, _ = w.Write(util.EscapeHTML(bytes.TrimRight(buf.Bytes(), "\r\n")))
	_, _ = w.WriteString("</p>\n")
	return ast.WalkSkipChildren, nil
}

func writeAnchor(w util.BufWriter, href string, class string) {
	_, _ = w.WriteString(`<a href="`)
	_, _ = w.Write(util.EscapeHTML([]byte(href)))
	_, _ = w.WriteString(`" rel="` + Rel + `"`)
	if class != "" {
		_, _ = w.WriteString(` class="` + class + `"`)
	}
	_ = w.WriteByte('>')
}

func (r *safeRenderer) link(w util.BufWriter, _ []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	n := node.(*ast.Link)
	href, ok := SafeHref(n.Destination)
	if !ok {
		return ast.WalkContinue, nil
	}
	if entering {
		writeAnchor(w, href, "")
	} else {
		_, _ = w.WriteString("</a>")
	}
	return ast.WalkContinue, nil
}

func (r *safeRenderer) autoLink(w util.BufWriter, src []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	n := node.(*ast.AutoLink)
	dest, label := n.URL(src), n.Label(src)
	lower := bytes.ToLower(dest)
	switch {
	case n.AutoLinkType == ast.AutoLinkEmail && !bytes.HasPrefix(lower, []byte("mailto:")):
		dest = append([]byte("mailto:"), dest...)
	case n.AutoLinkType == ast.AutoLinkURL && !bytes.Contains(lower, []byte(":")):
		dest = append([]byte("https://"), dest...)
	}
	href, ok := SafeHref(dest)
	if ok {
		writeAnchor(w, href, "")
	}
	_, _ = w.Write(util.EscapeHTML(label))
	if ok {
		_, _ = w.WriteString("</a>")
	}
	return ast.WalkContinue, nil
}

// image never loads the picture. It links to the address, labelled by the alt
// text, and inside a link it is only the alt text, since anchors do not nest.
func (r *safeRenderer) image(w util.BufWriter, src []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkSkipChildren, nil
	}
	n := node.(*ast.Image)
	alt := plainText(n, src)
	href, ok := SafeHref(n.Destination)
	if ok && !insideLink(n) {
		if alt == "" {
			alt = href
		}
		writeAnchor(w, href, "md-image")
		_, _ = w.Write(util.EscapeHTML([]byte(alt)))
		_, _ = w.WriteString("</a>")
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
func (r *safeRenderer) heading(w util.BufWriter, _ []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	level := min(node.(*ast.Heading).Level+2, 6)
	tag := []byte{'h', byte('0' + level)}
	if entering {
		_, _ = w.WriteString("<" + string(tag) + ">")
	} else {
		_, _ = w.WriteString("</" + string(tag) + ">\n")
	}
	return ast.WalkContinue, nil
}

func (r *safeRenderer) listItem(w util.BufWriter, _ []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		_, _ = w.WriteString("</li>\n")
		return ast.WalkContinue, nil
	}
	if first := node.FirstChild(); first != nil && first.FirstChild() != nil &&
		first.FirstChild().Kind() == extast.KindTaskCheckBox {
		_, _ = w.WriteString(`<li class="task-item">`)
	} else {
		_, _ = w.WriteString("<li>")
	}
	if fc := node.FirstChild(); fc != nil {
		if _, ok := fc.(*ast.TextBlock); !ok {
			_ = w.WriteByte('\n')
		}
	}
	return ast.WalkContinue, nil
}

func (r *safeRenderer) checkBox(w util.BufWriter, src []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	n := node.(*extast.TaskCheckBox)
	r.items++
	label := itemLabel(n, src)
	if !r.opt.Interactive {
		_, _ = w.WriteString(`<input type="checkbox" disabled`)
		if n.IsChecked {
			_, _ = w.WriteString(" checked")
		}
		_, _ = w.WriteString(` aria-label="`)
		_, _ = w.Write(util.EscapeHTML([]byte(label)))
		_, _ = w.WriteString(`"> `)
		return ast.WalkContinue, nil
	}
	name, state := "check", "false"
	if n.IsChecked {
		name, state = "uncheck", "true"
	}
	num := strconv.Itoa(r.items)
	_, _ = w.WriteString(`<button type="submit" class="check" id="`)
	_, _ = w.Write(util.EscapeHTML([]byte(r.opt.IDPrefix)))
	_, _ = w.WriteString(`check-` + num + `" role="checkbox" aria-checked="` + state +
		`" name="` + name + `" value="` + num + `" aria-label="`)
	_, _ = w.Write(util.EscapeHTML([]byte(label)))
	_, _ = w.WriteString(`"></button> `)
	return ast.WalkContinue, nil
}

// itemLabel is the text of a checklist item, which names its checkbox.
func itemLabel(box ast.Node, src []byte) string {
	var b strings.Builder
	for s := box.NextSibling(); s != nil; s = s.NextSibling() {
		b.WriteString(nodeText(s, src))
	}
	label := strings.Join(strings.Fields(b.String()), " ")
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
		s := string(v.Value(src))
		if v.SoftLineBreak() || v.HardLineBreak() {
			s += " "
		}
		return s
	case *ast.String:
		return string(v.Value)
	case *ast.AutoLink:
		return string(v.Label(src))
	case *refNode:
		return v.label
	}
	return plainText(n, src)
}
