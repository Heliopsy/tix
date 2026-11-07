// SPDX-License-Identifier: AGPL-3.0-or-later

package config

import "fmt"

// ImportLimitRefusal is what a server answers, with 413, to a task import
// larger than server.max_import_bytes: the limit, and the key that sets it.
func ImportLimitRefusal(limit int64) string {
	return fmt.Sprintf("a task import may be at most %s; the server's operator can raise server.max_import_bytes "+
		"(%s) to take a larger file", byteSize(limit), EnvName("server.max_import_bytes"))
}

// byteSize writes a byte count in the largest binary unit that divides it.
func byteSize(n int64) string {
	switch {
	case n >= 1<<20 && n%(1<<20) == 0:
		return fmt.Sprintf("%d MiB", n>>20)
	case n >= 1<<10 && n%(1<<10) == 0:
		return fmt.Sprintf("%d KiB", n>>10)
	}
	return fmt.Sprintf("%d bytes", n)
}
