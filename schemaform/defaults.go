package schemaform

// Ported from clicky-ui json-schema-form-defaults.ts.

// ApplyDefaults fills missing values from `default` keywords. It walks fixed
// object properties recursively and re-evaluates if/then branches after each
// pass, so a defaulted discriminator can activate defaults in its selected
// branch. Existing values, including null and empty strings, are never
// replaced. value is not mutated.
func ApplyDefaults(schema *Schema, value map[string]any) map[string]any {
	return applyObjectDefaults(schema, value, map[*Schema]bool{})
}

func applyObjectDefaults(schema *Schema, value map[string]any, ancestors map[*Schema]bool) map[string]any {
	if ancestors[schema] {
		return value
	}
	next := map[*Schema]bool{schema: true}
	for k := range ancestors {
		next[k] = true
	}
	current := value
	maxPasses := min(100, len(schema.Properties)+len(schema.AllOf)+2)
	for pass := 0; pass < maxPasses; pass++ {
		changed := false
		props := Effective(schema, current)
		for _, key := range props.Keys {
			prop := props.Map[key]
			existing, present := current[key]
			fieldValue, fieldChanged := existing, false
			if !present && prop.Default != nil {
				fieldValue, fieldChanged = cloneValue(prop.Default), true
			}
			if rendersAsObject(prop) && !isOpenStringMap(prop) {
				object, ok := asObject(fieldValue)
				if !ok {
					object = map[string]any{}
				}
				// Defaults only ever add keys, so equality means nothing applied.
				if defaulted := applyObjectDefaults(prop, object, next); !jsonEqual(defaulted, object) {
					fieldValue, fieldChanged = defaulted, true
				}
			}
			if fieldChanged {
				if !changed {
					current = shallowCopy(current)
					changed = true
				}
				current[key] = fieldValue
			}
		}
		if !changed {
			break
		}
	}
	return current
}

func shallowCopy(m map[string]any) map[string]any {
	out := make(map[string]any, len(m)+1)
	for k, v := range m {
		out[k] = v
	}
	return out
}
