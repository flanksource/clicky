package icons

import (
	"fmt"
	"strings"
)

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

// HTML returns an HTML representation using Iconify web components or Unicode fallback
func (i Icon) HTML() string {
	if i.Iconify != "" {
		return fmt.Sprintf(`<iconify-icon icon="%s"></iconify-icon>`, i.Iconify)
	}
	return i.Unicode
}

// Markdown returns the Unicode representation (same as String for icons)
func (i Icon) Markdown() string {
	return i.Unicode
}

func (i Icon) WithStyle(classes ...string) Icon {
	i.Style = strings.Join(classes, " ")
	return i
}

var (
	Config              = Icon{Unicode: "⚙️", Iconify: "codicon:gear", Style: "muted"}
	Equals              = Icon{Unicode: "=", Iconify: "mdi:assignment", Style: "muted"}
	Loop                = Icon{Unicode: "🔄", Iconify: "codicon:sync", Style: "muted"}
	If                  = QuestionRed
	SQL                 = Icon{Unicode: "🛢️", Iconify: "codicon:database", Style: "muted"}
	Success             = Icon{Unicode: "✓", Iconify: "codicon:check", Style: "success"}
	Error               = Icon{Unicode: "✗", Iconify: "codicon:error", Style: "error"}
	Fail                = Icon{Unicode: "✗", Iconify: "codicon:close", Style: "error"}
	Pass                = Icon{Unicode: "✓", Iconify: "codicon:pass", Style: "success"}
	Skip                = Icon{Unicode: "→", Iconify: "codicon:arrow-right", Style: "warning"}
	Unknown             = Icon{Unicode: "?", Iconify: "codicon:question", Style: "muted"}
	Info                = Icon{Unicode: "•", Iconify: "codicon:info", Style: "info"}
	Warning             = Icon{Unicode: "!", Iconify: "codicon:warning", Style: "warning"}
	Circle              = Icon{Unicode: "○", Iconify: "codicon:circle", Style: "muted"}
	ArrowUp             = Icon{Unicode: "↑", Iconify: "codicon:arrow-up", Style: "muted"}
	ArrowDown           = Icon{Unicode: "↓", Iconify: "codicon:arrow-down", Style: "muted"}
	ArrowLeft           = Icon{Unicode: "←", Iconify: "codicon:arrow-left", Style: "muted"}
	ArrowUpRight        = Icon{Unicode: "↗", Iconify: "codicon:arrow-up", Style: "muted"}
	ArrowDownRight      = Icon{Unicode: "↘", Iconify: "codicon:arrow-down", Style: "muted"}
	ArrowDownLeft       = Icon{Unicode: "↙", Iconify: "codicon:arrow-down", Style: "muted"}
	ArrowUpLeft         = Icon{Unicode: "↖", Iconify: "codicon:arrow-up", Style: "muted"}
	ArrowDoubleUpDown   = Icon{Unicode: "⇕", Iconify: "codicon:arrow-both", Style: "muted"}
	ArrowUpDown         = Icon{Unicode: "⇵", Iconify: "codicon:arrow-both", Style: "muted"}
	ArrowLeftRight      = Icon{Unicode: "⇄", Iconify: "codicon:arrow-swap", Style: "muted"}
	ArrowoubleLeftRight = Icon{Unicode: "⇔", Iconify: "codicon:arrow-swap", Style: "muted"}
	ArrowDoubleRight    = Icon{Unicode: "⇒", Iconify: "codicon:arrow-right", Style: "muted"}
	ArrowDoubleLeft     = Icon{Unicode: "⇐", Iconify: "codicon:arrow-left", Style: "muted"}
	ArrowRight          = Icon{Unicode: "→", Iconify: "codicon:arrow-right", Style: "muted"}
	ChevronUp           = Icon{Unicode: "▲", Iconify: "codicon:chevron-up", Style: "muted"}
	ChevronDown         = Icon{Unicode: "▼", Iconify: "codicon:chevron-down", Style: "muted"}
	Number              = Icon{Unicode: "#", Iconify: "fluent:number-symbol-16-regular", Style: "muted"}
	ChevronLeft         = Icon{Unicode: "◀", Iconify: "codicon:chevron-left", Style: "muted"}
	ChevronRight        = Icon{Unicode: "▶", Iconify: "codicon:chevron-right", Style: "muted"}
	InfoAlt             = Icon{Unicode: "ℹ️", Iconify: "codicon:info", Style: "info"}
	Star                = Icon{Unicode: "★", Iconify: "codicon:star-empty", Style: "muted"}
	Heart               = Icon{Unicode: "❤️", Iconify: "codicon:heart", Style: "muted"}
	Link                = Icon{Unicode: "🔗", Iconify: "codicon:link", Style: "muted"}
	Golang              = Icon{Unicode: "🐹", Iconify: "vscode-icons:file-type-go", Style: "muted"}
	Python              = Icon{Unicode: "🐍", Iconify: "vscode-icons:file-type-python", Style: "muted"}
	JS                  = Icon{Unicode: "🟨", Iconify: "vscode-icons:file-type-js", Style: "muted"}
	Math                = Icon{Unicode: "🧮", Iconify: "ix:plus-minus-times-divide", Style: "muted"}
	Boolean             = Icon{Unicode: "⊨", Iconify: "ix:data-type-boolean", Style: "muted"}
	Java                = Icon{Unicode: "☕", Iconify: "vscode-icons:file-type-java", Style: "muted"}
	XML                 = Icon{Unicode: "XML", Iconify: "carbon:xml", Style: "muted"}
	TS                  = Icon{Unicode: "🟦", Iconify: "vscode-icons:file-type-typescript", Style: "muted"}
	MD                  = Icon{Unicode: "📝", Iconify: "vscode-icons:file-type-markdown", Style: "muted"}
	File                = Icon{Unicode: "📄", Iconify: "codicon:file", Style: "muted"}
	Not                 = Icon{Unicode: "≠", Iconify: "mdi:not-equal", Style: "muted"}
	Folder              = Icon{Unicode: "📁", Iconify: "codicon:folder", Style: "muted"}
	Search              = Icon{Unicode: "🔍", Iconify: "codicon:search", Style: "muted"}
	Cloud               = Icon{Unicode: "☁️", Iconify: "codicon:cloud", Style: "muted"}
	Package             = Icon{Unicode: "📦", Iconify: "codicon:package", Style: "muted"}
	Lambda              = Icon{Unicode: "λ", Iconify: "codicon:symbol-method", Style: "muted"}
	Method              = Icon{Unicode: "ƒ", Iconify: "gravity-ui:curly-brackets-function", Style: "muted"}
	Variable            = Icon{Unicode: "𝑣", Iconify: "mdi:variable-box", Style: "muted"}
	Type                = Icon{Unicode: "🏷️", Iconify: "codicon:symbol-class", Style: "muted"}
	Interface           = Icon{Unicode: "🔗", Iconify: "codicon:symbol-interface", Style: "muted"}
	Table               = Icon{Unicode: "📋", Iconify: "codicon:table", Style: "muted"}
	Constant            = Icon{Unicode: "π", Iconify: "codicon:symbol-constant", Style: "muted"}
	Http                = Icon{Unicode: "🌐", Iconify: "codicon:globe", Style: "muted"}
	Queue               = Icon{Unicode: "📥", Iconify: "codicon:inbox", Style: "muted"}
	DB                  = Icon{Unicode: "🗄️", Iconify: "codicon:database", Style: "muted"}
	Zombie              = Icon{Unicode: "💀", Iconify: "codicon:skull", Style: "muted"}
	Target              = Icon{Unicode: "🎯", Iconify: "codicon:target", Style: "muted"}
	Plugin              = Icon{Unicode: "🧩", Iconify: "ix:jigsaw-filled", Style: "muted"}
	QuestionRed         = Icon{Unicode: "❓", Iconify: "codicon:question", Style: "error"}
	Idea                = Icon{Unicode: "💡", Iconify: "codicon:light-bulb", Style: "info"}
	Wrench              = Icon{Unicode: "🔧", Iconify: "codicon:wrench", Style: "muted"}
	Config2             = Icon{Unicode: "🛠️", Iconify: "codicon:gear", Style: "muted"}
	Clean               = Icon{Unicode: "🧹", Iconify: "mdi:broom", Style: "muted"}
	Launch              = Icon{Unicode: "🎉", Iconify: "codicon:rocket", Style: "muted"}
	Stop                = Icon{Unicode: "🛑", Iconify: "codicon:stop", Style: "muted"}
	Test                = Icon{Unicode: "🧪", Iconify: "codicon:beaker", Style: "muted"}
	AI                  = Icon{Unicode: "✨", Iconify: "codicon:robot", Style: "muted"}
	Robot               = Icon{Unicode: "🤖", Iconify: "codicon:robot", Style: "muted"}
	Start               = Play
	Play                = Icon{Unicode: "▶️", Iconify: "codicon:play", Style: "muted"}
	Pause               = Icon{Unicode: "⏸️", Iconify: "codicon:debug-pause", Style: "muted"}
	Reload              = Icon{Unicode: "🔄", Iconify: "codicon:refresh", Style: "muted"}
)
