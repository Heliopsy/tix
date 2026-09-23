// SPDX-License-Identifier: AGPL-3.0-or-later

package config

import (
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/heliopsy/tix/internal/core"
)

// EnvPrefix is prepended to every generated environment variable name.
const EnvPrefix = "TIX_"

// EnvConfigFile names the variable holding an explicit configuration path.
const EnvConfigFile = EnvPrefix + "CONFIG"

// KeyOutputColor is the key deciding whether output is coloured.
const KeyOutputColor = "output.color"

type valueKind int

const (
	kindString valueKind = iota
	kindBool
	kindInt
	kindDuration
	kindStrings
)

type secrecy int

const (
	secretNone secrecy = iota
	secretOpaque
	secretDSN
)

// Key describes one configuration key and its generated environment variable.
type Key struct {
	Path   string
	Env    string
	kind   valueKind
	secret secrecy
	index  []int
}

// Secret reports whether the key's value must be redacted when displayed.
func (k Key) Secret() bool { return k.secret != secretNone }

var durationType = reflect.TypeOf(core.Duration(0))

var (
	keyList  []Key
	keyIndex map[string]Key
)

func init() {
	keyList = walkType(reflect.TypeOf(Config{}), "", nil)
	sort.Slice(keyList, func(i, j int) bool { return keyList[i].Path < keyList[j].Path })
	keyIndex = make(map[string]Key, len(keyList))
	byEnv := make(map[string]string, len(keyList))
	for _, k := range keyList {
		if prev, dup := byEnv[k.Env]; dup {
			panic(fmt.Sprintf("config: keys %q and %q both generate %s", prev, k.Path, k.Env))
		}
		byEnv[k.Env] = k.Path
		keyIndex[k.Path] = k
	}
}

// Keys returns every configuration key, sorted by key path.
func Keys() []Key { return append([]Key(nil), keyList...) }

// Lookup returns the key with the given path.
func Lookup(path string) (Key, bool) {
	k, ok := keyIndex[path]
	return k, ok
}

// EnvName derives the environment variable name for a key path.
func EnvName(path string) string {
	return EnvPrefix + strings.ToUpper(strings.NewReplacer(".", "_", "-", "_").Replace(path))
}

func walkType(t reflect.Type, prefix string, index []int) []Key {
	var out []Key
	for i := range t.NumField() {
		f := t.Field(i)
		name := yamlName(f)
		if name == "" {
			continue
		}
		idx := append(append([]int(nil), index...), i)
		path := name
		if prefix != "" {
			path = prefix + "." + name
		}
		if f.Type.Kind() == reflect.Struct {
			out = append(out, walkType(f.Type, path, idx)...)
			continue
		}
		kind, ok := kindOf(f.Type)
		if !ok {
			continue
		}
		out = append(out, Key{Path: path, Env: EnvName(path), kind: kind, secret: secrecyOf(f), index: idx})
	}
	return out
}

func yamlName(f reflect.StructField) string {
	if !f.IsExported() {
		return ""
	}
	tag := f.Tag.Get("yaml")
	if tag == "-" {
		return ""
	}
	name, _, _ := strings.Cut(tag, ",")
	if name == "" {
		name = strings.ToLower(f.Name)
	}
	return name
}

func kindOf(t reflect.Type) (valueKind, bool) {
	if t == durationType {
		return kindDuration, true
	}
	switch t.Kind() {
	case reflect.String:
		return kindString, true
	case reflect.Bool:
		return kindBool, true
	case reflect.Int, reflect.Int64:
		return kindInt, true
	case reflect.Slice:
		if t.Elem().Kind() == reflect.String {
			return kindStrings, true
		}
		return 0, false
	default:
		return 0, false
	}
}

func secrecyOf(f reflect.StructField) secrecy {
	switch f.Tag.Get("secret") {
	case "opaque":
		return secretOpaque
	case "dsn":
		return secretDSN
	default:
		return secretNone
	}
}

func (k Key) field(cfg *Config) reflect.Value {
	return reflect.ValueOf(cfg).Elem().FieldByIndex(k.index)
}

// Get returns the key's value from cfg rendered as a string.
func (k Key) Get(cfg *Config) string {
	v := k.field(cfg)
	switch k.kind {
	case kindString:
		return v.String()
	case kindBool:
		return strconv.FormatBool(v.Bool())
	case kindInt:
		return strconv.FormatInt(v.Int(), 10)
	case kindDuration:
		return core.Duration(v.Int()).String()
	case kindStrings:
		out := make([]string, v.Len())
		for i := range out {
			out[i] = v.Index(i).String()
		}
		return strings.Join(out, ",")
	default:
		return ""
	}
}

// Set parses raw and stores it in cfg.
func (k Key) Set(cfg *Config, raw string) error {
	v := k.field(cfg)
	switch k.kind {
	case kindString:
		v.SetString(raw)
	case kindBool:
		parsed, err := strconv.ParseBool(strings.TrimSpace(raw))
		if err != nil {
			return core.Invalid("key %q got %q, expected a boolean such as true or false", k.Path, raw)
		}
		v.SetBool(parsed)
	case kindInt:
		parsed, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
		if err != nil {
			return core.Invalid("key %q got %q, expected an integer", k.Path, raw)
		}
		v.SetInt(parsed)
	case kindDuration:
		parsed, err := time.ParseDuration(strings.TrimSpace(raw))
		if err != nil {
			return core.Invalid("key %q got %q, expected a duration such as 15m or 24h", k.Path, raw)
		}
		v.SetInt(int64(parsed))
	case kindStrings:
		v.Set(reflect.ValueOf(splitList(raw)))
	}
	return nil
}

func splitList(raw string) []string {
	var out []string
	for _, part := range strings.Split(raw, ",") {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func mustDuration(s string) core.Duration {
	d, err := time.ParseDuration(s)
	if err != nil {
		panic("config: bad built-in duration " + s)
	}
	return core.Duration(d)
}

// flatten renders cfg as a key path to string map covering every key.
func flatten(cfg *Config) map[string]string {
	out := make(map[string]string, len(keyList))
	for _, k := range keyList {
		out[k.Path] = k.Get(cfg)
	}
	return out
}
