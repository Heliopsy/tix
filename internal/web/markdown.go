// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"html/template"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/markdown"
)

// previewLimit caps the text a preview renders; a description holds no more.
const previewLimit = core.MaxBodyLength

// markdownOptions links task references to the projects this reader can see
// and handles to the actors this tenant holds. Both directories load on the
// first reference that needs them, so text without one costs nothing.
func (h *handler) markdownOptions(r *http.Request, names actorNames) markdown.Options {
	var (
		once sync.Once
		keys map[string]bool
	)
	handles := map[string]bool{}
	var handlesOnce sync.Once
	return markdown.Options{
		TaskHref: func(ref string) string {
			parsed, err := core.ParseTaskRef(ref)
			if err != nil || parsed.IsID() {
				return ""
			}
			once.Do(func() {
				keys = map[string]bool{}
				projects, _, err := h.allProjects(r, true)
				if err != nil {
					return
				}
				for _, p := range projects {
					keys[strings.ToLower(p.Key)] = true
				}
			})
			if !keys[parsed.ProjectKey] {
				return ""
			}
			return "/tasks/" + parsed.String()
		},
		HandleHref: func(handle string) string {
			handlesOnce.Do(func() {
				for _, h := range names.Handles() {
					handles[strings.ToLower(h)] = true
				}
			})
			if !handles[strings.ToLower(handle)] {
				return ""
			}
			return "/tasks?q=" + url.QueryEscape("assignee:"+handle)
		},
	}
}

// renderProse draws the description and the comments of a task view.
func (h *handler) renderProse(r *http.Request, data *taskView, idPrefix string) {
	opt := h.markdownOptions(r, data.Names)
	comments := opt
	opt.Interactive = data.CanWrite
	opt.IDPrefix = idPrefix
	data.BodyHTML = markdown.Render(data.Task.Body, opt)
	data.CommentHTML = make(map[string]template.HTML, len(data.Comments))
	for _, c := range data.Comments {
		data.CommentHTML[c.ID] = markdown.Render(c.Body, comments)
	}
}

// checkItem ticks or clears one checklist item from the task screen. The
// button pressed names the item and whether it is being ticked, so a second
// press that crossed a refresh asks for the same state rather than flipping
// it back.
func (h *handler) checkItem(w http.ResponseWriter, r *http.Request) error {
	ref, err := taskRefOf(r)
	if err != nil {
		return err
	}
	in := core.CheckItemInput{Done: true}
	raw := field(r, "check")
	if raw == "" {
		raw, in.Done = field(r, "uncheck"), false
	}
	in.Item, err = strconv.Atoi(raw)
	if err != nil || in.Item < 1 {
		return core.Invalid("checklist item %q must be a number from 1", raw)
	}
	if in.Version, err = intField(r, "version"); err != nil {
		return err
	}
	if _, err := h.svc.CheckTaskItem(r.Context(), ref, in); err != nil {
		return err
	}
	if in.Done {
		redirect(w, r, taskPath(r), "item ticked")
	} else {
		redirect(w, r, taskPath(r), "item cleared")
	}
	return nil
}

// previewMarkdown answers the editor's Preview with the text as it will be
// drawn, without saving anything. Checklist items are drawn inert: a preview
// is not the description yet. Blank text answers with nothing, and the
// script says so in the reader's language.
func (h *handler) previewMarkdown(w http.ResponseWriter, r *http.Request) error {
	text := prose(r, "body")
	if len(text) > previewLimit {
		return core.Invalid("the text must be at most %d characters", previewLimit)
	}
	html := markdown.Render(text, h.markdownOptions(r, h.resolveActors(r)))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, err := w.Write([]byte(html))
	return err
}
