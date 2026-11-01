// SPDX-License-Identifier: AGPL-3.0-or-later

package markdown

import (
	"regexp"
	"strings"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// kindRef is the node a linked task reference or handle becomes.
var kindRef = ast.NewNodeKind("TixRef")

// refNode is a task reference or @handle that resolved to an address.
type refNode struct {
	ast.BaseInline
	label, href, class string
}

// Kind reports the node kind.
func (n *refNode) Kind() ast.NodeKind { return kindRef }

// Dump writes the node for goldmark's debugging output.
func (n *refNode) Dump(src []byte, level int) {
	ast.DumpHelper(n, src, level, map[string]string{"label": n.label, "href": n.href}, nil)
}

// refPattern finds a task reference or a handle. A reference is a project key
// and a number; the key may itself contain hyphens, so the last one splits it.
// The leading group keeps the match off the middle of a word, an address or
// an email.
var refPattern = regexp.MustCompile(`(^|[^\w@./-])(@[A-Za-z0-9][A-Za-z0-9_.-]*[A-Za-z0-9]|@[A-Za-z0-9]|[A-Za-z][A-Za-z0-9_-]*-[0-9]+)\b`)

// linkRefs replaces the references and handles in the document's text with
// links, wherever the options resolve them. Code and existing links keep
// their text.
func linkRefs(doc ast.Node, src []byte, opt Options) {
	if opt.TaskHref == nil && opt.HandleHref == nil {
		return
	}
	var texts []*ast.Text
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch n.Kind() {
		case ast.KindLink, ast.KindAutoLink, ast.KindImage, ast.KindCodeSpan,
			ast.KindCodeBlock, ast.KindFencedCodeBlock, ast.KindHTMLBlock, ast.KindRawHTML:
			return ast.WalkSkipChildren, nil
		}
		if t, ok := n.(*ast.Text); ok && !t.IsRaw() {
			texts = append(texts, t)
		}
		return ast.WalkContinue, nil
	})
	for _, t := range texts {
		splitRefs(t, src, opt)
	}
}

func splitRefs(t *ast.Text, src []byte, opt Options) {
	seg := t.Segment
	value := src[seg.Start:seg.Stop]
	parent := t.Parent()
	start := 0
	for _, m := range refPattern.FindAllSubmatchIndex(value, -1) {
		lo, hi := m[4], m[5]
		token := string(value[lo:hi])
		href, class := resolve(token, opt)
		if href == "" {
			continue
		}
		if lo > start {
			parent.InsertBefore(parent, t, ast.NewTextSegment(text.NewSegment(seg.Start+start, seg.Start+lo)))
		}
		parent.InsertBefore(parent, t, &refNode{label: token, href: href, class: class})
		start = hi
	}
	if start > 0 {
		t.Segment = text.NewSegment(seg.Start+start, seg.Stop)
	}
}

func resolve(token string, opt Options) (string, string) {
	if handle, ok := strings.CutPrefix(token, "@"); ok {
		if opt.HandleHref == nil {
			return "", ""
		}
		return opt.HandleHref(handle), "mention"
	}
	if opt.TaskHref == nil {
		return "", ""
	}
	return opt.TaskHref(token), "taskref"
}

func (r *safeRenderer) ref(w util.BufWriter, _ []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	n := node.(*refNode)
	_, _ = w.WriteString(`<a href="`)
	_, _ = w.Write(util.EscapeHTML(util.URLEscape([]byte(n.href), false)))
	_, _ = w.WriteString(`" class="` + n.class + `">`)
	_, _ = w.Write(util.EscapeHTML([]byte(n.label)))
	_, _ = w.WriteString("</a>")
	return ast.WalkContinue, nil
}

var _ renderer.NodeRenderer = (*safeRenderer)(nil)
