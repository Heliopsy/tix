// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/quickadd"
)

// RouteQuickAdd answers the one-line add box's preview of what its line sets.
const RouteQuickAdd = "/tasks/quickadd"

// quickPreview is the row under the one-line box: one chip per recognised
// token, and the reason the line would be or was refused.
type quickPreview struct {
	Chips []quickChip
	Error string
}

// quickChip is one recognised token. Unknown marks a project or handle this
// tenant does not have, which the add would be refused for.
type quickChip struct {
	Kind    string
	Value   string
	Unknown bool
}

// readerNow is the present in the zone this reader chose under Settings, or
// the deployment's own, which is the calendar "tomorrow" is counted in.
func (h *handler) readerNow(r *http.Request) time.Time {
	loc := h.style.Location()
	if zone := timezoneOf(r); zone != "" {
		if chosen, err := time.LoadLocation(zone); err == nil {
			loc = chosen
		}
	}
	return time.Now().In(loc)
}

// quickInput reads the box's line into the input. The project picker beside
// the box is a default, so a project the line names takes its place rather
// than contradicting it.
func (h *handler) quickInput(r *http.Request, in core.CreateTaskInput) (core.CreateTaskInput, error) {
	parsed, err := quickadd.Parse(in.Title, h.readerNow(r))
	if err != nil {
		return in, err
	}
	picked := in.ProjectRef
	in.ProjectRef = ""
	if in, err = parsed.Apply(in); err != nil {
		return in, err
	}
	if in.ProjectRef == "" {
		in.ProjectRef = picked
	}
	return in, nil
}

// previewQuickAdd answers the preview fragment for the line typed so far.
func (h *handler) previewQuickAdd(w http.ResponseWriter, r *http.Request) error {
	return h.renderFragment(w, r, "tasks.html", "quickadd-live", "Tasks", h.previewLine(r, r.URL.Query().Get("title")))
}

// previewLine parses a line and marks a project or handle the tenant does not
// have. Lookups that fail for any other reason mark nothing: the preview is a
// courtesy and the add itself is checked by the service.
func (h *handler) previewLine(r *http.Request, line string) quickPreview {
	if strings.TrimSpace(line) == "" {
		return quickPreview{}
	}
	parsed, err := quickadd.Parse(line, h.readerNow(r))
	if err != nil {
		return quickPreview{Error: safeMessage(err, core.KindOf(err))}
	}
	var handles func() []string
	out := quickPreview{Chips: make([]quickChip, 0, len(parsed.Tokens))}
	for _, tok := range parsed.Tokens {
		chip := quickChip{Kind: string(tok.Kind), Value: tok.Value}
		switch tok.Kind {
		case quickadd.KindProject:
			_, err := h.svc.GetProject(r.Context(), tok.Value)
			chip.Unknown = core.IsKind(err, core.KindNotFound)
		case quickadd.KindAssignee:
			if handles == nil {
				handles = h.actorHandles(r)
			}
			known := handles()
			chip.Unknown = len(known) > 0 && !slices.Contains(known, tok.Value)
		}
		out.Chips = append(out.Chips, chip)
	}
	return out
}

// refusalMessage is the one message a refused quick add shows under the box,
// whichever field the service named.
func refusalMessage(err error) string {
	for _, message := range refusal(err) {
		return message
	}
	return ""
}
