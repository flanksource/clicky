package rpc

import (
	"encoding/json"
	"testing"
)

// envVarLike mimics commons-db's types.EnvVar: a struct that should render as a
// flat string in generated schemas via the SchemaDescriber interface.
type envVarLike struct {
	Name  string `json:"name,omitempty"`
	Value string `json:"value,omitempty"`
}

func (envVarLike) JSONSchema() map[string]any { return map[string]any{"type": "string"} }

type widgetStruct struct {
	URL      envVarLike `json:"url"      clicky:"type=k8s-url-selector,title=URL,source=value,required"`
	Password envVarLike `json:"password" clicky:"type=k8s-secret-selector,title=Password,format=password"`
	Region   string     `json:"region"   clicky:"title=Region,property=region"`
}

func marshalSchema(t *testing.T, s *OpenAPISchema) map[string]any {
	t.Helper()
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return m
}

func TestSchemaForStructWidgets(t *testing.T) {
	s := SchemaForStruct(widgetStruct{})
	m := marshalSchema(t, s)

	props, ok := m["properties"].(map[string]any)
	if !ok {
		t.Fatalf("expected properties object, got %T", m["properties"])
	}

	url := props["url"].(map[string]any)
	if url["type"] != "string" {
		t.Errorf("EnvVar-like url should render as string, got %v", url["type"])
	}
	if url["x-clicky-component"] != "k8s-url-selector" {
		t.Errorf("url component = %v, want k8s-url-selector", url["x-clicky-component"])
	}
	if url["x-clicky-default-source"] != "value" {
		t.Errorf("url default-source = %v, want value", url["x-clicky-default-source"])
	}
	if url["title"] != "URL" {
		t.Errorf("url title = %v, want URL", url["title"])
	}

	pw := props["password"].(map[string]any)
	if pw["format"] != "password" {
		t.Errorf("password format = %v, want password", pw["format"])
	}
	if pw["x-clicky-component"] != "k8s-secret-selector" {
		t.Errorf("password component = %v, want k8s-secret-selector", pw["x-clicky-component"])
	}

	region := props["region"].(map[string]any)
	if region["x-clicky-property"] != "region" {
		t.Errorf("region x-clicky-property = %v, want region", region["x-clicky-property"])
	}

	required, _ := m["required"].([]any)
	if len(required) != 1 || required[0] != "url" {
		t.Errorf("required = %v, want [url]", required)
	}
}

type orderedCreds struct {
	Username string `json:"username" clicky:"title=Username,order=4"`
	Password string `json:"password" clicky:"title=Password,order=5"`
}

type orderedStruct struct {
	Name string `json:"name" clicky:"title=Name,order=0"`
	URL  string `json:"url"  clicky:"title=URL,order=2"`
	orderedCreds
	Note string `json:"note" clicky:"title=Note"`
}

func TestSchemaForStructOrder(t *testing.T) {
	s := SchemaForStruct(orderedStruct{})
	m := marshalSchema(t, s)
	props := m["properties"].(map[string]any)

	// A field tagged order=N carries x-clicky-order=N (a JSON number).
	want := map[string]float64{"name": 0, "url": 2, "username": 4, "password": 5}
	for key, n := range want {
		got, ok := props[key].(map[string]any)["x-clicky-order"]
		if !ok {
			t.Errorf("%s: missing x-clicky-order", key)
			continue
		}
		if got != n {
			t.Errorf("%s: x-clicky-order = %v, want %v", key, got, n)
		}
	}

	// An untagged field (no order=) carries no x-clicky-order.
	if _, ok := props["note"].(map[string]any)["x-clicky-order"]; ok {
		t.Errorf("note should not carry x-clicky-order")
	}
}

// ptrDescriber implements SchemaDescriber on its POINTER receiver only, so the
// value type does not satisfy the interface.
type ptrDescriber struct {
	Raw string `json:"raw"`
}

func (*ptrDescriber) JSONSchema() map[string]any {
	return map[string]any{"type": "string", "format": "uuid"}
}

type ptrDescriberHolder struct {
	ID ptrDescriber `json:"id"`
}

// TestSchemaForStructPointerReceiverDescriber guards the pointer-receiver path:
// the reflect code must construct a *ptrDescriber (not its Elem) or the
// SchemaDescriber assertion panics.
func TestSchemaForStructPointerReceiverDescriber(t *testing.T) {
	s := SchemaForStruct(ptrDescriberHolder{}) // must not panic
	m := marshalSchema(t, s)
	id := m["properties"].(map[string]any)["id"].(map[string]any)
	if id["type"] != "string" || id["format"] != "uuid" {
		t.Errorf("id schema = %v, want {type:string, format:uuid}", id)
	}
}

func TestSchemaFromMapExtensions(t *testing.T) {
	s := schemaFromMap(map[string]any{
		"type":               "string",
		"x-clicky-component": "k8s-secret-selector",
	})
	if s.Type != "string" {
		t.Errorf("type = %q, want string", s.Type)
	}
	if s.Extensions["x-clicky-component"] != "k8s-secret-selector" {
		t.Errorf("extension not captured: %v", s.Extensions)
	}
	m := marshalSchema(t, s)
	if m["x-clicky-component"] != "k8s-secret-selector" {
		t.Errorf("extension not marshalled inline: %v", m)
	}
}
