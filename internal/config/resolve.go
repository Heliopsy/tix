package config

import (
	"errors"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/output"
)

// Layer names one of the five configuration sources.
type Layer string

// Configuration layers, in descending precedence.
const (
	LayerFlag    Layer = "flag"
	LayerEnv     Layer = "environment"
	LayerDotenv  Layer = "dotenv"
	LayerFile    Layer = "file"
	LayerDefault Layer = "default"
)

// Options controls a configuration load.
type Options struct {
	Dir         string
	Home        string
	Environ     []string
	Flags       map[string]string
	Context     string
	NoDiscovery bool
}

// Resolved is a configuration together with the origin of every value.
type Resolved struct {
	Config      Config
	ConfigFile  string
	LocalFile   string
	DotenvFile  string
	ContextName string
	sources     map[string]Layer
}

// Entry is one key's effective value and the layer that supplied it.
type Entry struct {
	Key    string `json:"key"`
	Env    string `json:"env"`
	Value  string `json:"value"`
	Source Layer  `json:"source"`
	Secret bool   `json:"secret"`
}

// Source returns the layer that supplied a key, or "" when the key is unknown.
func (r *Resolved) Source(key string) Layer { return r.sources[key] }

// SourcesRaw returns every key with its unredacted value and origin layer.
func (r *Resolved) SourcesRaw() []Entry {
	cfg := r.Config
	out := make([]Entry, 0, len(keyList))
	for _, k := range keyList {
		out = append(out, Entry{
			Key:    k.Path,
			Env:    k.Env,
			Value:  k.Get(&cfg),
			Source: r.sources[k.Path],
			Secret: k.Secret(),
		})
	}
	return out
}

// Sources returns every key with its origin layer and secrets redacted.
func (r *Resolved) Sources() []Entry {
	out := r.SourcesRaw()
	for i, entry := range out {
		key, _ := Lookup(entry.Key)
		out[i].Value = redact(key.secret, entry.Value)
	}
	return out
}

// Load resolves configuration from flags, environment, .env, files and defaults.
func Load(opts Options) (*Resolved, error) {
	env := environMap(opts.Environ)
	dir, err := workingDir(opts.Dir)
	if err != nil {
		return nil, err
	}
	home := resolveHome(opts.Home, env)

	flagLayer, err := normalizeFlags(opts.Flags)
	if err != nil {
		return nil, err
	}
	envLayer := envValues(env)
	dotenvPath, dotenvLayer, err := dotenvLayerFor(dir, env)
	if err != nil {
		return nil, err
	}

	globalPath, err := FilePath(env, home)
	if err != nil {
		return nil, err
	}
	global, err := loadOptional(globalPath)
	if err != nil {
		return nil, err
	}

	fileLayer := copyValues(global.Values)
	above := []map[string]string{flagLayer, envLayer, dotenvLayer, fileLayer}

	localPath := ""
	local := &File{Values: map[string]string{}}
	if !opts.NoDiscovery && discoveryEnabled(above) {
		localPath = findUp(dir, discoveryFilenames(above))
		if localPath != "" {
			if local, err = LoadFile(localPath); err != nil {
				return nil, err
			}
			fileLayer = mergeValues(fileLayer, local.Values)
		}
	}

	contexts := mergeContexts(global.Contexts, local.Contexts)
	name, fromFlag := activeContextName(opts.Context, flagLayer, envLayer, dotenvLayer, local, global)
	if name != "" {
		ctx, ok := contexts[name]
		if !ok {
			if fromFlag {
				return nil, core.Invalid("context %q is not defined", name)
			}
			return nil, core.NotFound("context %q is not defined", name)
		}
		if err := ctx.validate(name); err != nil {
			return nil, err
		}
		fileLayer = mergeValues(fileLayer, contextValues(name, ctx))
	}

	resolved := &Resolved{
		Config:      Defaults(),
		ConfigFile:  globalPath,
		LocalFile:   localPath,
		DotenvFile:  dotenvPath,
		ContextName: name,
		sources:     make(map[string]Layer, len(keyList)),
	}
	if err := apply(resolved, flagLayer, envLayer, dotenvLayer, fileLayer); err != nil {
		return nil, err
	}
	resolved.Config.Contexts = contexts
	resolved.Config.Database.DSN = expandDSNHome(resolved.Config.Database.DSN, home)

	if err := Validate(&resolved.Config, resolved.sources); err != nil {
		return nil, err
	}
	return resolved, nil
}

func apply(r *Resolved, flags, envs, dotenvs, files map[string]string) error {
	defaults := Defaults()
	defaultValues := flatten(&defaults)
	ordered := []struct {
		layer  Layer
		values map[string]string
	}{
		{LayerFlag, flags},
		{LayerEnv, envs},
		{LayerDotenv, dotenvs},
		{LayerFile, files},
		{LayerDefault, defaultValues},
	}
	for _, k := range keyList {
		for _, candidate := range ordered {
			raw, ok := candidate.values[k.Path]
			if !ok {
				continue
			}
			if err := k.Set(&r.Config, raw); err != nil {
				return annotate(err, candidate.layer)
			}
			r.sources[k.Path] = candidate.layer
			break
		}
	}
	return nil
}

func annotate(err error, layer Layer) error {
	var domain *core.Error
	if errors.As(err, &domain) {
		return domain.WithDetail("layer", string(layer))
	}
	return err
}

func normalizeFlags(flags map[string]string) (map[string]string, error) {
	out := make(map[string]string, len(flags))
	for key, value := range flags {
		if _, ok := Lookup(key); !ok {
			return nil, core.Invalid("unknown configuration key %q set by flag", key)
		}
		out[key] = value
	}
	return out, nil
}

func envValues(env map[string]string) map[string]string {
	out := make(map[string]string)
	for _, k := range keyList {
		if value, ok := env[k.Env]; ok {
			out[k.Path] = value
		}
	}
	applyNoColor(out, func(name string) string { return env[name] })
	return out
}

// applyNoColor folds the NO_COLOR convention into the colour key. The explicit
// key wins over the convention, because it is the more specific statement.
func applyNoColor(values map[string]string, lookup func(string) string) {
	if _, explicit := values[KeyOutputColor]; explicit {
		return
	}
	if output.NoColorSet(lookup) {
		values[KeyOutputColor] = output.ColorNever
	}
}

func dotenvLayerFor(dir string, env map[string]string) (string, map[string]string, error) {
	out := map[string]string{}
	path := FindDotenv(dir)
	if path == "" {
		return "", out, nil
	}
	values, err := ParseDotenv(path)
	if err != nil {
		return "", nil, err
	}
	for _, k := range keyList {
		if _, exported := env[k.Env]; exported {
			continue
		}
		if value, ok := values[k.Env]; ok {
			out[k.Path] = value
		}
	}
	if !output.NoColorSet(func(name string) string { return env[name] }) {
		applyNoColor(out, func(name string) string { return values[name] })
	}
	return path, out, nil
}

func loadOptional(path string) (*File, error) {
	if path == "" {
		return &File{Values: map[string]string{}}, nil
	}
	return LoadFile(path)
}

func discoveryEnabled(layers []map[string]string) bool {
	if raw, ok := firstValue(layers, "discovery.enabled"); ok {
		enabled, err := strconv.ParseBool(strings.TrimSpace(raw))
		return err != nil || enabled
	}
	return true
}

func discoveryFilenames(layers []map[string]string) []string {
	if raw, ok := firstValue(layers, "discovery.filenames"); ok {
		if names := splitList(raw); len(names) > 0 {
			return names
		}
	}
	return DefaultDiscoveryFilenames
}

func firstValue(layers []map[string]string, key string) (string, bool) {
	for _, layer := range layers {
		if value, ok := layer[key]; ok {
			return value, true
		}
	}
	return "", false
}

func activeContextName(override string, flags, envs, dotenvs map[string]string, local, global *File) (string, bool) {
	if trimmed := strings.TrimSpace(override); trimmed != "" {
		return trimmed, true
	}
	if value, ok := firstValue([]map[string]string{flags, envs, dotenvs}, "current_context"); ok {
		return strings.TrimSpace(value), false
	}
	if local != nil && local.hasCurrent {
		return strings.TrimSpace(local.CurrentContext), false
	}
	if global != nil && global.hasCurrent {
		return strings.TrimSpace(global.CurrentContext), false
	}
	return "", false
}

func contextValues(name string, ctx Context) map[string]string {
	out := map[string]string{"current_context": name}
	assign := func(key, value string) {
		if strings.TrimSpace(value) != "" {
			out[key] = value
		}
	}
	assign("database.dsn", ctx.Database)
	assign("server.url", ctx.Server)
	assign("server.token", ctx.Token)
	assign("tenant", ctx.Tenant)
	assign("project", ctx.Project)
	assign("auth.mode", ctx.AuthMode)
	assign("hooks.mode", ctx.HookMode)
	return out
}

func mergeContexts(base, overlay map[string]Context) map[string]Context {
	if base == nil && overlay == nil {
		return nil
	}
	out := make(map[string]Context, len(base)+len(overlay))
	for name, ctx := range base {
		out[name] = ctx
	}
	for name, ctx := range overlay {
		out[name] = ctx
	}
	return out
}

func mergeValues(base, overlay map[string]string) map[string]string {
	out := copyValues(base)
	for key, value := range overlay {
		out[key] = value
	}
	return out
}

func copyValues(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func environMap(environ []string) map[string]string {
	if environ == nil {
		environ = os.Environ()
	}
	out := make(map[string]string, len(environ))
	for _, entry := range environ {
		if name, value, ok := strings.Cut(entry, "="); ok {
			out[name] = value
		}
	}
	return out
}

func workingDir(dir string) (string, error) {
	if strings.TrimSpace(dir) != "" {
		return dir, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", core.Internal("resolving working directory: %v", err)
	}
	return cwd, nil
}

func resolveHome(home string, env map[string]string) string {
	if strings.TrimSpace(home) != "" {
		return home
	}
	if value := strings.TrimSpace(env["HOME"]); value != "" {
		return value
	}
	value, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return value
}

func allowed(value string, set []string) bool { return slices.Contains(set, value) }
