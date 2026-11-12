package flags

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/pflag"
)

const schemaAnnotation = "clicky/schema"

// SetSchema retains a flag's JSON Schema for operation and form consumers.
func SetSchema(flag *pflag.Flag, schema map[string]any) error {
	data, err := json.Marshal(schema)
	if err != nil {
		return fmt.Errorf("flag %s schema: %w", flag.Name, err)
	}
	if schema == nil {
		return fmt.Errorf("flag %s requires a schema object", flag.Name)
	}
	if flag.Annotations == nil {
		flag.Annotations = map[string][]string{}
	}
	flag.Annotations[schemaAnnotation] = []string{string(data)}
	return nil
}

func Schema(flag *pflag.Flag) (map[string]any, error) {
	values := flag.Annotations[schemaAnnotation]
	if len(values) == 0 {
		return nil, nil
	}
	if len(values) != 1 {
		return nil, fmt.Errorf("flag %s has %d schema annotations", flag.Name, len(values))
	}
	// UseNumber keeps integer constraints such as const/default exact.
	var schema map[string]any
	decoder := json.NewDecoder(strings.NewReader(values[0]))
	decoder.UseNumber()
	if err := decoder.Decode(&schema); err != nil {
		return nil, fmt.Errorf("flag %s schema: %w", flag.Name, err)
	}
	if schema == nil {
		return nil, fmt.Errorf("flag %s requires a schema object", flag.Name)
	}
	return schema, nil
}
