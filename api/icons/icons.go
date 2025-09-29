package icons

import "fmt"

type Icon struct {
	Unicode string
	Iconify string
	Style   string
}

// String returns the Unicode representation of the icon
func (i Icon) String() string {
	return i.Unicode
}

// ANSI returns the Unicode representation (same as String for icons)
func (i Icon) ANSI() string {
	return i.Unicode
}

// HTML returns an HTML representation using Iconify classes or Unicode fallback
func (i Icon) HTML() string {
	if i.Iconify != "" {
		return fmt.Sprintf(`<i class="iconify" data-icon="%s">%s</i>`, i.Iconify, i.Unicode)
	}
	return i.Unicode
}

// Markdown returns the Unicode representation (same as String for icons)
func (i Icon) Markdown() string {
	return i.Unicode
}

var (
	Config                   = "⚙️"
	Success             Icon = Icon{Unicode: "✓", Iconify: "check", Style: "success"}
	Error                    = Icon{Unicode: "✗", Iconify: "close", Style: "error"}
	Fail                     = Icon{Unicode: "✗", Iconify: "close", Style: "error"}
	Pass                     = Icon{Unicode: "✓", Iconify: "check", Style: "success"}
	Skip                     = Icon{Unicode: "→", Iconify: "arrow-right", Style: "warning"}
	Unknown                  = Icon{Unicode: "?", Iconify: "help", Style: "muted"}
	Info                     = Icon{Unicode: "•", Iconify: "bullet", Style: "info"}
	Warning                  = Icon{Unicode: "!", Iconify: "alert-circle", Style: "warning"}
	Circle                   = Icon{Unicode: "○", Iconify: "circle", Style: "muted"}
	ArrowUp                  = Icon{Unicode: "↑", Iconify: "arrow-up", Style: "muted"}
	ArrowDown                = Icon{Unicode: "↓", Iconify: "arrow-down", Style: "muted"}
	ArrowLeft                = Icon{Unicode: "←", Iconify: "arrow-left", Style: "muted"}
	ArrowUpRight             = Icon{Unicode: "↗", Iconify: "arrow-up-right", Style: "muted"}
	ArrowDownRight           = Icon{Unicode: "↘", Iconify: "arrow-down-right", Style: "muted"}
	ArrowDownLeft            = Icon{Unicode: "↙", Iconify: "arrow-down-left", Style: "muted"}
	ArrowUpLeft              = Icon{Unicode: "↖", Iconify: "arrow-up-left", Style: "muted"}
	ArrowDoubleUpDown        = Icon{Unicode: "⇕", Iconify: "arrows-up-down", Style: "muted"}
	ArrowUpDown              = Icon{Unicode: "⇵", Iconify: "arrow-up-down", Style: "muted"}
	ArrowLeftRight           = Icon{Unicode: "⇄", Iconify: "arrows-left-right", Style: "muted"}
	ArrowoubleLeftRight      = Icon{Unicode: "⇔", Iconify: "arrows-left-right", Style: "muted"}
	ArrowDoubleRight         = Icon{Unicode: "⇒", Iconify: "arrow-right", Style: "muted"}
	ArrowDoubleLeft          = Icon{Unicode: "⇐", Iconify: "arrow-left", Style: "muted"}
	ArrowRight               = Icon{Unicode: "→", Iconify: "arrow-right", Style: "muted"}
	ChevronUp                = Icon{Unicode: "▲", Iconify: "chevron-up", Style: "muted"}
	ChevronDown              = Icon{Unicode: "▼", Iconify: "chevron-down", Style: "muted"}
	ChevronLeft              = Icon{Unicode: "◀", Iconify: "chevron-left", Style: "muted"}
	ChevronRight             = Icon{Unicode: "▶", Iconify: "chevron-right", Style: "muted"}
	InfoAlt                  = Icon{Unicode: "ℹ️", Iconify: "info", Style: "info"}
	Star                     = Icon{Unicode: "★", Iconify: "star", Style: "muted"}
	Heart                    = Icon{Unicode: "❤️", Iconify: "heart", Style: "muted"}
	Link                     = Icon{Unicode: "🔗", Iconify: "link", Style: "muted"}
	Golang                   = Icon{Unicode: "🐹", Iconify: "go", Style: "muted"}
	Python                   = Icon{Unicode: "🐍", Iconify: "python", Style: "muted"}
	JS                       = Icon{Unicode: "🟨", Iconify: "javascript", Style: "muted"}
	Java                     = Icon{Unicode: "☕", Iconify: "java", Style: "muted"}
	TS                       = Icon{Unicode: "🟦", Iconify: "typescript", Style: "muted"}
	MD                       = Icon{Unicode: "📝", Iconify: "markdown", Style: "muted"}
	File                     = Icon{Unicode: "📄", Iconify: "file", Style: "muted"}
	Folder                   = Icon{Unicode: "📁", Iconify: "folder", Style: "muted"}
	Search                   = Icon{Unicode: "🔍", Iconify: "search", Style: "muted"}
	Cloud                    = Icon{Unicode: "☁️", Iconify: "cloud", Style: "muted"}
	Package                  = Icon{Unicode: "📦", Iconify: "package", Style: "muted"}
	Lambda                   = Icon{Unicode: "λ", Iconify: "lambda", Style: "muted"}
	Method                   = Icon{Unicode: "ƒ", Iconify: "function", Style: "muted"}
	Variable                 = Icon{Unicode: "𝑣", Iconify: "variable", Style: "muted"}
	Type                     = Icon{Unicode: "🏷️", Iconify: "tag", Style: "muted"}
	Interface                = Icon{Unicode: "🔗", Iconify: "link", Style: "muted"}
	Constant                 = Icon{Unicode: "π", Iconify: "constant", Style: "muted"}
	Http                     = Icon{Unicode: "🌐", Iconify: "globe", Style: "muted"}
	Queue                    = Icon{Unicode: "📥", Iconify: "inbox", Style: "muted"}
	DB                       = Icon{Unicode: "🗄️", Iconify: "database", Style: "muted"}
)
