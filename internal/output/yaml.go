package output

import (
	"fmt"
	"io"

	"gopkg.in/yaml.v3"
)

type yamlFormatter struct{}

// Format marshals data to YAML.
func (y *yamlFormatter) Format(w io.Writer, data any) error {
	b, err := marshalYAML(data)
	if err != nil {
		return err
	}
	if _, err := w.Write(b); err != nil {
		return fmt.Errorf("writing yaml: %w", err)
	}
	return nil
}

// marshalYAML converts yaml.v3's panic on unsupported types into an error.
func marshalYAML(data any) (b []byte, err error) {
	defer func() {
		if r := recover(); r != nil {
			b, err = nil, fmt.Errorf("marshalling yaml: %v", r)
		}
	}()
	b, err = yaml.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("marshalling yaml: %w", err)
	}
	return b, nil
}
