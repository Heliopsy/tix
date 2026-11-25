// SPDX-License-Identifier: AGPL-3.0-or-later

package web

// The words assets/notify.js writes itself: a notification and the state of
// its two switches on Settings.
func init() {
	jsMessages = append(jsMessages,
		"Something new reached your inbox.",
		"somebody",
		"%s mentioned you",
		"%s assigned you",
		"%s moved it from %s to %s",
		"%s commented",
		"On",
		"Off",
		"This browser cannot show notifications.",
		"Notifications are blocked for this site. Allow them in the browser's site settings, then turn this on.",
	)
}
