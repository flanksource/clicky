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
		return fmt.Sprintf(`<iconify-icon icon="%s" class="text-lg"></iconify-icon>`, i.Iconify)
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
	AI                  = Icon{Unicode: "✨", Iconify: "codicon:robot", Style: "muted"}
	ArrowDoubleLeft     = Icon{Unicode: "⇐", Iconify: "codicon:arrow-left", Style: "muted"}
	ArrowDoubleRight    = Icon{Unicode: "⇒", Iconify: "codicon:arrow-right", Style: "muted"}
	ArrowDoubleUpDown   = Icon{Unicode: "⇕", Iconify: "codicon:arrow-both", Style: "muted"}
	ArrowDown           = Icon{Unicode: "↓", Iconify: "codicon:arrow-down", Style: "muted"}
	ArrowDownLeft       = Icon{Unicode: "↙", Iconify: "codicon:arrow-down", Style: "muted"}
	ArrowDownRight      = Icon{Unicode: "↘", Iconify: "codicon:arrow-down", Style: "muted"}
	ArrowLeft           = Icon{Unicode: "←", Iconify: "codicon:arrow-left", Style: "muted"}
	ArrowLeftRight      = Icon{Unicode: "⇄", Iconify: "codicon:arrow-swap", Style: "muted"}
	ArrowoubleLeftRight = Icon{Unicode: "⇔", Iconify: "codicon:arrow-swap", Style: "muted"}
	ArrowRight          = Icon{Unicode: "→", Iconify: "codicon:arrow-right", Style: "muted"}
	ArrowUp             = Icon{Unicode: "↑", Iconify: "codicon:arrow-up", Style: "muted"}
	ArrowUpDown         = Icon{Unicode: "⇵", Iconify: "codicon:arrow-both", Style: "muted"}
	ArrowUpLeft         = Icon{Unicode: "↖", Iconify: "codicon:arrow-up", Style: "muted"}
	ArrowUpRight        = Icon{Unicode: "↗", Iconify: "codicon:arrow-up", Style: "muted"}
	Boolean             = Icon{Unicode: "⊨", Iconify: "ix:data-type-boolean", Style: "muted"}
	ChevronDown         = Icon{Unicode: "▼", Iconify: "codicon:chevron-down", Style: "muted"}
	CI                  = Icon{Unicode: "🤖", Iconify: "codicon:github-action", Style: "muted"}
	Rocket              = Icon{Unicode: "🚀", Iconify: "codicon:rocket", Style: "muted"}
	Dependency          = Package
	ChevronLeft         = Icon{Unicode: "◀", Iconify: "codicon:chevron-left", Style: "muted"}
	ChevronRight        = Icon{Unicode: "▶", Iconify: "codicon:chevron-right", Style: "muted"}
	ChevronUp           = Icon{Unicode: "▲", Iconify: "codicon:chevron-up", Style: "muted"}
	Circle              = Icon{Unicode: "○", Iconify: "codicon:circle", Style: "muted"}
	Clean               = Icon{Unicode: "🧹", Iconify: "mdi:broom", Style: "muted"}
	Cloud               = Icon{Unicode: "☁️", Iconify: "codicon:cloud", Style: "muted"}
	Config              = Icon{Unicode: "⚙️", Iconify: "codicon:gear", Style: "muted"}
	Config2             = Icon{Unicode: "🛠️", Iconify: "codicon:gear", Style: "muted"}
	Constant            = Icon{Unicode: "π", Iconify: "codicon:symbol-constant", Style: "muted"}
	Feat                = Icon{Unicode: "✨", Iconify: "codicon:symbol-event", Style: "muted"}
	Chore               = Icon{Unicode: "🧹", Iconify: "codicon:symbol-namespace", Style: "muted"}
	Refactor            = Icon{Unicode: "🔨", Iconify: "codicon:symbol-structure", Style: "muted"}
	Fix                 = Icon{Unicode: "🐛", Iconify: "codicon:symbol-property", Style: "muted"}
	Docs                = Icon{Unicode: "📝", Iconify: "codicon:symbol-string", Style: "muted"}
	Sum                 = Icon{Unicode: "∑", Iconify: "codicon:symbol-constant", Style: "muted"}
	Style               = Icon{Unicode: "🎨", Iconify: "codicon:symbol-color", Style: "muted"}
	Undo                = Icon{Unicode: "↶", Iconify: "codicon:arrow-left", Style: "muted"}
	Redo                = Icon{Unicode: "↷", Iconify: "codicon:arrow-right", Style: "muted"}
	Debug               = Icon{Unicode: "🐞", Iconify: "codicon:bug", Style: "muted"}
	Warning             = Icon{Unicode: "⚠️", Iconify: "codicon:warning", Style: "warning"}
	Cost                = Icon{Unicode: "💲", Iconify: "codicon:cash", Style: "muted"}
	Database            = Icon{Unicode: "🗄️", Iconify: "codicon:database", Style: "muted"}
	Key                 = Icon{Unicode: "🔑", Iconify: "codicon:key", Style: "muted"}
	DB                  = Icon{Unicode: "🗄️", Iconify: "codicon:database", Style: "muted"}
	Equals              = Icon{Unicode: "=", Iconify: "mdi:assignment", Style: "muted"}
	Cross               = Icon{Unicode: "✗", Iconify: "codicon:close", Style: "text-red-500"}
	Error               = Cross
	Fail                = Cross
	File                = Icon{Unicode: "📄", Iconify: "codicon:file", Style: "muted"}
	Folder              = Icon{Unicode: "📁", Iconify: "codicon:folder", Style: "muted"}
	Golang              = Icon{Unicode: "🐹", Iconify: "vscode-icons:file-type-go", Style: "muted"}
	Heart               = Icon{Unicode: "❤️", Iconify: "codicon:heart", Style: "muted"}
	HeavyArrow          = Icon{Unicode: "➜", Iconify: "codicon:terminal", Style: "muted"}
	Http                = Icon{Unicode: "🌐", Iconify: "codicon:globe", Style: "muted"}
	Idea                = Icon{Unicode: "💡", Iconify: "codicon:light-bulb", Style: "info"}
	If                  = QuestionRed
	Info                = Icon{Unicode: "•", Iconify: "codicon:info", Style: "info"}
	InfoAlt             = Icon{Unicode: "ℹ️", Iconify: "codicon:info", Style: "info"}
	Interface           = Icon{Unicode: "🔗", Iconify: "codicon:symbol-interface", Style: "muted"}
	Java                = Icon{Unicode: "☕", Iconify: "vscode-icons:file-type-java", Style: "muted"}
	JS                  = Icon{Unicode: "🟨", Iconify: "vscode-icons:file-type-js", Style: "muted"}
	Kubernetes          = Icon{Unicode: "☸️", Iconify: "vscode-icons:file-type-kubernetes", Style: "muted"}
	Lambda              = Icon{Unicode: "λ", Iconify: "codicon:symbol-method", Style: "muted"}
	Launch              = Icon{Unicode: "🎉", Iconify: "codicon:rocket", Style: "muted"}
	Link                = Icon{Unicode: "🔗", Iconify: "codicon:link", Style: "muted"}
	Lock                = Icon{Unicode: "🔒", Iconify: "codicon:lock", Style: "muted"}
	Loop                = Icon{Unicode: "🔄", Iconify: "codicon:sync", Style: "muted"}
	Math                = Icon{Unicode: "🧮", Iconify: "ix:plus-minus-times-divide", Style: "muted"}
	Monitor             = Icon{Unicode: "🖥️", Iconify: "codicon:monitor", Style: "muted"}
	Infrastructure      = Icon{Unicode: "🏗️", Iconify: "codicon:tools", Style: "muted"}
	Scaling             = Icon{Unicode: "📈", Iconify: "codicon:graph", Style: "muted"}
	Reliability         = Icon{Unicode: "🛡️", Iconify: "codicon:shield", Style: "muted"}
	Network             = Icon{Unicode: "🌐", Iconify: "codicon:globe", Style: "muted"}
	MD                  = Icon{Unicode: "📝", Iconify: "vscode-icons:file-type-markdown", Style: "muted"}
	Method              = Icon{Unicode: "ƒ", Iconify: "gravity-ui:curly-brackets-function", Style: "muted"}
	MinimalArrow        = Icon{Unicode: "❯", Iconify: "codicon:terminal", Style: "muted"}
	Not                 = Icon{Unicode: "≠", Iconify: "mdi:not-equal", Style: "muted"}
	Number              = Icon{Unicode: "#", Iconify: "fluent:number-symbol-16-regular", Style: "muted"}
	Package             = Icon{Unicode: "📦", Iconify: "codicon:package", Style: "muted"}
	Pass                = Check
	Check               = Icon{Unicode: "✓", Iconify: "codicon:pass", Style: "text-green-500"}
	Pause               = Icon{Unicode: "⏸", Iconify: "codicon:debug-pause", Style: "muted"}
	Pending             = Icon{Unicode: "⏳", Iconify: "codicon:hourglass", Style: "muted"}
	Performance         = Icon{Unicode: "⚡", Iconify: "codicon:rocket", Style: "muted"}
	Play                = Icon{Unicode: "▶", Iconify: "codicon:play", Style: "muted"}
	Plugin              = Icon{Unicode: "🧩", Iconify: "ix:jigsaw-filled", Style: "muted"}
	Python              = Icon{Unicode: "🐍", Iconify: "vscode-icons:file-type-python", Style: "muted"}
	QuestionRed         = Icon{Unicode: "❓", Iconify: "codicon:question", Style: "error"}
	Queue               = Icon{Unicode: "📥", Iconify: "codicon:inbox", Style: "muted"}
	Reload              = Icon{Unicode: "🔄", Iconify: "codicon:refresh", Style: "muted"}
	Robot               = Icon{Unicode: "🤖", Iconify: "codicon:robot", Style: "muted"}
	Search              = Icon{Unicode: "🔍", Iconify: "codicon:search", Style: "muted"}
	Skip                = Icon{Unicode: "→", Iconify: "codicon:arrow-right", Style: "warning"}
	SQL                 = DB
	Star                = Icon{Unicode: "★", Iconify: "codicon:star-empty", Style: "muted"}
	Start               = Play
	Stop                = Icon{Unicode: "🛑", Iconify: "codicon:stop", Style: "muted"}
	Success             = Check
	Table               = Icon{Unicode: "📋", Iconify: "codicon:table", Style: "muted"}
	Target              = Icon{Unicode: "🎯", Iconify: "codicon:target", Style: "muted"}
	Test                = Icon{Unicode: "🧪", Iconify: "codicon:beaker", Style: "muted"}
	TS                  = Icon{Unicode: "🟦", Iconify: "vscode-icons:file-type-typescript", Style: "muted"}
	Type                = Icon{Unicode: "🏷️", Iconify: "codicon:symbol-class", Style: "muted"}
	Unknown             = Icon{Unicode: "?", Iconify: "codicon:question", Style: "muted"}
	Variable            = Icon{Unicode: "𝑣", Iconify: "mdi:variable-box", Style: "font-bold"}

	Exclamation = Icon{Unicode: "‼️", Iconify: "codicon:warning", Style: "warning"}
	Wrench      = Icon{Unicode: "🔧", Iconify: "codicon:wrench", Style: "muted"}
	XML         = Icon{Unicode: "XML", Iconify: "carbon:xml", Style: "muted"}
	Zombie      = Icon{Unicode: "💀", Iconify: "codicon:skull", Style: "muted"}
)
