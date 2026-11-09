// SPDX-License-Identifier: AGPL-3.0-or-later

// Package capi is the C ABI of the embedded package, written as ordinary Go:
// JSON strings in and out, instances addressed by integer handles, and no
// panic ever returned to the caller. embedded/libtix exports it to C.
package capi

import (
	"bytes"
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/heliopsy/tix/embedded"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/version"
)

// StopTimeout bounds how long Stop waits for an instance to drain.
const StopTimeout = 30 * time.Second

var (
	mu        sync.Mutex
	next      int64
	instances = map[int64]*embedded.Instance{}
)

// errorBody is the object every failure is returned as.
type errorBody struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// call runs fn and renders its result or its failure, a panic included, as JSON.
func call(fn func() (any, error)) (out string) {
	defer func() {
		if r := recover(); r != nil {
			out = failure(core.Internal("panic in tix: %v", r))
		}
	}()
	v, err := fn()
	if err != nil {
		return failure(err)
	}
	b, err := json.Marshal(v)
	if err != nil {
		return failure(core.Internal("encoding the answer").Wrap(err))
	}
	return string(b)
}

func failure(err error) string {
	var body errorBody
	body.Error.Code = embedded.ErrorCode(err)
	body.Error.Message = embedded.ErrorMessage(err)
	b, _ := json.Marshal(body) //nolint:errchkjson // two strings always marshal
	return string(b)
}

// decode reads one JSON object into v, refusing unknown fields so a misspelt
// option is an error rather than a default.
func decode(in string, v any) error {
	dec := json.NewDecoder(bytes.NewReader([]byte(in)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return core.Invalid("reading the request json: %v", err)
	}
	if dec.More() {
		return core.Invalid("reading the request json: more than one value")
	}
	return nil
}

func lookup(handle int64) (*embedded.Instance, error) {
	mu.Lock()
	defer mu.Unlock()
	in, ok := instances[handle]
	if !ok {
		return nil, core.NotFound("no running instance has handle %d", handle)
	}
	return in, nil
}

type startRequest struct {
	DBPath        string `json:"db_path"`
	Listen        string `json:"listen"`
	InitIfMissing bool   `json:"init_if_missing"`
	JournalMode   string `json:"journal_mode"`
}

type started struct {
	Handle int64  `json:"handle"`
	URL    string `json:"url"`
}

// Start starts an instance from {"db_path","listen","init_if_missing",
// "journal_mode"} and answers {"handle","url"}.
func Start(options string) string {
	return call(func() (any, error) {
		var req startRequest
		if err := decode(options, &req); err != nil {
			return nil, err
		}
		in, err := embedded.Start(context.Background(), embedded.Options{
			DBPath: req.DBPath, Listen: req.Listen, InitIfMissing: req.InitIfMissing,
			JournalMode: req.JournalMode,
		})
		if err != nil {
			return nil, err
		}
		mu.Lock()
		next++
		handle := next
		instances[handle] = in
		mu.Unlock()
		return started{Handle: handle, URL: in.URL()}, nil
	})
}

// Stop stops the instance and forgets its handle, answering {}.
func Stop(handle int64) string {
	return call(func() (any, error) {
		mu.Lock()
		in, ok := instances[handle]
		delete(instances, handle)
		mu.Unlock()
		if !ok {
			return nil, core.NotFound("no running instance has handle %d", handle)
		}
		ctx, cancel := context.WithTimeout(context.Background(), StopTimeout)
		defer cancel()
		if err := in.Stop(ctx); err != nil {
			return nil, err
		}
		return struct{}{}, nil
	})
}

// Tenants answers {"tenants":[{"key","name"}]}.
func Tenants(handle int64) string {
	return call(func() (any, error) {
		in, err := lookup(handle)
		if err != nil {
			return nil, err
		}
		tenants, err := in.Tenants(context.Background())
		if err != nil {
			return nil, err
		}
		if tenants == nil {
			tenants = []embedded.Tenant{}
		}
		return struct {
			Tenants []embedded.Tenant `json:"tenants"`
		}{tenants}, nil
	})
}

// CreateTenant creates a tenant from {"key","name"} and answers it.
func CreateTenant(handle int64, request string) string {
	return call(func() (any, error) {
		var req embedded.Tenant
		if err := decode(request, &req); err != nil {
			return nil, err
		}
		in, err := lookup(handle)
		if err != nil {
			return nil, err
		}
		return in.CreateTenant(context.Background(), req.Key, req.Name)
	})
}

type tokenRequest struct {
	Tenant string   `json:"tenant"`
	Name   string   `json:"name"`
	Scopes []string `json:"scopes"`
	TTL    string   `json:"ttl"`
}

// Token mints a token from {"tenant","name","scopes","ttl"} and answers
// {"token"}.
func Token(handle int64, request string) string {
	return call(func() (any, error) {
		var req tokenRequest
		if err := decode(request, &req); err != nil {
			return nil, err
		}
		ttl, err := ParseTTL(req.TTL)
		if err != nil {
			return nil, err
		}
		in, err := lookup(handle)
		if err != nil {
			return nil, err
		}
		token, err := in.Token(context.Background(), embedded.TokenRequest{
			Tenant: req.Tenant, Name: req.Name, Scopes: req.Scopes, TTL: ttl,
		})
		if err != nil {
			return nil, err
		}
		return struct {
			Token string `json:"token"`
		}{token}, nil
	})
}

// ParseTTL reads a token lifetime: empty for the default, "never", a Go
// duration such as 720h, or a whole number of days such as 90d.
func ParseTTL(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	switch {
	case s == "":
		return 0, nil
	case s == "never":
		return -1, nil
	case strings.HasSuffix(s, "d"):
		days, err := strconv.Atoi(strings.TrimSuffix(s, "d"))
		if err != nil || days <= 0 {
			return 0, core.Invalid("ttl %q: want a positive number of days, a duration such as 720h, or never", s)
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil || d <= 0 {
		return 0, core.Invalid("ttl %q: want a positive duration such as 720h, a number of days such as 90d, or never", s)
	}
	return d, nil
}

// Version answers {"version","commit"} of the library.
func Version() string {
	return call(func() (any, error) {
		return struct {
			Version string `json:"version"`
			Commit  string `json:"commit"`
		}{version.Version, version.Commit}, nil
	})
}
