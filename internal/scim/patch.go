// SPDX-License-Identifier: AGPL-3.0-or-later

package scim

import (
	"encoding/json"
	"slices"
	"strings"

	"github.com/heliopsy/tix/internal/core"
)

// PatchRequest is a PATCH body.
type PatchRequest struct {
	Schemas    []string  `json:"schemas"`
	Operations []PatchOp `json:"Operations"`
}

// PatchOp is one operation of a PATCH.
type PatchOp struct {
	Op    string          `json:"op"`
	Path  string          `json:"path"`
	Value json.RawMessage `json:"value"`
}

// Patch operations, compared ignoring case because Entra ID capitalises them.
const (
	opAdd     = "add"
	opReplace = "replace"
	opRemove  = "remove"
)

// normalOp checks an operation's name and returns it lower-cased.
func normalOp(op PatchOp) (string, error) {
	name := strings.ToLower(strings.TrimSpace(op.Op))
	switch name {
	case opAdd, opReplace, opRemove:
		return name, nil
	}
	return "", badRequest(TypeInvalidSyntax, "op must be add, replace or remove, not "+op.Op)
}

// normalPath drops a core schema prefix and lower-cases the attribute names,
// leaving a bracketed filter's value as it was sent.
func normalPath(path string) string {
	path = stripURN(strings.TrimSpace(path))
	head, rest, found := strings.Cut(path, "[")
	if !found {
		return strings.ToLower(path)
	}
	return strings.ToLower(head) + "[" + rest
}

// strValue reads a value sent as a string, or as a one-entry multi-value.
func strValue(raw json.RawMessage) (string, error) {
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return strings.TrimSpace(s), nil
	}
	var many []MultiValue
	if err := json.Unmarshal(raw, &many); err == nil && len(many) > 0 {
		return strings.TrimSpace(many[0].Value), nil
	}
	var one MultiValue
	if err := json.Unmarshal(raw, &one); err == nil {
		return strings.TrimSpace(one.Value), nil
	}
	return "", badRequest(TypeInvalidValue, "expected a string value")
}

// pathless applies an operation without a path: its value is an object
// whose members are each applied as a path.
func pathless(op string, raw json.RawMessage, apply func(op, path string, value json.RawMessage) error) error {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return badRequest(TypeInvalidSyntax, "an operation without a path takes an object value")
	}
	keys := make([]string, 0, len(obj))
	for k := range obj {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, k := range keys {
		if err := apply(op, k, obj[k]); err != nil {
			return err
		}
	}
	return nil
}

// applyUserPatch applies a PATCH to a member. Attributes tix keeps nowhere,
// such as the enterprise extension, are accepted and ignored.
func applyUserPatch(u *core.SCIMUser, req PatchRequest) error {
	if len(req.Operations) == 0 {
		return badRequest(TypeInvalidSyntax, "a patch needs at least one operation")
	}
	for _, op := range req.Operations {
		name, err := normalOp(op)
		if err != nil {
			return err
		}
		if op.Path == "" {
			if name == opRemove {
				return badRequest("noTarget", "remove needs a path")
			}
			if err := pathless(name, op.Value, func(o, p string, v json.RawMessage) error {
				return userOp(u, o, normalPath(p), v)
			}); err != nil {
				return err
			}
			continue
		}
		if err := userOp(u, name, normalPath(op.Path), op.Value); err != nil {
			return err
		}
	}
	return nil
}

// userOp applies one operation on one path of a member.
func userOp(u *core.SCIMUser, op, path string, value json.RawMessage) error {
	remove := op == opRemove
	text := func(set func(string)) error {
		if remove {
			set("")
			return nil
		}
		s, err := strValue(value)
		if err != nil {
			return err
		}
		set(s)
		return nil
	}
	switch {
	case path == "active":
		if remove {
			return nil
		}
		on, err := parseBool(value, true)
		if err != nil {
			return err
		}
		u.Active = on
	case path == "username":
		if remove {
			return badRequest(TypeMutability, "userName is required and cannot be removed")
		}
		return text(func(s string) { u.UserName = s })
	case path == "externalid":
		return text(func(s string) { u.ExternalID = s })
	case path == "displayname":
		return text(func(s string) { u.DisplayName = s })
	case path == "name.givenname":
		return text(func(s string) { u.GivenName = s })
	case path == "name.familyname":
		return text(func(s string) { u.FamilyName = s })
	case path == "name":
		if remove {
			u.GivenName, u.FamilyName = "", ""
			return nil
		}
		var n Name
		if err := json.Unmarshal(value, &n); err != nil {
			return badRequest(TypeInvalidValue, "name must be an object")
		}
		if n.GivenName != "" || op == opReplace {
			u.GivenName = strings.TrimSpace(n.GivenName)
		}
		if n.FamilyName != "" || op == opReplace {
			u.FamilyName = strings.TrimSpace(n.FamilyName)
		}
	case strings.HasPrefix(path, "emails"):
		return emailOp(u, remove, value)
	}
	return nil
}

// emailOp sets a member's address from an emails value of any shape.
func emailOp(u *core.SCIMUser, remove bool, value json.RawMessage) error {
	if remove {
		u.Email = ""
		return nil
	}
	var many []emailIn
	if err := json.Unmarshal(value, &many); err == nil {
		if email := pickEmail(many); email != "" {
			u.Email = email
		}
		return nil
	}
	var one emailIn
	if err := json.Unmarshal(value, &one); err == nil && one.Value != "" {
		u.Email = strings.TrimSpace(one.Value)
		return nil
	}
	s, err := strValue(value)
	if err != nil {
		return err
	}
	u.Email = s
	return nil
}

// applyGroupPatch applies a PATCH to a group: its name, externalId and
// members.
func applyGroupPatch(g *core.SCIMGroup, req PatchRequest) error {
	if len(req.Operations) == 0 {
		return badRequest(TypeInvalidSyntax, "a patch needs at least one operation")
	}
	for _, op := range req.Operations {
		name, err := normalOp(op)
		if err != nil {
			return err
		}
		if op.Path == "" {
			if name == opRemove {
				return badRequest("noTarget", "remove needs a path")
			}
			if err := pathless(name, op.Value, func(o, p string, v json.RawMessage) error {
				return groupOp(g, o, normalPath(p), v)
			}); err != nil {
				return err
			}
			continue
		}
		if err := groupOp(g, name, normalPath(op.Path), op.Value); err != nil {
			return err
		}
	}
	return nil
}

// groupOp applies one operation on one path of a group.
func groupOp(g *core.SCIMGroup, op, path string, value json.RawMessage) error {
	switch {
	case path == "displayname":
		if op == opRemove {
			return badRequest(TypeMutability, "displayName is required and cannot be removed")
		}
		s, err := strValue(value)
		if err != nil {
			return err
		}
		g.DisplayName = s
	case path == "externalid":
		if op == opRemove {
			g.ExternalID = ""
			return nil
		}
		s, err := strValue(value)
		if err != nil {
			return err
		}
		g.ExternalID = s
	case path == "members":
		return membersOp(g, op, value)
	case strings.HasPrefix(path, "members["):
		if op != opRemove {
			return badRequest(TypeInvalidPath, "a filtered members path can only be removed")
		}
		conds, err := parseFilter(path, resourceGroup)
		if err != nil || len(conds) != 1 || conds[0].Attr != core.SCIMAttrMember {
			return badRequest(TypeInvalidPath, "expected members[value eq \"id\"]")
		}
		g.Members = withoutMembers(g.Members, []string{conds[0].Value})
	}
	return nil
}

// memberIDs reads a members value: a list of references, or one.
func memberIDs(value json.RawMessage) ([]string, error) {
	if len(value) == 0 || string(value) == "null" {
		return nil, nil
	}
	var many []MultiValue
	if err := json.Unmarshal(value, &many); err != nil {
		var one MultiValue
		if err := json.Unmarshal(value, &one); err != nil {
			return nil, badRequest(TypeInvalidValue, "members must be a list of {value}")
		}
		many = []MultiValue{one}
	}
	out := make([]string, 0, len(many))
	for _, m := range many {
		if id := strings.TrimSpace(m.Value); id != "" {
			out = append(out, id)
		}
	}
	return out, nil
}

// membersOp adds, removes or replaces members.
func membersOp(g *core.SCIMGroup, op string, value json.RawMessage) error {
	ids, err := memberIDs(value)
	if err != nil {
		return err
	}
	switch op {
	case opAdd:
		for _, id := range ids {
			if !slices.ContainsFunc(g.Members, func(r core.SCIMRef) bool { return r.ID == id }) {
				g.Members = append(g.Members, core.SCIMRef{ID: id})
			}
		}
	case opRemove:
		if len(ids) == 0 {
			g.Members = nil
			return nil
		}
		g.Members = withoutMembers(g.Members, ids)
	case opReplace:
		g.Members = nil
		for _, id := range ids {
			g.Members = append(g.Members, core.SCIMRef{ID: id})
		}
	}
	return nil
}

// withoutMembers drops the named members.
func withoutMembers(refs []core.SCIMRef, ids []string) []core.SCIMRef {
	return slices.DeleteFunc(slices.Clone(refs), func(r core.SCIMRef) bool { return slices.Contains(ids, r.ID) })
}
