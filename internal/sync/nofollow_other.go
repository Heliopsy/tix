// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build !unix

package sync

// noFollow is empty where the platform has no such flag; ReadConfined's
// EvalSymlinks check and os.Root still keep the open inside the directory.
const noFollow = 0
