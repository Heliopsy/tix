// SPDX-License-Identifier: AGPL-3.0-or-later

package output

import (
	"encoding/json"
	"fmt"
	"io"
)

type jsonFormatter struct{}

// Format marshals data to indented JSON terminated by a newline.
func (j *jsonFormatter) Format(w io.Writer, data any) error {
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling json: %w", err)
	}
	if _, err := w.Write(append(b, '\n')); err != nil {
		return fmt.Errorf("writing json: %w", err)
	}
	return nil
}
