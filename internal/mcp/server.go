// SPDX-License-Identifier: AGPL-3.0-or-later

package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/heliopsy/tix/internal/core"
)

// Caller sends one request to the tix API and returns the response body, or
// the server's error.
type Caller interface {
	Raw(ctx context.Context, method, path string, q url.Values, body []byte) ([]byte, error)
}

// instructions is what the server tells a client about itself.
const instructions = "Tools for tix, a task tracker built for agents and people. " +
	"Each tool is one call to the tix HTTP API. Tasks are named by their ref, such as infra-42. " +
	"To work a queue, claim_next hands out the next unblocked task with a lease token; " +
	"renew it with claim_renew while working and finish with task_transition or claim_release."

// NewServer returns an MCP server offering the exposed tools, each answered
// by one request through caller.
func NewServer(tools []Tool, caller Caller, version string) *sdk.Server {
	srv := sdk.NewServer(&sdk.Implementation{Name: "tix", Version: version},
		&sdk.ServerOptions{Instructions: instructions})
	for _, t := range Exposed(tools) {
		srv.AddTool(t.descriptor(), t.handler(caller))
	}
	return srv
}

// descriptor is the tool as a client lists it.
func (t Tool) descriptor() *sdk.Tool {
	read := isRead(t.Op.Method)
	ann := &sdk.ToolAnnotations{
		ReadOnlyHint:   read,
		IdempotentHint: read || t.Op.Method == http.MethodPut || t.Op.Method == http.MethodDelete,
	}
	if !read {
		destructive := t.Destructive
		ann.DestructiveHint = &destructive
	}
	return &sdk.Tool{Name: t.Name, Description: t.Description, InputSchema: t.Schema, Annotations: ann}
}

func (t Tool) handler(caller Caller) sdk.ToolHandler {
	return func(ctx context.Context, req *sdk.CallToolRequest) (*sdk.CallToolResult, error) {
		path, q, body, err := t.request(req.Params.Arguments)
		if err != nil {
			return failure(err), nil
		}
		raw, err := caller.Raw(ctx, t.Op.Method, path, q, body)
		if err != nil {
			return failure(err), nil
		}
		return success(raw), nil
	}
}

// request turns a call's arguments into the path, query and body of one
// request. It checks only what it must to build one; the server validates.
func (t Tool) request(rawArgs json.RawMessage) (string, url.Values, []byte, error) {
	args := map[string]any{}
	if len(bytes.TrimSpace(rawArgs)) > 0 && !bytes.Equal(bytes.TrimSpace(rawArgs), []byte("null")) {
		dec := json.NewDecoder(bytes.NewReader(rawArgs))
		dec.UseNumber()
		if err := dec.Decode(&args); err != nil {
			return "", nil, nil, core.Invalid("arguments are not a JSON object: %v", err)
		}
	}
	path := t.Op.Path
	q := url.Values{}
	bodyFields := map[string]any{}
	var bodyWhole any
	names := make([]string, 0, len(args))
	for name := range args {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		value := args[name]
		p, isParam := t.args[name]
		switch {
		case isParam && p.In == "path":
			path = strings.Replace(path, "{"+name+"}", url.PathEscape(scalar(value)), 1)
		case isParam:
			addQuery(q, name, value)
		case t.nested && name == bodyArgument:
			bodyWhole = value
		case !t.nested && t.Op.Body != nil && hasProperty(t.Schema, name):
			bodyFields[name] = value
		default:
			return "", nil, nil, core.Invalid("%s takes no argument %q", t.Name, name)
		}
	}
	if strings.Contains(path, "{") {
		return "", nil, nil, core.Invalid("%s needs every path argument: %s", t.Name, path)
	}
	body, err := t.encodeBody(bodyWhole, bodyFields)
	return path, q, body, err
}

func (t Tool) encodeBody(whole any, fields map[string]any) ([]byte, error) {
	if t.Op.Body == nil {
		return nil, nil
	}
	var v any
	switch {
	case t.nested && whole != nil:
		v = whole
	case !t.nested && (len(fields) > 0 || t.Op.BodyRequired):
		v = fields
	case t.Op.BodyRequired:
		v = map[string]any{}
	default:
		return nil, nil
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, core.Invalid("encoding the request body: %v", err)
	}
	return raw, nil
}

func hasProperty(schema map[string]any, name string) bool {
	props, _ := schema["properties"].(map[string]any)
	_, ok := props[name]
	return ok
}

// addQuery writes one argument as the query parameter the API reads: a list
// repeats the key, an object travels as JSON.
func addQuery(q url.Values, name string, value any) {
	switch v := value.(type) {
	case nil:
	case []any:
		for _, item := range v {
			q.Add(name, scalar(item))
		}
	case map[string]any:
		raw, _ := json.Marshal(v)
		q.Set(name, string(raw))
	default:
		q.Set(name, scalar(v))
	}
}

func scalar(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case bool:
		return strconv.FormatBool(x)
	case json.Number:
		return x.String()
	case nil:
		return ""
	default:
		return fmt.Sprint(x)
	}
}

// success returns the response as text, and as structured content when it is
// a JSON object.
func success(raw []byte) *sdk.CallToolResult {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return &sdk.CallToolResult{Content: []sdk.Content{&sdk.TextContent{Text: `{"status":"ok"}`}}}
	}
	res := &sdk.CallToolResult{Content: []sdk.Content{&sdk.TextContent{Text: string(trimmed)}}}
	var obj map[string]any
	if json.Unmarshal(trimmed, &obj) == nil {
		res.StructuredContent = obj
	}
	return res
}

// failure reports an error the way the API's envelope carries it, so a model
// reads the same kind a script would.
func failure(err error) *sdk.CallToolResult {
	kind, message := core.KindInternal, err.Error()
	var details map[string]any
	var de *core.Error
	if errors.As(err, &de) {
		kind, message, details = de.Kind, de.Message, de.Details
	}
	body := map[string]any{"code": kind, "message": message}
	if len(details) > 0 {
		body["details"] = details
	}
	envelope := map[string]any{"error": body}
	raw, _ := json.Marshal(envelope)
	return &sdk.CallToolResult{
		IsError:           true,
		Content:           []sdk.Content{&sdk.TextContent{Text: string(raw)}},
		StructuredContent: envelope,
	}
}
