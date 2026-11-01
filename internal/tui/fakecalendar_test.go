// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"context"

	"github.com/heliopsy/tix/internal/core"
)

// The terminal interface has no calendar feed view, so the fake answers the
// calendar methods with nothing; a call reaching one would be a binding nobody
// declared.

func (f *fakeService) GetCalendarFeed(context.Context) (*core.CalendarFeed, error) {
	return nil, core.NotFound("calendar feed not found")
}

func (f *fakeService) CreateCalendarFeed(context.Context, core.CalendarFeedInput) (*core.IssuedCalendarFeed, error) {
	return nil, core.Invalid("not offered in the terminal interface")
}

func (f *fakeService) RotateCalendarFeed(context.Context) (*core.IssuedCalendarFeed, error) {
	return nil, core.Invalid("not offered in the terminal interface")
}

func (f *fakeService) SetCalendarFeedFilter(context.Context, core.CalendarFeedInput) (*core.CalendarFeed, error) {
	return nil, core.Invalid("not offered in the terminal interface")
}

func (f *fakeService) RevokeCalendarFeed(context.Context) error {
	return core.Invalid("not offered in the terminal interface")
}

func (f *fakeService) ReadCalendarFeed(context.Context, string) (*core.CalendarEntries, error) {
	return nil, core.Invalid("not offered in the terminal interface")
}
