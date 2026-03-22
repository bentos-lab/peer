package codingagent

import (
	"encoding/json"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v5"
)

func encodeSchema(schema map[string]any) (string, bool, error) {
	if len(schema) == 0 {
		return "", false, nil
	}
	raw, err := json.Marshal(schema)
	if err != nil {
		return "", false, err
	}
	return string(raw), true, nil
}

func compileSchema(schemaText string) (*jsonschema.Schema, error) {
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("schema.json", strings.NewReader(schemaText)); err != nil {
		return nil, err
	}
	return compiler.Compile("schema.json")
}
