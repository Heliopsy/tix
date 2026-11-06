// SPDX-License-Identifier: AGPL-3.0-or-later

package taskio

import (
	"encoding/json"
	"io"
	"strconv"
	"strings"

	"github.com/heliopsy/tix/internal/core"
)

// Source systems Reader.Source can report; empty is a generic file.
const (
	SourceJira   = "jira"
	SourceGitHub = "github"
	SourceTrello = "trello"
	SourceLinear = "linear"
	SourceTix    = "tix"
)

var trelloHeaders = []string{
	"id", "idShort", "name", "desc", "list", "labels", "members",
	"due", "start", "closed", "shortLink", "url", "checklists",
}

type trelloNamed struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Color    string `json:"color"`
	Username string `json:"username"`
	FullName string `json:"fullName"`
}

type trelloCard struct {
	ID        string        `json:"id"`
	IDShort   json.Number   `json:"idShort"`
	Name      string        `json:"name"`
	Desc      string        `json:"desc"`
	Due       *string       `json:"due"`
	Start     *string       `json:"start"`
	Closed    bool          `json:"closed"`
	ShortLink string        `json:"shortLink"`
	URL       string        `json:"url"`
	IDList    string        `json:"idList"`
	IDLabels  []string      `json:"idLabels"`
	Labels    []trelloNamed `json:"labels"`
	IDMembers []string      `json:"idMembers"`
}

type trelloChecklist struct {
	IDCard     string `json:"idCard"`
	Name       string `json:"name"`
	CheckItems []struct {
		Name  string `json:"name"`
		State string `json:"state"`
	} `json:"checkItems"`
}

type sliceReader struct {
	headers []string
	rows    []Row
	source  string
}

func (s *sliceReader) Headers() []string { return s.headers }

func (s *sliceReader) Source() string { return s.source }

func (s *sliceReader) Next() (Row, error) {
	if len(s.rows) == 0 {
		return Row{}, io.EOF
	}
	row := s.rows[0]
	s.rows = s.rows[1:]
	return row, nil
}

func openTrello(parts map[string]json.RawMessage, limits Limits) (Reader, error) {
	var cards []trelloCard
	var lists, labels, members []trelloNamed
	var checklists []trelloChecklist
	for key, dst := range map[string]any{
		"cards": &cards, "lists": &lists, "labels": &labels,
		"members": &members, "checklists": &checklists,
	} {
		raw, ok := parts[key]
		if !ok {
			continue
		}
		if err := json.Unmarshal(raw, dst); err != nil {
			return nil, core.Invalid("reading the Trello %s: %v", key, err)
		}
	}
	if len(cards) > limits.MaxRows {
		return nil, limits.rowsError()
	}
	listNames := names(lists, func(n trelloNamed) string { return n.Name })
	labelNames := names(labels, func(n trelloNamed) string { return firstNonEmpty(n.Name, n.Color) })
	memberNames := names(members, func(n trelloNamed) string { return firstNonEmpty(n.Username, n.FullName) })
	checks := map[string][]string{}
	for _, cl := range checklists {
		lines := []string{"### " + cl.Name}
		for _, item := range cl.CheckItems {
			mark := " "
			if item.State == "complete" {
				mark = "x"
			}
			lines = append(lines, "- ["+mark+"] "+item.Name)
		}
		checks[cl.IDCard] = append(checks[cl.IDCard], strings.Join(lines, "\n"))
	}
	out := &sliceReader{headers: trelloHeaders, source: SourceTrello}
	for i, c := range cards {
		row := Row{Line: i + 1, Headers: trelloHeaders, Values: map[string]string{
			"id":         c.ID,
			"idShort":    c.IDShort.String(),
			"name":       c.Name,
			"desc":       c.Desc,
			"list":       listNames[c.IDList],
			"labels":     trelloLabels(c, labelNames),
			"members":    lookupAll(c.IDMembers, memberNames),
			"due":        deref(c.Due),
			"start":      deref(c.Start),
			"closed":     strconv.FormatBool(c.Closed),
			"shortLink":  c.ShortLink,
			"url":        c.URL,
			"checklists": strings.Join(checks[c.ID], "\n\n"),
		}}
		for _, h := range trelloHeaders {
			if len(row.Values[h]) > limits.MaxCell {
				return nil, limits.cellError(row.Line, h)
			}
		}
		out.rows = append(out.rows, row)
	}
	return out, nil
}

func trelloLabels(c trelloCard, byID map[string]string) string {
	if len(c.Labels) > 0 {
		var parts []string
		for _, l := range c.Labels {
			if n := firstNonEmpty(l.Name, l.Color, byID[l.ID]); n != "" {
				parts = append(parts, n)
			}
		}
		return strings.Join(parts, ", ")
	}
	return lookupAll(c.IDLabels, byID)
}

func names(items []trelloNamed, name func(trelloNamed) string) map[string]string {
	out := make(map[string]string, len(items))
	for _, it := range items {
		out[it.ID] = name(it)
	}
	return out
}

func lookupAll(ids []string, byID map[string]string) string {
	var parts []string
	for _, id := range ids {
		if n := byID[id]; n != "" {
			parts = append(parts, n)
		}
	}
	return strings.Join(parts, ", ")
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
