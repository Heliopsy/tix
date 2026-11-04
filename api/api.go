// SPDX-License-Identifier: AGPL-3.0-or-later

// Package api embeds the generated OpenAPI document of the HTTP API, so the
// server describes exactly the binary it is.
package api

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"sync"

	"go.yaml.in/yaml/v4"
)

//go:embed openapi.yaml
var document []byte

// YAML returns the OpenAPI document as it is committed.
func YAML() []byte { return document }

// JSON returns the OpenAPI document as JSON, converted once.
var JSON = sync.OnceValues(func() ([]byte, error) {
	var doc map[string]any
	if err := yaml.Unmarshal(document, &doc); err != nil {
		return nil, fmt.Errorf("reading the embedded openapi document: %w", err)
	}
	return json.Marshal(doc)
})
