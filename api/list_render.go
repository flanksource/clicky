package api

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

func (l List) inline(parts []string) bool {
	return l.MaxInline > 0 && len(parts) <= l.MaxInline && !l.Ordered && !l.Numbered && !strings.Contains(strings.Join(parts, ""), "\n")
}

func (l List) renderList(render TextTransformer, customBullet string) string {
	parts := make([]string, 0, len(l.Items))
	for _, item := range l.Items {
		parts = append(parts, render(item))
	}
	if l.inline(parts) {
		return strings.Join(parts, ", ")
	}
	indent := strings.Repeat(" ", max(0, l.Indent))
	for i, part := range parts {
		bullet := ""
		if !l.Unstyled {
			bullet = "-"
			if l.Numbered || l.Ordered {
				bullet = fmt.Sprintf("%d.", i+1)
			} else if l.Bullet != nil {
				bullet = strings.TrimRight(customBullet, " \t")
			}
			bullet += strings.Repeat(" ", max(1, l.Spacing))
		}
		prefix := indent + bullet
		continuation := strings.Repeat(" ", ansi.StringWidth(prefix))
		parts[i] = prefix + strings.ReplaceAll(part, "\n", "\n"+continuation)
	}
	return strings.Join(parts, "\n")
}
