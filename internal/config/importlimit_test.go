// SPDX-License-Identifier: AGPL-3.0-or-later

package config_test

import (
	"testing"

	"github.com/heliopsy/tix/internal/config"
)

func TestImportLimitRefusalNamesTheLimitAndTheKey(t *testing.T) {
	for _, tc := range []struct {
		limit int64
		want  string
	}{
		{32 << 20, "a task import may be at most 32 MiB; the server's operator can raise server.max_import_bytes " +
			"(TIX_SERVER_MAX_IMPORT_BYTES) to take a larger file"},
		{3 << 10, "a task import may be at most 3 KiB; the server's operator can raise server.max_import_bytes " +
			"(TIX_SERVER_MAX_IMPORT_BYTES) to take a larger file"},
		{1000, "a task import may be at most 1000 bytes; the server's operator can raise server.max_import_bytes " +
			"(TIX_SERVER_MAX_IMPORT_BYTES) to take a larger file"},
	} {
		if got := config.ImportLimitRefusal(tc.limit); got != tc.want {
			t.Errorf("ImportLimitRefusal(%d) = %q, want %q", tc.limit, got, tc.want)
		}
	}
}
