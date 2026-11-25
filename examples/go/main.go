// SPDX-License-Identifier: AGPL-3.0-or-later

// Command tix-example makes one pass over the tix HTTP API with net/http and
// nothing else. docs/examples.md walks through it step by step.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// region setup
var (
	api   = strings.TrimRight(os.Getenv("TIX_URL"), "/") + "/api/v1"
	token = os.Getenv("TIX_TOKEN")
)

// APIError is the error envelope every tix route answers with.
type APIError struct {
	Status  int            `json:"-"`
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details"`
}

func (e *APIError) Error() string { return fmt.Sprintf("%d %s: %s", e.Status, e.Code, e.Message) }

// Task holds the task fields this example reads.
type Task struct {
	Ref      string `json:"ref"`
	Title    string `json:"title"`
	Status   string `json:"status"`
	Priority int    `json:"priority"`
	Version  int    `json:"version"`
}

func call(method, path string, query url.Values, body, out any) error {
	u := api + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	var payload io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		payload = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, u, payload)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 300 {
		var envelope struct{ Error *APIError }
		if json.NewDecoder(resp.Body).Decode(&envelope) != nil || envelope.Error == nil {
			envelope.Error = &APIError{Code: "http", Message: resp.Status}
		}
		envelope.Error.Status = resp.StatusCode
		return envelope.Error
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// endregion

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	// region whoami
	var me struct{ Handle, Kind string }
	if err := call("GET", "/whoami", nil, nil, &me); err != nil {
		return err
	}
	fmt.Printf("signed in as %s (%s)\n", me.Handle, me.Kind)
	// endregion

	// region cursor
	var start struct {
		NextSeq int64 `json:"next_seq"`
	}
	if err := call("GET", "/events/poll", nil, nil, &start); err != nil {
		return err
	}
	// endregion

	// region create-project
	key := os.Getenv("TIX_PROJECT")
	if key == "" {
		key = fmt.Sprintf("ex%d", time.Now().Unix())
	}
	var project struct{ Key string }
	if err := call("POST", "/projects", nil, map[string]any{"key": key, "name": "API examples"}, &project); err != nil {
		return err
	}
	fmt.Printf("created project %s\n", project.Key)
	// endregion

	// region create-task
	var task Task
	if err := call("POST", "/tasks", nil, map[string]any{"project_ref": key, "title": "Try the API"}, &task); err != nil {
		return err
	}
	ref, createdVersion := task.Ref, task.Version
	fmt.Printf("created %s at version %d\n", ref, createdVersion)
	// endregion

	// region update-task
	update := map[string]any{
		"title":    "Try the tix API",
		"priority": 1,
		"due_at":   "2030-01-15T17:00:00Z",
		"version":  task.Version,
	}
	if err := call("PATCH", "/tasks/"+ref, nil, update, &task); err != nil {
		return err
	}
	fmt.Printf("updated %s to version %d\n", ref, task.Version)
	// endregion

	// region conflict
	change := map[string]any{"title": "Try the tix HTTP API", "version": createdVersion}
	err := call("PATCH", "/tasks/"+ref, nil, change, &task)
	var apiErr *APIError
	if errors.As(err, &apiErr) && apiErr.Status == http.StatusConflict && apiErr.Code == "conflict" {
		fmt.Printf("conflict: %s is at version %v, retrying\n", ref, apiErr.Details["current_version"])
		if err := call("GET", "/tasks/"+ref, nil, nil, &task); err != nil {
			return err
		}
		change["version"] = task.Version
		err = call("PATCH", "/tasks/"+ref, nil, change, &task)
	}
	if err != nil {
		return err
	}
	fmt.Printf("retitled %s at version %d\n", ref, task.Version)
	// endregion

	// region transition
	move := map[string]any{"to": "doing", "version": task.Version}
	if err := call("POST", "/tasks/"+ref+"/transition", nil, move, &task); err != nil {
		return err
	}
	fmt.Printf("%s is now %s\n", ref, task.Status)
	// endregion

	// region comment
	var comment struct{ ID string }
	if err := call("POST", "/tasks/"+ref+"/comments", nil, map[string]any{"body": "Started from the API example."}, &comment); err != nil {
		return err
	}
	fmt.Printf("commented %s\n", comment.ID)
	// endregion

	// region list
	filter := url.Values{"project": {key}, "status": {"doing"}, "priority": {"1"}, "sort": {"due_at"}}
	var page struct{ Items []Task }
	if err := call("GET", "/tasks", filter, nil, &page); err != nil {
		return err
	}
	for _, t := range page.Items {
		fmt.Printf("listed %s %s p%d %s\n", t.Ref, t.Status, t.Priority, t.Title)
	}
	// endregion

	// region events
	since := url.Values{"since_seq": {fmt.Sprint(start.NextSeq)}, "types": {"task.*,comment.added"}, "limit": {"10"}}
	var batch struct {
		Events []struct {
			Seq  int64
			Type string
		}
	}
	if err := call("GET", "/events/poll", since, nil, &batch); err != nil {
		return err
	}
	for _, e := range batch.Events {
		fmt.Printf("event %d %s\n", e.Seq, e.Type)
	}
	// endregion
	return nil
}
