// SPDX-License-Identifier: AGPL-3.0-or-later

package sshd

import (
	"fmt"
	"math"
	"time"

	"charm.land/ssh"
)

// throttleKey holds a connection's rate limit decision in its context: the
// wait before the next token, or zero when the connection was admitted.
type throttleKey struct{}

// throttle charges the connection's source one token the first time it is
// asked and records the outcome, so a client offering several keys pays once.
func (s *Server) throttle(ctx ssh.Context) {
	if _, ok := ctx.Value(throttleKey{}).(time.Duration); ok {
		return
	}
	ctx.SetValue(throttleKey{}, s.limiter.take(sourceOf(ctx.RemoteAddr())))
}

// throttled returns how long the connection behind ctx must wait, or zero
// when the rate limit admitted it.
func throttled(ctx interface{ Value(key any) any }) time.Duration {
	wait, _ := ctx.Value(throttleKey{}).(time.Duration)
	return wait
}

// throttleMessage is what a client over the rate limit is told.
func throttleMessage(wait time.Duration) string {
	return fmt.Sprintf("too many connections from this address; try again in %d seconds",
		int(math.Ceil(wait.Seconds())))
}
