package api

import "testing"

func TestColumnStringNormalizesKeyValueShapes(t *testing.T) {
	column := ColumnDef{Type: ColumnTypeKeyValue}
	if got := ColumnString(column, map[string]any{"team": "core", "env": "prod"}); got != "env=prod, team=core" {
		t.Fatalf("map string = %q", got)
	}

	list := []any{
		map[string]any{"key": "env", "value": "prod"},
		map[string]any{"name": "env", "value": "staging"},
	}
	if got := ColumnString(ColumnDef{Type: ColumnTypeKeyValues}, list); got != "env=prod, env=staging" {
		t.Fatalf("pair list string = %q", got)
	}

	encoded := `[{"key":"region","value":"eu"}]`
	if got := ColumnString(ColumnDef{Type: ColumnTypeKeyValues}, encoded); got != "region=eu" {
		t.Fatalf("encoded pair list string = %q", got)
	}
}

func TestColumnTextableKeepsStructuredValues(t *testing.T) {
	value := ColumnTextable(ColumnDef{Type: ColumnTypeKeyValue}, map[string]any{"env": "prod"})
	list, ok := value.(DescriptionList)
	if !ok || len(list.Items) != 1 || list.Items[0].Key != "env" {
		t.Fatalf("expected description list, got %#v", value)
	}

	jsonValue := ColumnTextable(ColumnDef{Type: ColumnTypeJSON}, map[string]any{"enabled": true})
	code, ok := jsonValue.(Code)
	if !ok || code.Language != "json" || code.Content != `{"enabled":true}` {
		t.Fatalf("expected canonical JSON code, got %#v", jsonValue)
	}
}

func TestColumnTextableFallsBackForMalformedKeyValues(t *testing.T) {
	value := ColumnTextable(ColumnDef{Type: ColumnTypeKeyValues}, "not-json")
	if got := value.String(); got != "not-json" {
		t.Fatalf("fallback = %q", got)
	}
}
