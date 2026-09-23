// SPDX-License-Identifier: AGPL-3.0-or-later

package config

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// DotenvName is the filename looked for when loading a dotenv file.
const DotenvName = ".env"

// FindDotenv returns the nearest .env at or above dir, or "" when none exists.
func FindDotenv(dir string) string { return findUp(dir, []string{DotenvName}) }

// ParseDotenv reads KEY=VALUE pairs from a dotenv file.
func ParseDotenv(path string) (map[string]string, error) {
	f, err := os.Open(path) // #nosec G304 -- the path is discovered from the user's own tree
	if err != nil {
		return nil, fmt.Errorf("reading dotenv %q: %w", path, err)
	}
	defer func() { _ = f.Close() }()

	out := make(map[string]string)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		name, value, ok := parseDotenvLine(scanner.Text())
		if ok {
			out[name] = value
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading dotenv %q: %w", path, err)
	}
	return out, nil
}

func parseDotenvLine(line string) (string, string, bool) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return "", "", false
	}
	trimmed = strings.TrimPrefix(trimmed, "export ")
	name, value, found := strings.Cut(trimmed, "=")
	name = strings.TrimSpace(name)
	if !found || name == "" {
		return "", "", false
	}
	return name, unquoteDotenv(strings.TrimSpace(value)), true
}

func unquoteDotenv(value string) string {
	if len(value) >= 2 && value[0] == '\'' && value[len(value)-1] == '\'' {
		return value[1 : len(value)-1]
	}
	if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
		inner := value[1 : len(value)-1]
		return strings.NewReplacer(`\n`, "\n", `\t`, "\t", `\"`, `"`, `\\`, `\`).Replace(inner)
	}
	if idx := strings.Index(value, " #"); idx >= 0 {
		return strings.TrimSpace(value[:idx])
	}
	return value
}
