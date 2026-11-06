// SPDX-License-Identifier: AGPL-3.0-or-later

package httpapi_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/wire"
)

func TestTaskExportAnswersInTheFormatAsked(t *testing.T) {
	f := newFixture(t)
	f.createTask("exported")
	route := strings.Replace(wire.RouteProjectTaskExport, "{ref}", "infra", 1)
	for _, tc := range []struct {
		format core.TaskFormat
		media  string
		file   string
	}{
		{core.TaskFormatCSV, wire.ContentCSV, "infra-tasks.csv"},
		{core.TaskFormatJSON, wire.ContentJSON, "infra-tasks.json"},
		{core.TaskFormatNDJSON, wire.ContentNDJSON, "infra-tasks.ndjson"},
		{core.TaskFormatMarkdown, wire.ContentMarkdown, "infra-tasks.md"},
	} {
		t.Run(string(tc.format), func(t *testing.T) {
			resp := f.call(http.MethodPost, route, core.TaskExportInput{Format: tc.format, Columns: []string{"title"}})
			mustStatus(t, resp, http.StatusOK)
			if got := resp.Header.Get(wire.HeaderContentType); got != tc.media {
				t.Errorf("content type = %q, want %q", got, tc.media)
			}
			if got := resp.Header.Get(wire.HeaderContentDisposition); !strings.Contains(got, tc.file) {
				t.Errorf("content disposition = %q, want %s", got, tc.file)
			}
			if body := readBody(t, resp); !strings.Contains(body, "exported") {
				t.Errorf("body = %s", body)
			}
		})
	}
}

func TestTaskExportRefusesInTheErrorEnvelope(t *testing.T) {
	f := newFixture(t)
	route := strings.Replace(wire.RouteProjectTaskExport, "{ref}", "infra", 1)
	resp := f.call(http.MethodPost, route, core.TaskExportInput{Columns: []string{"nope"}})
	mustStatus(t, resp, http.StatusBadRequest)
	if got := resp.Header.Get(wire.HeaderContentDisposition); got != "" {
		t.Errorf("a refusal was offered as a download: %q", got)
	}
	missing := strings.Replace(wire.RouteProjectTaskExport, "{ref}", "nope", 1)
	mustStatus(t, f.call(http.MethodPost, missing, nil), http.StatusNotFound)
}
