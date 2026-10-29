package flags

import (
	"fmt"
	"strconv"
	"strings"
)

func parseSpreadsheetColumn(selector string) (SpreadsheetReadOptions, string, error) {
	var options SpreadsheetReadOptions
	selector = strings.TrimSpace(selector)
	sheet, column, selected := strings.Cut(selector, "!")
	quoted := strings.HasPrefix(selector, "'")
	if quoted {
		var name strings.Builder
		end := -1
		for i := 1; i < len(selector); i++ {
			if selector[i] != '\'' {
				name.WriteByte(selector[i])
				continue
			}
			if i+1 < len(selector) && selector[i+1] == '\'' {
				name.WriteByte('\'')
				i++
				continue
			}
			end = i
			break
		}
		if end < 0 {
			return options, "", fmt.Errorf("unterminated quoted worksheet name in %q", selector)
		}
		column, selected = strings.CutPrefix(selector[end+1:], "!")
		if !selected {
			return options, "", fmt.Errorf("expected ! after quoted worksheet name in %q", selector)
		}
		sheet = name.String()
	}
	if !selected {
		return options, selector, nil
	}
	if strings.TrimSpace(sheet) == "" {
		return options, "", fmt.Errorf("worksheet name is empty in %q", selector)
	}
	if strings.TrimSpace(column) == "" {
		return options, "", fmt.Errorf("column name is empty in %q", selector)
	}
	if !quoted && strings.HasPrefix(sheet, "#") {
		index, err := strconv.Atoi(strings.TrimPrefix(sheet, "#"))
		if err != nil || index < 1 {
			return options, "", fmt.Errorf("worksheet index must be a positive integer in %q", selector)
		}
		options.Index = index
	} else {
		options.Sheet = sheet
	}
	return options, column, nil
}
