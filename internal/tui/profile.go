// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import "github.com/charmbracelet/colorprofile"

// NewProfile reads the colour depth of one client's own terminal.
//
// A profile is per connection, so a process drawing several terminals at once
// resolves one each: share a profile and every client renders at the depth of
// whichever was measured first. Detection normally ends in a character-device
// test, which a network stream can never pass, so it is skipped here and the
// terminal type the client declared decides instead. environ is read last wins,
// which is how a session reports the terminal type of the pseudo-terminal it
// allocated.
func NewProfile(environ []string) colorprofile.Profile {
	return colorprofile.Env(environ)
}
