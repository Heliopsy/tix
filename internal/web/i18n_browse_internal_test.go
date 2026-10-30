// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import "github.com/heliopsy/tix/internal/core"

// recoveryLedes are the ledes the handlers hand the recovery codes screen,
// written out here because they are arguments rather than named values.
var recoveryLedes = []string{
	"You are signed in. Keep these codes somewhere safe: each one signs you in " +
		"once if you lose the authenticator, and they are not shown again.",
	"Your second factor is on. Keep these codes somewhere safe: " +
		"each one signs you in once if you lose the authenticator, and they are not shown again.",
	"These replace every code issued before, including any already used. " +
		"They are not shown again.",
}

func init() {
	labelSets = append(labelSets,
		func() []string {
			var out []string
			for _, w := range statsWindows {
				out = append(out, w.Label)
			}
			return append(out, core.StatsLeaderboardMeasure)
		},
		func() []string {
			var out []string
			for _, k := range activityKinds {
				out = append(out, k.Label)
			}
			return out
		},
		func() []string {
			var out []string
			for _, c := range priorityChoices {
				out = append(out, c.Label)
			}
			return out
		},
		func() []string {
			out := append([]string(nil), recoveryLedes...)
			for _, v := range []totpView{
				signInChallenge("ticket", "", "", nil),
				signInChallenge("ticket", "", "otpauth://totp/x?secret=ABCD", nil),
				enrolment("", nil),
			} {
				out = append(out, v.Heading, v.Lede)
			}
			return out
		},
	)
}
