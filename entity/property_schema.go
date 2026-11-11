package entity

import "encoding/json"

func (p Property) MarshalJSON() ([]byte, error) {
	type plain Property
	data, err := json.Marshal(plain(p))
	if err != nil || len(p.Extensions) == 0 {
		return data, err
	}
	var fields map[string]any
	if err := json.Unmarshal(data, &fields); err != nil {
		return nil, err
	}
	for key, value := range p.Extensions {
		fields[key] = value
	}
	return json.Marshal(fields)
}
