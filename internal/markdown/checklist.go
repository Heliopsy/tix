// SPDX-License-Identifier: AGPL-3.0-or-later

package markdown

import (
	"errors"
	"strings"

	"github.com/yuin/goldmark/ast"
	extast "github.com/yuin/goldmark/extension/ast"
)

// ErrNoItem reports a checklist item number the text does not have.
var ErrNoItem = errors.New("no such checklist item")

// boxes returns the checklist items of a parsed document in reading order,
// which is the order Render numbers them in.
func boxes(doc ast.Node) []*extast.TaskCheckBox {
	var out []*extast.TaskCheckBox
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if box, ok := n.(*extast.TaskCheckBox); ok && entering {
			out = append(out, box)
		}
		return ast.WalkContinue, nil
	})
	return out
}

// Checklist counts the ticked and the total checklist items in src.
func Checklist(src string) (done, total int) {
	if !strings.Contains(src, "[") {
		return 0, 0
	}
	for _, box := range boxes(parse([]byte(src))) {
		total++
		if box.IsChecked {
			done++
		}
	}
	return done, total
}

// Items reports, item by item in reading order, whether each checklist item
// of src is ticked.
func Items(src string) []bool {
	list := boxes(parse([]byte(src)))
	out := make([]bool, len(list))
	for i, box := range list {
		out[i] = box.IsChecked
	}
	return out
}

// SetItem ticks or clears checklist item n, counted from one, and returns the
// text with only that marker changed. An item already in the asked-for state
// returns src unchanged.
func SetItem(src string, n int, done bool) (string, error) {
	source := []byte(src)
	list := boxes(parse(source))
	if n < 1 || n > len(list) {
		return "", ErrNoItem
	}
	box := list[n-1]
	if box.IsChecked == done {
		return src, nil
	}
	block := box.Parent()
	if block == nil || block.Lines().Len() == 0 {
		return "", ErrNoItem
	}
	at := block.Lines().At(0).Start
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
