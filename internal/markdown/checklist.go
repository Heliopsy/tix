// SPDX-License-Identifier: AGPL-3.0-or-later

package markdown

import (
	"errors"
	"hash/maphash"
	"strings"
	"sync"

	"github.com/yuin/goldmark/v2/ast"
	"github.com/yuin/goldmark/v2/extension"
	"github.com/yuin/goldmark/v2/parser"
	"github.com/yuin/goldmark/v2/util"
)

// boxParser finds checklist items without parsing the rest of the inline
// text. Which list items are checklist items is settled by the block
// structure, link reference definitions and tables, and the checkbox at the
// start of an item, all of which it shares with md. Links, emphasis and code
// spans cannot move that checkbox, and parsing them was nearly all of what a
// task listing spent: counting the checklists of 100 imported descriptions of
// 5 to 50 KB allocated about 90 MB a page.
var boxParser = parser.New(
	parser.WithDefaultParsers(false),
	parser.WithBlockParsers(
		util.Prioritized(parser.NewSetextHeadingParser(), 100),
		util.Prioritized(parser.NewThematicBreakParser(), 200),
		util.Prioritized(parser.NewListParser(), 300),
		util.Prioritized(parser.NewListItemParser(), 400),
		util.Prioritized(parser.NewCodeBlockParser(), 500),
		util.Prioritized(parser.NewATXHeadingParser(), 600),
		util.Prioritized(parser.NewFencedCodeBlockParser(), 700),
		util.Prioritized(parser.NewBlockquoteParser(), 800),
		util.Prioritized(parser.NewHTMLBlockParser(), 900),
		util.Prioritized(parser.NewParagraphParser(), 1000),
	),
	parser.WithParagraphTransformers(util.Prioritized(parser.LinkReferenceParagraphTransformer, 100)),
	parser.WithExtensions(extension.NewTableParser(), extension.NewTaskListItemParser()),
)

// mayHaveBox reports whether src holds anything a checkbox could be: an
// opening bracket, a space or an x, and a closing bracket. Text without one
// has no checklist, however many links it carries.
func mayHaveBox(src string) bool {
	for i := strings.IndexByte(src, '['); i >= 0 && i+2 < len(src); {
		if src[i+2] == ']' && (util.IsSpace(src[i+1]) || src[i+1] == 'x' || src[i+1] == 'X') {
			return true
		}
		next := strings.IndexByte(src[i+1:], '[')
		if next < 0 {
			return false
		}
		i += next + 1
	}
	return false
}

// listBoxes returns the checklist items of src, parsing only what decides
// them.
func listBoxes(src []byte) []*ast.ListItem {
	return boxes(boxParser.Parse(src))
}

// ErrNoItem reports a checklist item number the text does not have.
var ErrNoItem = errors.New("no such checklist item")

// boxes returns the checklist items of a parsed document in reading order,
// which is the order Render numbers them in.
func boxes(doc ast.Node) []*ast.ListItem {
	var out []*ast.ListItem
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if item, ok := n.(*ast.ListItem); ok && entering && extension.IsTask(item) {
			out = append(out, item)
		}
		return ast.WalkContinue, nil
	})
	return out
}

func checked(item *ast.ListItem) bool {
	status, _ := extension.TaskStatusOf(item)
	return status == extension.TaskStatusCompleted
}

// Checklist counts the ticked and the total checklist items in src.
func Checklist(src string) (done, total int) {
	if !mayHaveBox(src) {
		return 0, 0
	}
	if len(src) < countCacheMin {
		return countBoxes(src)
	}
	key := maphash.String(countSeed, src)
	if c, ok := counts.get(key); ok {
		return c[0], c[1]
	}
	done, total = countBoxes(src)
	counts.put(key, [2]int{done, total})
	return done, total
}

func countBoxes(src string) (done, total int) {
	for _, box := range listBoxes([]byte(src)) {
		total++
		if checked(box) {
			done++
		}
	}
	return done, total
}

// Every listed task has its checklist counted, and every screen lists some:
// the sidebar's overdue badge alone counts up to a hundred on each page.
// Parsing a long description costs many times its size, so the counts of long
// texts are kept, by a hash of the text, in a cache that empties when full.
const (
	countCacheMin  = 4 << 10
	countCacheSize = 8192
)

var (
	countSeed = maphash.MakeSeed()
	counts    = countCache{m: make(map[uint64][2]int)}
)

// countCache holds checklist counts by text hash, at most countCacheSize.
type countCache struct {
	mu sync.Mutex
	m  map[uint64][2]int
}

func (c *countCache) get(key uint64) ([2]int, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.m[key]
	return v, ok
}

func (c *countCache) put(key uint64, v [2]int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.m) >= countCacheSize {
		clear(c.m)
	}
	c.m[key] = v
}

// Items reports, item by item in reading order, whether each checklist item
// of src is ticked.
func Items(src string) []bool {
	if !mayHaveBox(src) {
		return []bool{}
	}
	list := listBoxes([]byte(src))
	out := make([]bool, len(list))
	for i, box := range list {
		out[i] = checked(box)
	}
	return out
}

// SetItem ticks or clears checklist item n, counted from one, and returns the
// text with only that marker changed. An item already in the asked-for state
// returns src unchanged.
func SetItem(src string, n int, done bool) (string, error) {
	source := []byte(src)
	list := listBoxes(source)
	if n < 1 || n > len(list) {
		return "", ErrNoItem
	}
	box := list[n-1]
	if checked(box) == done {
		return src, nil
	}
	block, ok := box.FirstChild().(ast.BlockNode)
	if !ok || len(block.Source()) == 0 {
		return "", ErrNoItem
	}
	at := block.Source()[0].Start
	for at < len(source) && source[at] != '[' && (source[at] == ' ' || source[at] == '\t') {
		at++
	}
	if at+2 >= len(source) || source[at] != '[' || source[at+2] != ']' {
		return "", ErrNoItem
	}
	mark := byte(' ')
	if done {
		mark = 'x'
	}
	source[at+1] = mark
	return string(source), nil
}
