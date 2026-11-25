// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"context"

	"github.com/heliopsy/tix/internal/core"
)

// The terminal interface has no CalDAV view, so the fake answers the app
// password methods with nothing; a call reaching one would be a binding nobody
// declared.

func (f *fakeService) GetDAVInfo(context.Context) (*core.DAVInfo, error) {
	return nil, core.Invalid("not offered in the terminal interface")
}

func (f *fakeService) ListDAVPasswords(context.Context) ([]core.DAVPassword, error) {
	return nil, core.Invalid("not offered in the terminal interface")
}

func (f *fakeService) CreateDAVPassword(context.Context, core.CreateDAVPasswordInput) (*core.IssuedDAVPassword, error) {
	return nil, core.Invalid("not offered in the terminal interface")
}

func (f *fakeService) RevokeDAVPassword(context.Context, string) error {
	return core.Invalid("not offered in the terminal interface")
}
