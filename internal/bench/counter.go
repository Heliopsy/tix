// SPDX-License-Identifier: AGPL-3.0-or-later

package bench

import "sync/atomic"

// atomicCounter hands out distinct lease tokens to racing goroutines.
type atomicCounter struct{ n atomic.Int64 }

func (c *atomicCounter) next() int64 { return c.n.Add(1) }
