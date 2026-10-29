package api

import (
	"fmt"
	"strings"

	"github.com/flanksource/clicky/internal/markdownutil"
)

type Avatar struct {
	Name string
	URL  string
}

func (a Avatar) String() string { return a.Name }
func (a Avatar) ANSI() string   { return a.Name }

func (a Avatar) HTML() string {
	if a.URL == "" {
		return htmlEscapeString(a.Name)
	}
	return fmt.Sprintf(`<span class="inline-flex items-center gap-1" style="display: inline-flex; align-items: center; gap: 0.25rem; vertical-align: middle"><img src="%s" alt="" width="24" height="24" style="width: 24px; height: 24px; max-width: 24px; max-height: 24px; flex-shrink: 0; border-radius: 50%%; object-fit: cover">%s</span>`, htmlEscapeString(a.URL), htmlEscapeString(a.Name))
}

func (a Avatar) Markdown() string {
	return a.MarkdownWithOptions(MarkdownOptions{})
}

func (a Avatar) MarkdownWithOptions(options MarkdownOptions) string {
	label := markdownutil.Text(a.Name)
	if a.URL == "" || options.Dialect == DialectSlack {
		return label
	}
	if options.Dialect == DialectGFM {
		return a.HTML()
	}
	name := strings.NewReplacer("{", "&#123;", "}", "&#125;").Replace(htmlEscapeString(a.Name))
	return fmt.Sprintf(`<span className="inline-flex items-center gap-1" style={{display: "inline-flex", alignItems: "center", gap: "0.25rem", verticalAlign: "middle"}}><img src="%s" alt="" width={24} height={24} style={{width: "24px", height: "24px", maxWidth: "24px", maxHeight: "24px", flexShrink: 0, borderRadius: "50%%", objectFit: "cover"}} />%s</span>`, htmlEscapeString(a.URL), name)
}

func (a Avatar) MarkdownSlack() string {
	return a.MarkdownWithOptions(MarkdownOptions{Dialect: DialectSlack})
}
