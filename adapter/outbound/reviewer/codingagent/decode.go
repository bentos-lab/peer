package codingagent

import (
	"encoding/json"
	"fmt"
)

func decodeModelOutput(outputMap map[string]any, target any) error {
	raw, err := json.Marshal(outputMap)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(raw, target); err != nil {
		return fmt.Errorf("invalid formatted model output: %w", err)
	}
	return nil
}
