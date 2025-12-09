package icons

import (
	"fmt"
	"strings"

	"github.com/flanksource/clicky/api/tailwind"
	"github.com/muesli/termenv"
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

// ANSI returns the Unicode representation with color styling applied
func (i Icon) ANSI() string {
	if i.Style == "" {
		return i.Unicode
	}

	style := tailwind.ParseStyle(i.Style)
	output := termenv.DefaultOutput()
	styled := output.String(i.Unicode)

	if style.Foreground != "" {
		styled = styled.Foreground(tailwind.ClassToFgColor(i.Style))
	}
	if style.Bold {
		styled = styled.Bold()
	}

	return styled.String()
}

// HTML returns an HTML representation using Iconify web components or Unicode fallback
func (i Icon) HTML() string {
	classes := "text-lg"
	if i.Style != "" {
		classes = classes + " " + i.Style
	}
	if i.Iconify != "" {
		return fmt.Sprintf(`<iconify-icon icon="%s" class="%s"></iconify-icon>`, i.Iconify, classes)
	}
	if i.Style != "" {
		return fmt.Sprintf(`<span class="%s">%s</span>`, i.Style, i.Unicode)
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
	AI                = Icon{Unicode: "✨", Iconify: "ion:sparkles", Style: "muted"}
	Archive           = Icon{Unicode: "🗜️", Iconify: "vscode-icons:file-type-zip", Style: "muted"}
	Argocd            = Icon{Unicode: "🚀", Iconify: "devicon:argocd", Style: "muted"}
	ArrowDoubleLeft   = Icon{Unicode: "⇐", Iconify: "material-symbols-light:keyboard-double-arrow-left", Style: "muted"}
	ArrowDoubleDown   = Icon{Unicode: "⇕", Iconify: "material-symbols-light:keyboard-double-arrow-down", Style: "muted"}
	ArrowDoubleRight  = Icon{Unicode: "⇒", Iconify: "material-symbols-light:keyboard-double-arrow-right", Style: "muted"}
	ArrowDoubleUpDown = Icon{Unicode: "⇕", Iconify: "ion:swap-vertical", Style: "muted"}
	ArrowDown         = Icon{Unicode: "↓", Iconify: "fluent:arrow-down-24-filled", Style: "muted"}
	ArrowDownLeft     = Icon{Unicode: "↙", Iconify: "fluent:arrow-down-left-24-filled", Style: "muted"}
	ArrowDownRight    = Icon{Unicode: "↘", Iconify: "fluent:arrow-down-right-24-filled", Style: "muted"}
	ArrowLeft         = Icon{Unicode: "←", Iconify: "fluent:arrow-left-24-filled", Style: "muted"}
	ArrowLeftRight    = Icon{Unicode: "⇄", Iconify: "ion:swap-horizontal", Style: "muted"}
	ArrowRight        = Icon{Unicode: "→", Iconify: "fluent:arrow-right-24-filled", Style: "muted"}
	ArrowUp           = Icon{Unicode: "↑", Iconify: "fluent:arrow-up-24-filled", Style: "muted"}
	ArrowUpDown       = Icon{Unicode: "⇵", Iconify: "fluent:arrow-sort-20-filled", Style: "muted"}
	ArrowUpLeft       = Icon{Unicode: "↖", Iconify: "fluent:arrow-up-left-24-filled", Style: "muted"}
	ArrowUpRight      = Icon{Unicode: "↗", Iconify: "fluent:arrow-up-right-24-filled", Style: "muted"}
	Audio             = Icon{Unicode: "🎵", Iconify: "vscode-icons:file-type-audio", Style: "muted"}
	Boolean           = Icon{Unicode: "⊨", Iconify: "ix:data-type-boolean", Style: "muted"}
	Check             = Icon{Unicode: "✓", Iconify: "ion:checkmark", Style: "text-green-500"}
	ChevronDown       = Icon{Unicode: "▼", Iconify: "ion:chevron-down", Style: "muted"}
	ChevronLeft       = Icon{Unicode: "◀", Iconify: "ion:chevron-back-circle", Style: "muted"}
	ChevronRight      = Icon{Unicode: "▶", Iconify: "ion:chevron-forward-circle", Style: "muted"}
	ChevronUp         = Icon{Unicode: "▲", Iconify: "ion:chevron-up", Style: "muted"}
	Chore             = Icon{Unicode: "🧹", Iconify: "ion:construct", Style: "muted"}
	CI                = Icon{Unicode: "🤖", Iconify: "ion:git-network", Style: "muted"}
	Circle            = Icon{Unicode: "○", Iconify: "ion:ellipse-outline", Style: "muted"}
	Clean             = Icon{Unicode: "🧹", Iconify: "mdi:broom", Style: "muted"}
	Cloud             = Icon{Unicode: "☁️", Iconify: "ion:cloud", Style: "muted"}
	Code              = Icon{Unicode: "💻", Iconify: "ion:code", Style: "muted"}
	Config            = Icon{Unicode: "⚙️", Iconify: "ion:settings", Style: "muted"}
	Config2           = Icon{Unicode: "🛠️", Iconify: "ion:settings", Style: "muted"}
	Constant          = Icon{Unicode: "π", Iconify: "ion:cube", Style: "muted"}
	Cost              = Icon{Unicode: "💲", Iconify: "ion:cash", Style: "muted"}
	Cross             = Icon{Unicode: "✗", Iconify: "ion:close", Style: "text-red-500"}
	CSS               = Icon{Unicode: "🎨", Iconify: "vscode-icons:file-type-css", Style: "muted"}
	CSV               = Icon{Unicode: "📑", Iconify: "fluent-mdl2:grid-view-small", Style: "muted"}
	Database          = Icon{Unicode: "🗄️", Iconify: "ion:server", Style: "muted"}
	DB                = Icon{Unicode: "🗄️", Iconify: "ion:server", Style: "muted"}
	Debug             = Icon{Unicode: "🐞", Iconify: "ion:bug", Style: "muted"}
	Dependency        = Package
	Docker            = Icon{Unicode: "🐳", Iconify: "vscode-icons:file-type-docker2", Style: "muted"}
	Docs              = Icon{Unicode: "📚", Iconify: "fluent:library-32-filled", Style: "muted"}
	Equals            = Icon{Unicode: "=", Iconify: "mdi:assignment", Style: "muted"}
	Error             = Cross
	Excel             = Icon{Unicode: "📊", Iconify: "vscode-icons:file-type-excel", Style: "muted"}
	Exclamation       = Icon{Unicode: "‼️", Iconify: "ion:warning", Style: "warning"}
	Executable        = Icon{Unicode: "⚙️", Iconify: "ion:settings", Style: "muted"}
	Fail              = Cross
	Feat              = Icon{Unicode: "✨", Iconify: "ion:sparkles", Style: "muted"}
	File              = Icon{Unicode: "📄", Iconify: "ion:document", Style: "muted"}
	Fix               = Icon{Unicode: "🐛", Iconify: "ion:bug", Style: "muted"}
	Folder            = Icon{Unicode: "📁", Iconify: "vscode-icons:default-folder", Style: "muted"}
	Git               = Icon{Unicode: "🔧", Iconify: "vscode-icons:file-type-git", Style: "muted"}
	Github            = Icon{Unicode: "🐙", Iconify: "devicon:github", Style: "muted"}
	Gitlab            = Icon{Unicode: "🦊", Iconify: "vscode-icons:file-type-gitlab", Style: "muted"}
	Golang            = Icon{Unicode: "🐹", Iconify: "vscode-icons:file-type-go-gopher", Style: "muted"}
	Heart             = Icon{Unicode: "❤️", Iconify: "ion:heart", Style: "muted"}
	HeavyArrow        = Icon{Unicode: "➜", Iconify: "ion:terminal", Style: "muted"}
	HTML              = Icon{Unicode: "🌐", Iconify: "vscode-icons:file-type-html", Style: "muted"}
	Http              = Icon{Unicode: "🌐", Iconify: "ion:globe", Style: "muted"}
	Idea              = Icon{Unicode: "💡", Iconify: "ion:bulb", Style: "info"}
	If                = QuestionRed
	Prometheus        = Icon{Unicode: "📡", Iconify: "devicon:prometheus", Style: "muted"}
	Terraform         = Icon{Unicode: "💻", Iconify: "devicon:terraform", Style: "muted"}
	Image             = Icon{Unicode: "🖼️", Iconify: "vscode-icons:file-type-image", Style: "muted"}
	Info              = Icon{Unicode: "•", Iconify: "ion:information-circle", Style: "info"}
	InfoAlt           = Icon{Unicode: "ℹ️", Iconify: "ion:information", Style: "info"}
	Infrastructure    = Icon{Unicode: "🏗️", Iconify: "ion:construct", Style: "muted"}
	Interface         = Icon{Unicode: "🔗", Iconify: "ion:git-branch", Style: "muted"}
	Java              = Icon{Unicode: "☕", Iconify: "vscode-icons:file-type-java", Style: "muted"}
	JS                = Icon{Unicode: "🟨", Iconify: "vscode-icons:file-type-js", Style: "muted"}
	JSON              = Icon{Unicode: "🗒️", Iconify: "vscode-icons:file-type-json", Style: "muted"}
	Key               = Icon{Unicode: "🔑", Iconify: "ion:key", Style: "muted"}
	Kubernetes        = Icon{Unicode: "☸️", Iconify: "devicon:kubernetes", Style: "muted"}
	Kustomize         = Icon{Unicode: "🧩", Iconify: "vscode-icons:file-type-kustomize", Style: "muted"}
	Lambda            = Icon{Unicode: "λ", Iconify: "ion:code-working", Style: "muted"}
	Launch            = Icon{Unicode: "🎉", Iconify: "ion:rocket", Style: "muted"}
	Link              = Icon{Unicode: "🔗", Iconify: "ion:link", Style: "muted"}
	Lock              = Icon{Unicode: "🔒", Iconify: "ion:lock-closed", Style: "muted"}
	Loop              = Icon{Unicode: "🔄", Iconify: "ion:refresh", Style: "muted"}
	LUA               = Icon{Unicode: "", Iconify: "vscode-icons:file-type-lua", Style: "muted"}
	Makefile          = Icon{Unicode: "🛠️", Iconify: "vscode-icons:file-type-makefile", Style: "muted"}
	Markdown          = MD
	Math              = Icon{Unicode: "🧮", Iconify: "ix:plus-minus-times-divide", Style: "muted"}
	MD                = Icon{Unicode: "📝", Iconify: "devicon:markdown", Style: "muted"}
	MDX               = Icon{Unicode: "📚", Iconify: "vscode-icons:file-type-mdx", Style: "muted"}
	Method            = Icon{Unicode: "ƒ", Iconify: "gravity-ui:curly-brackets-function", Style: "muted"}
	MinimalArrow      = Icon{Unicode: "❯", Iconify: "ion:terminal", Style: "muted"}
	Monitor           = Icon{Unicode: "🖥️", Iconify: "ion:desktop", Style: "muted"}
	Network           = Icon{Unicode: "🌐", Iconify: "ion:globe", Style: "muted"}
	Node              = Icon{Unicode: "🟨", Iconify: "vscode-icons:file-type-node", Style: "muted"}
	Not               = Icon{Unicode: "≠", Iconify: "mdi:not-equal", Style: "muted"}
	NPM               = Icon{Unicode: "📦", Iconify: "vscode-icons:file-type-npm", Style: "muted"}
	Number            = Icon{Unicode: "#", Iconify: "fluent:number-symbol-16-regular", Style: "muted"}
	Package           = Icon{Unicode: "📦", Iconify: "ion:cube", Style: "muted"}
	Pass              = Check
	Pause             = Icon{Unicode: "⏸", Iconify: "ion:pause", Style: "muted"}
	PDF               = Icon{Unicode: "📄", Iconify: "vscode-icons:file-type-pdf", Style: "muted"}
	Pending           = Icon{Unicode: "⏳", Iconify: "ion:hourglass-outline", Style: "muted"}
	Performance       = Icon{Unicode: "⚡", Iconify: "ion:speedometer", Style: "muted"}
	Play              = Icon{Unicode: "▶", Iconify: "ion:play-circle", Style: "muted"}
	Plugin            = Icon{Unicode: "🧩", Iconify: "ix:jigsaw-filled", Style: "muted"}
	PowerPoint        = Icon{Unicode: "📈", Iconify: "vscode-icons:file-type-ppt", Style: "muted"}
	Powershell        = Icon{Unicode: "💻", Iconify: "vscode-icons:file-type-powershell", Style: "muted"}
	Python            = Icon{Unicode: "🐍", Iconify: "vscode-icons:file-type-python", Style: "muted"}
	QuestionRed       = Icon{Unicode: "❓", Iconify: "ion:help-circle", Style: "error"}
	Queue             = Icon{Unicode: "📥", Iconify: "ion:file-tray-stacked", Style: "muted"}
	React             = Icon{Unicode: "⚛️", Iconify: "vscode-icons:file-type-reactjs", Style: "muted"}
	Redo              = Icon{Unicode: "↷", Iconify: "ion:arrow-redo", Style: "muted"}
	Refactor          = Icon{Unicode: "🔨", Iconify: "ion:hammer", Style: "muted"}
	Reliability       = Icon{Unicode: "🛡️", Iconify: "ion:shield", Style: "muted"}
	Reload            = Icon{Unicode: "🔄", Iconify: "ion:refresh", Style: "muted"}
	Robot             = Icon{Unicode: "🤖", Iconify: "ion:hardware-chip", Style: "muted"}
	Rocket            = Icon{Unicode: "🚀", Iconify: "ion:rocket", Style: "muted"}
	Ruby              = Icon{Unicode: "💎", Iconify: "vscode-icons:file-type-ruby", Style: "muted"}
	Scaling           = Icon{Unicode: "📈", Iconify: "ion:trending-up", Style: "muted"}
	Search            = Icon{Unicode: "🔍", Iconify: "ion:search", Style: "muted"}
	Shell             = Icon{Unicode: "💻", Iconify: "vscode-icons:file-type-shell", Style: "muted"}
	Skip              = Icon{Unicode: "→", Iconify: "ion:arrow-forward", Style: "warning"}
	SQL               = DB
	Star              = Icon{Unicode: "★", Iconify: "ion:star-outline", Style: "muted"}
	Start             = Play
	Stop              = Icon{Unicode: "■", Iconify: "ion:stop-circle", Style: "muted"}
	Style             = Icon{Unicode: "🎨", Iconify: "ion:color-palette", Style: "muted"}
	Success           = Check
	Sum               = Icon{Unicode: "∑", Iconify: "ion:calculator", Style: "muted"}
	Table             = Icon{Unicode: "📋", Iconify: "ion:grid", Style: "muted"}
	Target            = Icon{Unicode: "🎯", Iconify: "ion:navigate-circle", Style: "muted"}
	Terminal          = Icon{Unicode: "💻", Iconify: "ion:terminal", Style: "muted"}
	Test              = Icon{Unicode: "🧪", Iconify: "ion:flask", Style: "muted"}
	TS                = Icon{Unicode: "🟦", Iconify: "vscode-icons:file-type-typescript", Style: "muted"}
	Type              = Icon{Unicode: "🏷️", Iconify: "ion:pricetag", Style: "muted"}
	TypeScript        = Icon{Unicode: "🟦", Iconify: "vscode-icons:file-type-typescript", Style: "muted"}
	Undo              = Icon{Unicode: "↶", Iconify: "ion:arrow-undo", Style: "muted"}
	Unknown           = Icon{Unicode: "?", Iconify: "ion:help-circle", Style: "muted"}
	Variable          = Icon{Unicode: "𝑣", Iconify: "mdi:variable-box", Style: "font-bold"}
	Video             = Icon{Unicode: "🎬", Iconify: "vscode-icons:file-type-video", Style: "muted"}
	Warning           = Icon{Unicode: "⚠️", Iconify: "ion:warning", Style: "warning"}
	Wrench            = Icon{Unicode: "🔧", Iconify: "ion:build", Style: "muted"}
	XML               = Icon{Unicode: "📄", Iconify: "vscode-icons:file-type-xml", Style: "muted"}
	YAML              = Icon{Unicode: "📄", Iconify: "vscode-icons:file-type-yaml", Style: "muted"}
	Zombie            = Icon{Unicode: "💀", Iconify: "ion:skull", Style: "muted"}
	Add               = Icon{Unicode: "➕", Iconify: "ion:add-circle", Style: "text-green-500"}
	Delete            = Icon{Unicode: "➖", Iconify: "ion:remove-circle", Style: "text-red-500"}
	Edit              = Icon{Unicode: "✏️", Iconify: "ion:pencil", Style: "text-yellow-500"}
	Rename            = Icon{Unicode: "🔀", Iconify: "ion:swap-horizontal", Style: "text-blue-500"}
)

var All = map[string]Icon{
	"pdf":        PDF,
	"npm":        NPM,
	"node":       Node,
	"dockerfile": Docker,

	"powershell":          Powershell,
	"lua":                 LUA,
	"docs":                Docs,
	"mdx":                 MDX,
	"csv":                 CSV,
	"react":               React,
	"makefile":            Makefile,
	"typescript":          TypeScript,
	"yaml":                YAML,
	"html":                HTML,
	"css":                 CSS,
	"json":                JSON,
	"xml":                 XML,
	"docker":              Docker,
	"kustomize":           Kustomize,
	"excel":               Excel,
	"powerpoint":          PowerPoint,
	"executable":          Executable,
	"terminal":            Terminal,
	"archive":             Archive,
	"image":               Image,
	"video":               Video,
	"audio":               Audio,
	"ai":                  AI,
	"code":                Code,
	"arrow_double_left":   ArrowDoubleLeft,
	"arrow_double_right":  ArrowDoubleRight,
	"arrow_double_updown": ArrowDoubleUpDown,
	"arrow_down":          ArrowDown,
	"arrow_down_left":     ArrowDownLeft,
	"arrow_down_right":    ArrowDownRight,
	"arrow_left":          ArrowLeft,
	"arrow_left_right":    ArrowLeftRight,
	"arrow_right":         ArrowRight,
	"arrow_up":            ArrowUp,
	"arrow_up_down":       ArrowUpDown,
	"arrow_up_left":       ArrowUpLeft,
	"arrow_up_right":      ArrowUpRight,
	"boolean":             Boolean,
	"chevron_down":        ChevronDown,
	"ci":                  CI,
	"rocket":              Rocket,
	"dependency":          Dependency,
	"chevron_left":        ChevronLeft,
	"chevron_right":       ChevronRight,
	"chevron_up":          ChevronUp,
	"circle":              Circle,
	"clean":               Clean,
	"cloud":               Cloud,
	"config":              Config,
	"config2":             Config2,
	"constant":            Constant,
	"feat":                Feat,
	"chore":               Chore,
	"refactor":            Refactor,
	"fix":                 Fix,
	"sum":                 Sum,
	"style":               Style,
	"undo":                Undo,
	"redo":                Redo,
	"debug":               Debug,
	"warning":             Warning,
	"cost":                Cost,
	"database":            Database,
	"key":                 Key,
	"db":                  DB,
	"equals":              Equals,
	"cross":               Cross,
	"error":               Error,
	"fail":                Fail,
	"file":                File,
	"folder":              Folder,
	"golang":              Golang,
	"heart":               Heart,
	"heavy_arrow":         HeavyArrow,
	"http":                Http,
	"idea":                Idea,
	"if":                  If,
	"info":                Info,
	"info_alt":            InfoAlt,
	"interface":           Interface,
	"java":                Java,
	"js":                  JS,
	"kubernetes":          Kubernetes,
	"lambda":              Lambda,
	"launch":              Launch,
	"link":                Link,
	"markdown":            Markdown,
	"lock":                Lock,
	"loop":                Loop,
	"math":                Math,
	"monitor":             Monitor,
	"infrastructure":      Infrastructure,
	"scaling":             Scaling,
	"reliability":         Reliability,
	"network":             Network,
	"md":                  MD,
	"method":              Method,
	"minimal_arrow":       MinimalArrow,
	"not":                 Not,
	"number":              Number,
	"package":             Package,
	"pass":                Pass,
	"check":               Check,
	"pause":               Pause,
	"pending":             Pending,
	"performance":         Performance,
	"play":                Play,
	"stop":                Stop,
	"plugin":              Plugin,
	"python":              Python,
	"question_red":        QuestionRed,
	"queue":               Queue,
	"reload":              Reload,
	"robot":               Robot,
	"search":              Search,
	"skip":                Skip,
	"sql":                 SQL,
	"star":                Star,
	"start":               Start,
	"success":             Success,
	"table":               Table,
	"target":              Target,
	"test":                Test,
	"ts":                  TS,
	"type":                Type,
	"unknown":             Unknown,
	"variable":            Variable,
	"exclamation":         Exclamation,
	"wrench":              Wrench,
	"zombie":              Zombie,
}
