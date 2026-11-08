// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build unix

package sync

import "syscall"

// noFollow refuses to open a symbolic link as the final path component.
const noFollow = syscall.O_NOFOLLOW
