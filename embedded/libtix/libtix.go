// SPDX-License-Identifier: AGPL-3.0-or-later

// Command libtix is the C ABI of tix for dart:ffi and other foreign callers,
// built with -buildmode=c-shared or -buildmode=c-archive. It is the only cgo
// in this repository, and nothing the tix binary is built from imports it.
// Every function but TixFree returns a JSON string the caller releases with
// TixFree. See docs/embedding.md.
package main

/*
#include <stdlib.h>
*/
import "C"

import (
	"unsafe"

	"github.com/heliopsy/tix/embedded/capi"
)

func main() {}

func out(s string) *C.char { return C.CString(s) }

// TixStart starts tix from {"db_path","listen","init_if_missing"} and returns
// {"handle","url"} or {"error":{"code","message"}}.
//
//export TixStart
func TixStart(options *C.char) *C.char { return out(capi.Start(C.GoString(options))) }

// TixStop stops the instance behind handle and returns {} or an error.
//
//export TixStop
func TixStop(handle C.longlong) *C.char { return out(capi.Stop(int64(handle))) }

// TixTenants returns {"tenants":[{"key","name"}]} or an error.
//
//export TixTenants
func TixTenants(handle C.longlong) *C.char { return out(capi.Tenants(int64(handle))) }

// TixCreateTenant creates a tenant from {"key","name"} and returns it or an error.
//
//export TixCreateTenant
func TixCreateTenant(handle C.longlong, request *C.char) *C.char {
	return out(capi.CreateTenant(int64(handle), C.GoString(request)))
}

// TixToken mints a token from {"tenant","name","scopes","ttl"} and returns
// {"token"} or an error.
//
//export TixToken
func TixToken(handle C.longlong, request *C.char) *C.char {
	return out(capi.Token(int64(handle), C.GoString(request)))
}

// TixVersion returns {"version","commit"}.
//
//export TixVersion
func TixVersion() *C.char { return out(capi.Version()) }

// TixFree releases a string returned by any other function. NULL is ignored.
//
//export TixFree
func TixFree(p *C.char) { C.free(unsafe.Pointer(p)) }
