package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/thereisnotime/tix/internal/core"
	"gopkg.in/yaml.v3"
)

// RelativeConfigPath is the configuration file path inside a config home.
const RelativeConfigPath = "tix/config.yaml"

// File is one parsed configuration file.
type File struct {
	Path           string
	Values         map[string]string
	Contexts       map[string]Context
	CurrentContext string
	hasCurrent     bool
}

// FilePath returns the configuration file to use, or "" when none exists.
func FilePath(env map[string]string, home string) (string, error) {
	if explicit := strings.TrimSpace(env[EnvConfigFile]); explicit != "" {
		path := expandHome(explicit, home)
		if _, err := os.Stat(path); err != nil {
			return "", core.Invalid("%s names %q which does not exist", EnvConfigFile, path)
		}
		return path, nil
	}
	var candidates []string
	if xdg := strings.TrimSpace(env["XDG_CONFIG_HOME"]); xdg != "" {
		candidates = append(candidates, filepath.Join(expandHome(xdg, home), RelativeConfigPath))
	}
	if home != "" {
		candidates = append(candidates, filepath.Join(home, ".config", RelativeConfigPath))
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, nil
		}
	}
	return "", nil
}

// LoadFile parses a configuration file into flat values and contexts.
func LoadFile(path string) (*File, error) {
	raw, err := os.ReadFile(path) // #nosec G304 -- the path is chosen by the user
	if err != nil {
		return nil, fmt.Errorf("reading config %q: %w", path, err)
	}
	var tree map[string]any
	if err := yaml.Unmarshal(raw, &tree); err != nil {
		return nil, core.Invalid("parsing config %q: %v", path, err)
	}

	doc := &File{Path: path, Values: make(map[string]string)}
	if node, ok := tree["contexts"]; ok {
		delete(tree, "contexts")
		contexts, cerr := decodeContexts(node, path)
		if cerr != nil {
			return nil, cerr
		}
		doc.Contexts = contexts
	}
	if err := flattenTree(tree, "", doc.Values, path); err != nil {
		return nil, err
	}
	doc.CurrentContext, doc.hasCurrent = doc.Values["current_context"]
	return doc, nil
}

func decodeContexts(node any, path string) (map[string]Context, error) {
	encoded, err := yaml.Marshal(node)
	if err != nil {
		return nil, core.Invalid("parsing contexts in %q: %v", path, err)
	}
	var contexts map[string]Context
	decoder := yaml.NewDecoder(strings.NewReader(string(encoded)))
	decoder.KnownFields(true)
	if err := decoder.Decode(&contexts); err != nil {
		return nil, core.Invalid("parsing contexts in %q: %v", path, err)
	}
	return contexts, nil
}

func flattenTree(tree map[string]any, prefix string, out map[string]string, path string) error {
	names := make([]string, 0, len(tree))
	for name := range tree {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		key := name
		if prefix != "" {
			key = prefix + "." + name
		}
		switch value := tree[name].(type) {
		case map[string]any:
			if err := flattenTree(value, key, out, path); err != nil {
				return err
			}
		default:
			if _, known := Lookup(key); !known {
				return core.Invalid("unknown key %q in config %q", key, path)
			}
			out[key] = scalarString(value)
		}
	}
	return nil
}

func scalarString(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return v
	case bool:
		return strconv.FormatBool(v)
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case []any:
		parts := make([]string, len(v))
		for i, item := range v {
			parts[i] = scalarString(item)
		}
		return strings.Join(parts, ",")
	default:
		return fmt.Sprint(v)
	}
}

// Save writes cfg to path atomically with owner-only permissions.
func Save(path string, cfg *Config) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("creating config directory %q: %w", dir, err)
	}
	encoded, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("encoding config %q: %w", path, err)
	}
	temp, err := os.CreateTemp(dir, ".tix-config-*")
	if err != nil {
		return fmt.Errorf("creating temp config in %q: %w", dir, err)
	}
	tempName := temp.Name()
	defer func() { _ = os.Remove(tempName) }()

	if err := temp.Chmod(0o600); err != nil {
		_ = temp.Close()
		return fmt.Errorf("securing temp config %q: %w", tempName, err)
	}
	if _, err := temp.Write(encoded); err != nil {
		_ = temp.Close()
		return fmt.Errorf("writing temp config %q: %w", tempName, err)
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return fmt.Errorf("syncing temp config %q: %w", tempName, err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("closing temp config %q: %w", tempName, err)
	}
	if err := os.Rename(tempName, path); err != nil {
		return fmt.Errorf("replacing config %q: %w", path, err)
	}
	return nil
}
