// SPDX-License-Identifier: AGPL-3.0-or-later

package taskio

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"

	"github.com/heliopsy/tix/internal/core"
)

const headerSample = 200

type object struct {
	keys []string
	vals map[string]any
}

func decodeValue(dec *json.Decoder) (any, error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	return decodeFrom(dec, tok)
}

func decodeFrom(dec *json.Decoder, tok json.Token) (any, error) {
	switch tok {
	case json.Delim('{'):
		obj := &object{vals: map[string]any{}}
		for dec.More() {
			kt, err := dec.Token()
			if err != nil {
				return nil, err
			}
			key, _ := kt.(string)
			v, err := decodeValue(dec)
			if err != nil {
				return nil, err
			}
			if _, dup := obj.vals[key]; !dup {
				obj.keys = append(obj.keys, key)
			}
			obj.vals[key] = v
		}
		_, err := dec.Token()
		return obj, err
	case json.Delim('['):
		var arr []any
		for dec.More() {
			v, err := decodeValue(dec)
			if err != nil {
				return nil, err
			}
			arr = append(arr, v)
		}
		_, err := dec.Token()
		return arr, err
	}
	return tok, nil
}

type flatRow struct {
	keys []string
	vals map[string]string
}

func flatten(obj *object) flatRow {
	out := flatRow{vals: map[string]string{}}
	flattenInto(&out, "", obj)
	return out
}

func flattenInto(out *flatRow, prefix string, obj *object) {
	for _, k := range obj.keys {
		name := k
		if prefix != "" {
			name = prefix + "." + k
		}
		if nested, ok := obj.vals[k].(*object); ok {
			flattenInto(out, name, nested)
			continue
		}
		if _, dup := out.vals[name]; !dup {
			out.keys = append(out.keys, name)
		}
		out.vals[name] = cellText(obj.vals[k])
	}
}

var objectNameKeys = []string{"name", "login", "title", "key", "displayName"}

func cellText(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case json.Number:
		return t.String()
	case bool:
		if t {
			return "true"
		}
		return "false"
	case []any:
		parts := make([]string, 0, len(t))
		for _, e := range t {
			if s := elementText(e); s != "" {
				parts = append(parts, s)
			}
		}
		return strings.Join(parts, ", ")
	}
	return ""
}

func elementText(v any) string {
	obj, ok := v.(*object)
	if !ok {
		if _, nested := v.([]any); nested {
			return ""
		}
		return cellText(v)
	}
	for _, k := range objectNameKeys {
		if s := cellText(obj.vals[k]); s != "" {
			return s
		}
	}
	return ""
}

type recordSource interface {
	next() (line int, obj *object, err error)
}

type jsonReader struct {
	src     recordSource
	capped  *cappedReader
	limits  Limits
	headers []string
	pending []Row
	rows    int
	source  string
	err     error
}

func newJSONReader(src recordSource, capped *cappedReader, limits Limits) (Reader, error) {
	r := &jsonReader{src: src, capped: capped, limits: limits}
	seen := map[string]bool{}
	for len(r.pending) < headerSample {
		row, keys, err := r.read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		for _, k := range keys {
			if !seen[k] {
				seen[k] = true
				r.headers = append(r.headers, k)
			}
		}
		if len(r.headers) > limits.MaxColumns {
			return nil, limits.columnsError()
		}
		r.pending = append(r.pending, row)
	}
	if len(r.pending) == 0 {
		return nil, core.Invalid("the file holds no tasks")
	}
	for i := range r.pending {
		r.pending[i].Headers = r.headers
	}
	r.source = DetectSource(r.headers)
	return r, nil
}

func (r *jsonReader) read() (Row, []string, error) {
	line, obj, err := r.src.next()
	if r.capped.over() {
		return Row{}, nil, r.capped.err
	}
	if errors.Is(err, io.EOF) {
		return Row{}, nil, io.EOF
	}
	if err != nil {
		var domain *core.Error
		if errors.As(err, &domain) {
			return Row{}, nil, domain
		}
		return Row{}, nil, core.Invalid("reading JSON: %v", err)
	}
	r.rows++
	if r.rows > r.limits.MaxRows {
		return Row{}, nil, r.limits.rowsError()
	}
	flat := flatten(obj)
	if len(flat.keys) > r.limits.MaxColumns {
		return Row{}, nil, r.limits.columnsError()
	}
	for _, k := range flat.keys {
		if len(flat.vals[k]) > r.limits.MaxCell {
			return Row{}, nil, r.limits.cellError(line, k)
		}
	}
	return Row{Line: line, Values: flat.vals, Headers: r.headers}, flat.keys, nil
}

func (r *jsonReader) Headers() []string { return r.headers }

func (r *jsonReader) Source() string { return r.source }

func (r *jsonReader) Next() (Row, error) {
	if len(r.pending) > 0 {
		row := r.pending[0]
		r.pending = r.pending[1:]
		return row, nil
	}
	row, _, err := r.read()
	return row, err
}

type arraySource struct {
	dec   *json.Decoder
	index int
	done  bool
}

func (a *arraySource) next() (int, *object, error) {
	if a.done || !a.dec.More() {
		if !a.done {
			a.done = true
			if _, err := a.dec.Token(); err != nil {
				return 0, nil, err
			}
		}
		return 0, nil, io.EOF
	}
	a.index++
	v, err := decodeValue(a.dec)
	if err != nil {
		return 0, nil, err
	}
	obj, ok := v.(*object)
	if !ok {
		return 0, nil, core.Invalid("record %d is not a JSON object", a.index)
	}
	return a.index, obj, nil
}

func openJSON(br *bufio.Reader, capped *cappedReader, limits Limits) (Reader, error) {
	dec := json.NewDecoder(br)
	dec.UseNumber()
	tok, err := dec.Token()
	if capped.over() {
		return nil, capped.err
	}
	if errors.Is(err, io.EOF) {
		return nil, core.Invalid("the file is empty")
	}
	if err != nil {
		return nil, core.Invalid("reading JSON: %v", err)
	}
	switch tok {
	case json.Delim('['):
		return newJSONReader(&arraySource{dec: dec}, capped, limits)
	case json.Delim('{'):
		return openWrapper(dec, capped, limits)
	}
	return nil, core.Invalid("the JSON file is neither an array of tasks nor an object holding one")
}

func openWrapper(dec *json.Decoder, capped *cappedReader, limits Limits) (Reader, error) {
	parts := map[string]json.RawMessage{}
	for dec.More() {
		kt, err := dec.Token()
		if err != nil {
			return nil, jsonError(capped, err)
		}
		key, _ := kt.(string)
		if key == "tasks" || key == "issues" {
			tok, err := dec.Token()
			if err != nil {
				return nil, jsonError(capped, err)
			}
			if tok == json.Delim('[') {
				return newJSONReader(&arraySource{dec: dec}, capped, limits)
			}
			if _, err := decodeFrom(dec, tok); err != nil {
				return nil, jsonError(capped, err)
			}
			continue
		}
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			return nil, jsonError(capped, err)
		}
		switch key {
		case "cards", "lists", "labels", "members", "checklists":
			parts[key] = raw
		}
	}
	cards, ok := parts["cards"]
	if !ok {
		return nil, core.Invalid("the JSON file has no tasks, issues or cards array")
	}
	if _, ok := parts["lists"]; ok {
		return openTrello(parts, limits)
	}
	inner := json.NewDecoder(bytes.NewReader(cards))
	inner.UseNumber()
	if tok, err := inner.Token(); err != nil || tok != json.Delim('[') {
		return nil, core.Invalid("the cards member is not an array")
	}
	return newJSONReader(&arraySource{dec: inner}, capped, limits)
}

func jsonError(capped *cappedReader, err error) error {
	if capped.over() {
		return capped.err
	}
	return core.Invalid("reading JSON: %v", err)
}

type lineSource struct {
	br   *bufio.Reader
	line int
}

func (l *lineSource) next() (int, *object, error) {
	for {
		raw, err := l.br.ReadBytes('\n')
		if len(raw) == 0 && err != nil {
			return 0, nil, err
		}
		l.line++
		raw = bytes.TrimSpace(raw)
		if len(raw) == 0 {
			if err != nil {
				return 0, nil, err
			}
			continue
		}
		dec := json.NewDecoder(bytes.NewReader(raw))
		dec.UseNumber()
		v, derr := decodeValue(dec)
		if derr != nil {
			return 0, nil, core.Invalid("line %d is not valid JSON: %v", l.line, derr)
		}
		obj, ok := v.(*object)
		if !ok || dec.More() {
			return 0, nil, core.Invalid("line %d is not a single JSON object", l.line)
		}
		return l.line, obj, nil
	}
}

func openNDJSON(br *bufio.Reader, capped *cappedReader, limits Limits) (Reader, error) {
	return newJSONReader(&lineSource{br: br}, capped, limits)
}
