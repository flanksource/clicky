package formatters

import (
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/flanksource/clicky/api"
)

// SortField represents a field to sort by with its priority
type SortField struct {
	Name     string
	Priority int
	Direction string // "asc" or "desc"
}

// ExtractSortFields extracts sort fields from struct tags
func ExtractSortFields(typ reflect.Type) []SortField {
	var sortFields []SortField
	
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		
		// Skip unexported fields
		if !field.IsExported() {
			continue
		}
		
		// Parse pretty tag
		prettyTag := field.Tag.Get("pretty")
		if prettyTag == "" || prettyTag == "-" || prettyTag == "hide" {
			continue
		}
		
		// Get field name
		fieldName := field.Name
		jsonTag := field.Tag.Get("json")
		if jsonTag != "" && jsonTag != "-" {
			if parts := strings.Split(jsonTag, ","); parts[0] != "" {
				fieldName = parts[0]
			}
		}
		
		// Look for sort=N in the pretty tag
		parts := strings.Split(prettyTag, ",")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if strings.HasPrefix(part, "sort=") {
				sortValue := strings.TrimPrefix(part, "sort=")
				if priority, err := strconv.Atoi(sortValue); err == nil {
					sortField := SortField{
						Name:      fieldName,
						Priority:  priority,
						Direction: "asc", // Default to ascending
					}
					
					// Check for direction
					for _, p := range parts {
						if strings.HasPrefix(p, "dir=") {
							sortField.Direction = strings.TrimPrefix(p, "dir=")
							break
						}
					}
					
					sortFields = append(sortFields, sortField)
					break
				}
			}
		}
	}
	
	// Sort by priority (lower number = higher priority)
	sort.Slice(sortFields, func(i, j int) bool {
		return sortFields[i].Priority < sortFields[j].Priority
	})
	
	return sortFields
}

// SortRows sorts rows based on multiple sort fields
func SortRows(rows []api.PrettyDataRow, sortFields []SortField) {
	if len(sortFields) == 0 {
		return
	}
	
	sort.Slice(rows, func(i, j int) bool {
		for _, field := range sortFields {
			valI := rows[i][field.Name]
			valJ := rows[j][field.Name]
			
			cmp := compareValues(valI, valJ)
			if cmp != 0 {
				if field.Direction == "desc" {
					return cmp > 0
				}
				return cmp < 0
			}
		}
		return false
	})
}

// compareValues compares two values and returns -1, 0, or 1
func compareValues(a, b interface{}) int {
	// Extract actual value from FieldValue if needed
	if fieldValA, ok := a.(api.FieldValue); ok {
		a = fieldValA.Value
	}
	if fieldValB, ok := b.(api.FieldValue); ok {
		b = fieldValB.Value
	}
	
	// Handle nil values
	if a == nil && b == nil {
		return 0
	}
	if a == nil {
		return -1
	}
	if b == nil {
		return 1
	}
	
	// Convert to strings for comparison
	strA := toString(a)
	strB := toString(b)
	
	if strA < strB {
		return -1
	} else if strA > strB {
		return 1
	}
	return 0
}

// toString converts a value to string for comparison
func toString(v interface{}) string {
	if v == nil {
		return ""
	}
	
	switch val := v.(type) {
	case string:
		return val
	case int, int8, int16, int32, int64:
		return padNumber(v)
	case uint, uint8, uint16, uint32, uint64:
		return padNumber(v)
	case float32, float64:
		return padNumber(v)
	case bool:
		if val {
			return "1"
		}
		return "0"
	default:
		// Use reflection for other types
		rv := reflect.ValueOf(v)
		if rv.Kind() == reflect.String {
			return rv.String()
		}
		// Default to string representation
		return strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(rv.String(), "\n", " "), "\t", " "))
	}
}

// padNumber pads numbers for proper string comparison
func padNumber(v interface{}) string {
	// Convert to string and pad with zeros for proper sorting
	switch val := v.(type) {
	case int:
		return padInt(int64(val))
	case int8:
		return padInt(int64(val))
	case int16:
		return padInt(int64(val))
	case int32:
		return padInt(int64(val))
	case int64:
		return padInt(val)
	case uint:
		return padUint(uint64(val))
	case uint8:
		return padUint(uint64(val))
	case uint16:
		return padUint(uint64(val))
	case uint32:
		return padUint(uint64(val))
	case uint64:
		return padUint(val)
	case float32:
		return padFloat(float64(val))
	case float64:
		return padFloat(val)
	default:
		return ""
	}
}

func padInt(n int64) string {
	if n < 0 {
		// For negative numbers, add a prefix that sorts before positive
		return "!" + strconv.FormatInt(n, 10)
	}
	// Pad positive numbers with zeros
	return "#" + strings.Repeat("0", 20-len(strconv.FormatInt(n, 10))) + strconv.FormatInt(n, 10)
}

func padUint(n uint64) string {
	// Pad with zeros
	return "#" + strings.Repeat("0", 20-len(strconv.FormatUint(n, 10))) + strconv.FormatUint(n, 10)
}

func padFloat(f float64) string {
	if f < 0 {
		return "!" + strconv.FormatFloat(f, 'f', -1, 64)
	}
	return "#" + strconv.FormatFloat(f, 'f', -1, 64)
}