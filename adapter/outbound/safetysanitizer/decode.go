package safetysanitizer

import (
	"encoding/json"
	"fmt"
)

func decodeSanitizerOutput(output map[string]any, target any) error {
	raw, err := jsonMarshal(output)
	if err != nil {
		return err
	}
	if err := jsonUnmarshal(raw, target); err != nil {
		return fmt.Errorf("invalid sanitizer output: %w", err)
	}
	return nil
}

var jsonMarshal = func(value any) ([]byte, error) { return json.Marshal(value) }
var jsonUnmarshal = func(data []byte, target any) error { return json.Unmarshal(data, target) }
