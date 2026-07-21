package entitydemo

import (
	"fmt"
	"strconv"
	"strings"
)

func containsAll(have, want []string) bool {
	if len(want) == 0 {
		return true
	}
	set := make(map[string]struct{}, len(have))
	for _, value := range have {
		set[strings.ToLower(strings.TrimSpace(value))] = struct{}{}
	}
	for _, value := range want {
		if _, ok := set[strings.ToLower(strings.TrimSpace(value))]; !ok {
			return false
		}
	}
	return true
}

func canonicalTeam(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	switch value {
	case "", "team/platform":
		return "team/platform"
	case "platform":
		return "team/platform"
	case "team/core":
		return "team/core"
	case "core":
		return "team/core"
	case "team/data":
		return "team/data"
	case "data":
		return "team/data"
	default:
		if strings.HasPrefix(value, "team/") {
			return value
		}
		return "team/" + value
	}
}

func labelFromCanonicalTeam(value string) string {
	return titleCase(strings.TrimPrefix(value, "team/"))
}

func canonicalStatus(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	switch value {
	case "", "status:healthy":
		return "status:healthy"
	case "healthy":
		return "status:healthy"
	case "status:degraded":
		return "status:degraded"
	case "degraded":
		return "status:degraded"
	case "status:paused":
		return "status:paused"
	case "paused":
		return "status:paused"
	default:
		if strings.HasPrefix(value, "status:") {
			return value
		}
		return "status:" + value
	}
}

func labelFromCanonicalStatus(value string) string {
	return titleCase(strings.TrimPrefix(value, "status:"))
}

func statusStyle(value string) string {
	switch value {
	case "status:healthy":
		return "text-green-600 font-semibold"
	case "status:degraded":
		return "text-amber-600 font-semibold"
	case "status:paused":
		return "text-slate-500"
	default:
		return "text-slate-700"
	}
}

func syntheticEvents(name string, count int) []string {
	if count <= 0 {
		return nil
	}
	events := make([]string, 0, count)
	for i := 0; i < count; i++ {
		events = append(events, fmt.Sprintf("%s event %d", name, i+1))
	}
	return events
}

func lastEntry(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[len(values)-1]
}

func copyStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	out := make(map[string]string, len(values))
	for key, value := range values {
		out[key] = value
	}
	return out
}

func intFlag(flags map[string]string, key string, fallback int) int {
	if value, err := strconv.Atoi(flags[key]); err == nil {
		return value
	}
	return fallback
}

func boolFlag(flags map[string]string, key string) bool {
	value, err := strconv.ParseBool(flags[key])
	return err == nil && value
}

func boolFlagDefault(flags map[string]string, key string, fallback bool) bool {
	value, err := strconv.ParseBool(flags[key])
	if err != nil {
		return fallback
	}
	return value
}

func stringValue(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case fmt.Stringer:
		return strings.TrimSpace(typed.String())
	case nil:
		return ""
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", typed))
	}
}

func sliceValue(value any) []string {
	switch typed := value.(type) {
	case []string:
		return append([]string(nil), typed...)
	case []any:
		items := make([]string, 0, len(typed))
		for _, item := range typed {
			if value := stringValue(item); value != "" {
				items = append(items, value)
			}
		}
		return items
	case string:
		if typed == "" {
			return nil
		}
		parts := strings.Split(typed, ",")
		items := make([]string, 0, len(parts))
		for _, part := range parts {
			if value := strings.TrimSpace(part); value != "" {
				items = append(items, value)
			}
		}
		return items
	default:
		value := stringValue(value)
		if value == "" {
			return nil
		}
		return []string{value}
	}
}

func boolValue(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		parsed, err := strconv.ParseBool(typed)
		return err == nil && parsed
	default:
		parsed, err := strconv.ParseBool(fmt.Sprintf("%v", typed))
		return err == nil && parsed
	}
}

func valueOrDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func titleCase(value string) string {
	if value == "" {
		return ""
	}
	parts := strings.FieldsFunc(value, func(r rune) bool {
		return r == '-' || r == '_' || r == ' '
	})
	for i := range parts {
		if parts[i] == "" {
			continue
		}
		parts[i] = strings.ToUpper(parts[i][:1]) + strings.ToLower(parts[i][1:])
	}
	return strings.Join(parts, " ")
}
