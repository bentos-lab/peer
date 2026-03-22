package tracing

import (
	"encoding/json"
	"fmt"
)

func (g *Generator) tracef(format string, args ...any) {
	if g.logger == nil {
		return
	}
	g.logger.Tracef(format, args...)
}

func toCompactJSON(value any) string {
	raw, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprintf(`{"marshal_error":%q}`, err.Error())
	}
	return string(raw)
}
