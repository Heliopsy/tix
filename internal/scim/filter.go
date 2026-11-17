// SPDX-License-Identifier: AGPL-3.0-or-later

package scim

import (
	"strings"
	"unicode"

	"github.com/heliopsy/tix/internal/core"
)

// resource names the kind of resource a filter is parsed for.
type resource int

const (
	resourceUser resource = iota
	resourceGroup
)

// errFilter is the refusal of a filter tix cannot answer.
func errFilter(detail string) error { return badRequest(TypeInvalidFilter, detail) }

// parseFilter reads the subset of RFC 7644 filters identity providers send to
// match a resource: equality terms joined with "and". A user is matched by
// id, userName, externalId and an email value; a group by id, displayName,
// externalId and a member.
func parseFilter(raw string, kind resource) ([]core.SCIMCondition, error) {
	toks, err := tokenize(raw)
	if err != nil {
		return nil, err
	}
	var out []core.SCIMCondition
	for len(toks) > 0 {
		cond, rest, err := parseTerm(toks, kind)
		if err != nil {
			return nil, err
		}
		out = append(out, cond)
		toks = rest
		if len(toks) == 0 {
			break
		}
		if !strings.EqualFold(toks[0].text, "and") || toks[0].quoted {
			return nil, errFilter("only equality terms joined with and are supported")
		}
		toks = toks[1:]
		if len(toks) == 0 {
			return nil, errFilter("the filter ends after and")
		}
	}
	return out, nil
}

// token is one word, quoted string or bracket of a filter.
type token struct {
	text   string
	quoted bool
}

// tokenize splits a filter into words, quoted strings and brackets.
func tokenize(raw string) ([]token, error) {
	var out []token
	rs := []rune(raw)
	for i := 0; i < len(rs); {
		r := rs[i]
		switch {
		case unicode.IsSpace(r):
			i++
		case r == '[' || r == ']':
			out = append(out, token{text: string(r)})
			i++
		case r == '"':
			var sb strings.Builder
			i++
			closed := false
			for i < len(rs) {
				if rs[i] == '\\' && i+1 < len(rs) {
					sb.WriteRune(rs[i+1])
					i += 2
					continue
				}
				if rs[i] == '"' {
					closed = true
					i++
					break
				}
				sb.WriteRune(rs[i])
				i++
			}
			if !closed {
				return nil, errFilter("a quoted value is not closed")
			}
			out = append(out, token{text: sb.String(), quoted: true})
		default:
			start := i
			for i < len(rs) && !unicode.IsSpace(rs[i]) && rs[i] != '[' && rs[i] != ']' && rs[i] != '"' {
				i++
			}
			out = append(out, token{text: string(rs[start:i])})
		}
	}
	if len(out) == 0 {
		return nil, errFilter("the filter is empty")
	}
	return out, nil
}

// parseTerm reads one term: attr eq value, attr[sub eq value].leaf eq value,
// or a standalone members[value eq "id"].
func parseTerm(toks []token, kind resource) (core.SCIMCondition, []token, error) {
	if toks[0].quoted {
		return core.SCIMCondition{}, nil, errFilter("a term starts with an attribute name")
	}
	attr := stripURN(toks[0].text)
	toks = toks[1:]
	bracketAttr, bracketValue := "", ""
	if len(toks) > 0 && toks[0].text == "[" && !toks[0].quoted {
		if len(toks) < 5 || toks[4].text != "]" || !strings.EqualFold(toks[2].text, "eq") {
			return core.SCIMCondition{}, nil, errFilter("only [attribute eq value] is supported inside brackets")
		}
		bracketAttr, bracketValue = strings.ToLower(toks[1].text), toks[3].text
		toks = toks[5:]
		if len(toks) > 0 && strings.HasPrefix(toks[0].text, ".") && !toks[0].quoted {
			attr += toks[0].text
			toks = toks[1:]
		}
	}
	lower := strings.ToLower(attr)
	if kind == resourceGroup && lower == "members" && bracketAttr == "value" {
		return core.SCIMCondition{Attr: core.SCIMAttrMember, Value: bracketValue}, toks, nil
	}
	if len(toks) < 2 || !strings.EqualFold(toks[0].text, "eq") || toks[0].quoted {
		return core.SCIMCondition{}, nil, errFilter("only the eq operator is supported")
	}
	value := toks[1].text
	toks = toks[2:]
	name, err := filterAttr(lower, bracketAttr, bracketValue, kind)
	if err != nil {
		return core.SCIMCondition{}, nil, err
	}
	return core.SCIMCondition{Attr: name, Value: value}, toks, nil
}

// filterAttr maps a filter's attribute path to the condition it names.
func filterAttr(attr, bracketAttr, bracketValue string, kind resource) (string, error) {
	if bracketAttr != "" {
		if kind == resourceUser && attr == "emails.value" && bracketAttr == "type" {
			if !strings.EqualFold(bracketValue, "work") {
				return "", errFilter("only work emails can be filtered on")
			}
			return core.SCIMAttrEmail, nil
		}
		return "", errFilter("the filter " + attr + " is not supported")
	}
	switch {
	case attr == "id":
		return core.SCIMAttrID, nil
	case attr == "externalid":
		return core.SCIMAttrExternalID, nil
	case kind == resourceUser && attr == "username":
		return core.SCIMAttrUserName, nil
	case kind == resourceUser && (attr == "emails" || attr == "emails.value"):
		return core.SCIMAttrEmail, nil
	case kind == resourceGroup && attr == "displayname":
		return core.SCIMAttrDisplayName, nil
	case kind == resourceGroup && (attr == "members" || attr == "members.value"):
		return core.SCIMAttrMember, nil
	}
	return "", errFilter("filtering on " + attr + " is not supported")
}

// stripURN drops a core schema prefix from an attribute path.
func stripURN(attr string) string {
	for _, urn := range []string{SchemaUser + ":", SchemaGroup + ":"} {
		if len(attr) > len(urn) && strings.EqualFold(attr[:len(urn)], urn) {
			return attr[len(urn):]
		}
	}
	return attr
}
