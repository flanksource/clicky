package entity

import "encoding/json"

// MarshalJSON merges Extensions into the property object. Standard fields are
// kept as raw JSON so merging never re-encodes (and rounds) their numbers.
func (p Property) MarshalJSON() ([]byte, error) {
	type plain Property
	data, err := json.Marshal(plain(p))
	if err != nil || len(p.Extensions) == 0 {
		return data, err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return nil, err
	}
	for key, value := range p.Extensions {
		raw, err := json.Marshal(value)
		if err != nil {
			return nil, err
		}
		fields[key] = raw
	}
	return json.Marshal(fields)
}
