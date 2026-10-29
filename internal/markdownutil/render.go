package markdownutil

import (
	"fmt"
	"regexp"
	"strings"
)

var textEscapes = strings.NewReplacer(
	`\`, `\\`, "&", "&amp;", "<", `\<`, ">", `\>`,
	"`", "\\`", "*", `\*`, "_", `\_`, "[", `\[`, "]", `\]`,
	"#", `\#`, "~", `\~`,
)

var backtickRuns = regexp.MustCompile("`+")

func Text(value string) string {
	lines := strings.Split(textEscapes.Replace(value), "\n")
	for i, line := range lines {
		start := len(line) - len(strings.TrimLeft(line, " "))
		if start > 3 {
			continue
		}
		end := strings.IndexAny(line[start:], " \t")
		if end < 0 {
			continue
		}
		marker := line[start : start+end]
		if marker == "-" || marker == "+" {
			lines[i] = line[:start] + `\` + line[start:]
			continue
		}
		if len(marker) < 2 || !strings.ContainsAny(marker[len(marker)-1:], ".)") || strings.Trim(marker[:len(marker)-1], "0123456789") != "" {
			continue
		}
		position := start + len(marker) - 1
		lines[i] = line[:position] + `\` + line[position:]
	}
	return strings.Join(lines, "\n")
}

func Fence(content string) string {
	return strings.Repeat("`", max(3, longestBackticks(content)+1))
}

func CodeSpan(content string) string {
	fence := strings.Repeat("`", longestBackticks(content)+1)
	if strings.HasPrefix(content, "`") || strings.HasSuffix(content, "`") || (strings.HasPrefix(content, " ") && strings.HasSuffix(content, " ") && strings.TrimSpace(content) != "") {
		content = " " + content + " "
	}
	return fence + content + fence
}

func Destination(value string) string {
	var result strings.Builder
	for _, character := range value {
		if character <= ' ' || strings.ContainsRune("()<>\\", character) {
			fmt.Fprintf(&result, "%%%02X", character)
		} else {
			result.WriteRune(character)
		}
	}
	return result.String()
}

func longestBackticks(content string) int {
	longest := 0
	for _, run := range backtickRuns.FindAllString(content, -1) {
		longest = max(longest, len(run))
	}
	return longest
}
