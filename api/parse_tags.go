package api

import (
	"strconv"
	"strings"
)

// ParsePrettyTag converts a struct tag string into field configuration.
// Supports format options, styling, colors, and tree/table settings.
func ParsePrettyTag(tag string) PrettyField {
	return ParsePrettyTagWithName("", tag)
}

// ParsePrettyTagWithName creates field configuration from a struct tag,
// using the provided field name as the default label and identifier.
func ParsePrettyTagWithName(fieldName, tag string) PrettyField {
	field := PrettyField{
		Name:          fieldName,
		Label:         fieldName, // Default label to field name
		FormatOptions: make(map[string]string),
		ColorOptions:  make(map[string]string),
	}

	if tag == "" {
		return field
	}

	parts := strings.Split(tag, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)

		// Parse key=value pairs
		if strings.Contains(part, "=") {
			kv := strings.SplitN(part, "=", 2)
			key := strings.TrimSpace(kv[0])
			value := strings.TrimSpace(kv[1])

			switch key {
			case "label":
				field.Label = value
			case "sort":
				field.FormatOptions["sort"] = value
			case "dir", "direction":
				field.FormatOptions["dir"] = value
			case "format":
				field.Format = value
			case "digits":
				field.FormatOptions["digits"] = value
			case "style":
				field.Style = value
			case "label_style":
				field.LabelStyle = value
			case "header_style":
				field.TableOptions.HeaderStyle = value
			case "row_style":
				field.TableOptions.RowStyle = value
			case "title":
				field.TableOptions.Title = value
			case "indent":
				if field.TreeOptions == nil {
					field.TreeOptions = DefaultTreeOptions()
				}
				if size, err := strconv.Atoi(value); err == nil {
					field.TreeOptions.IndentSize = size
				}
			case "render":
				// Look up custom render function
				if fn, exists := RenderFuncRegistry[value]; exists {
					field.RenderFunc = fn
				}
			case "max_depth":
				if field.TreeOptions == nil {
					field.TreeOptions = DefaultTreeOptions()
				}
				if depth, err := strconv.Atoi(value); err == nil {
					field.TreeOptions.MaxDepth = depth
				}
			case ColorGreen, ColorRed, ColorBlue, "yellow", "cyan", "magenta":
				field.ColorOptions[key] = value
			default:
				field.FormatOptions[key] = value
			}
		} else {
			// Simple flags
			switch part {
			case "table":
				field.Format = FormatTable
			case "tree":
				field.Format = FormatTree
				if field.TreeOptions == nil {
					field.TreeOptions = DefaultTreeOptions()
				}
			case "struct":
				field.Format = "struct"
			case FormatHide:
				field.Format = FormatHide
			case SortAsc, SortDesc:
				field.FormatOptions["dir"] = part
			case "compact":
				field.CompactItems = true
			case "no_icons":
				if field.TreeOptions == nil {
					field.TreeOptions = DefaultTreeOptions()
				}
				field.TreeOptions.ShowIcons = false
			case "ascii":
				if field.TreeOptions == nil {
					field.TreeOptions = ASCIITreeOptions()
				} else {
					field.TreeOptions.UseUnicode = false
					field.TreeOptions.BranchPrefix = "+-- "
					field.TreeOptions.LastPrefix = "`-- "
					field.TreeOptions.IndentPrefix = "    "
					field.TreeOptions.ContinuePrefix = "|   "
				}
			default:
				field.FormatOptions[part] = "true"
			}
		}
	}

	return field
}
