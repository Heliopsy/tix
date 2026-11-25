// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAPageTheBrowserLeftIsNeitherLoggedNorAnswered(t *testing.T) {
	t.Parallel()
	var logs bytes.Buffer
	h := &handler{logger: slog.New(slog.NewTextHandler(&logs, nil))}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r := httptest.NewRequest(http.MethodGet, RouteTasks, nil).WithContext(ctx)
	w := httptest.NewRecorder()

	h.fail(w, r, fmt.Errorf("listing tasks: %w", context.Canceled))

	if logs.Len() != 0 {
		t.Errorf("logged %q for a browser that left", logs.String())
	}
	if w.Body.Len() != 0 {
		t.Errorf("answered a browser that left with %d bytes", w.Body.Len())
	}
}
