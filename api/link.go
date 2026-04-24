package api

import "fmt"

type LinkTarget string

const (
	LinkTargetDialog LinkTarget = "Dialog"
	LinkTargetHover  LinkTarget = "Hover"
	LinkTargetExpand LinkTarget = "Expand"
	LinkTargetClicky LinkTarget = "_clicky"
	LinkTargetSelf   LinkTarget = "_self"
	LinkTargetWindow LinkTarget = "_window"
	LinkTargetTab    LinkTarget = "_tab"
)

type Link struct {
	Href    string
	Target  LinkTarget
	Content Text
}

func NewLink(href string) Link {
	return Link{Href: href}
}

func (l Link) Add(child Textable) Link {
	l.Content = l.Content.Add(child)
	return l
}

func (l Link) AddText(content string, styles ...string) Link {
	l.Content = l.Content.AddText(content, styles...)
	return l
}

func (l Link) Append(text any, styles ...string) Link {
	l.Content = l.Content.Append(text, styles...)
	return l
}

func (l Link) Appendf(format string, args ...interface{}) Link {
	l.Content = l.Content.Appendf(format, args...)
	return l
}

func (l Link) AppendStyle(classes ...string) Link {
	l.Content = l.Content.AppendStyle(classes...)
	return l
}

func (l Link) Prefix(prefix string) Link {
	l.Content = l.Content.Prefix(prefix)
	return l
}

func (l Link) Space() Link {
	l.Content = l.Content.Space()
	return l
}

func (l Link) Styles(classes ...string) Link {
	l.Content = l.Content.Styles(classes...)
	return l
}

func (l Link) Suffix(suffix string) Link {
	l.Content = l.Content.Suffix(suffix)
	return l
}

func (l Link) Tab() Link {
	l.Content = l.Content.Tab()
	return l
}

func (l Link) Text(content string, styles ...string) Link {
	l.Content = l.Content.Text(content, styles...)
	return l
}

func (l Link) WithStyles(styles ...string) Link {
	l.Content = l.Content.WithStyles(styles...)
	return l
}

func (l Link) WithTarget(target LinkTarget) Link {
	l.Target = target
	return l
}

func (l Link) WithTooltip(tooltip Textable) Link {
	l.Content = l.Content.WithTooltip(tooltip)
	return l
}

func (l Link) String() string {
	return l.Content.String()
}

func (l Link) ANSI() string {
	return l.Content.ANSI()
}

func (l Link) Markdown() string {
	label := l.Content.Markdown()
	if l.Href == "" {
		return label
	}
	return fmt.Sprintf("[%s](%s)", label, l.Href)
}

func (l Link) HTML() string {
	content := l.Content.HTML()
	if l.Href == "" {
		return content
	}
	target, rel := htmlLinkTargetAttributes(l.Target)
	return fmt.Sprintf(`<a href="%s"%s%s>%s</a>`, htmlEscapeString(l.Href), target, rel, content)
}

type LinkCommand struct {
	Command string
	Args    []string
	Flags   map[string]string
	Target  LinkTarget
	AutoRun bool
	Content Text
}

func NewLinkCommand(command string) LinkCommand {
	return LinkCommand{Command: command, Target: LinkTargetSelf}
}

func (l LinkCommand) Add(child Textable) LinkCommand {
	l.Content = l.Content.Add(child)
	return l
}

func (l LinkCommand) AddText(content string, styles ...string) LinkCommand {
	l.Content = l.Content.AddText(content, styles...)
	return l
}

func (l LinkCommand) Append(text any, styles ...string) LinkCommand {
	l.Content = l.Content.Append(text, styles...)
	return l
}

func (l LinkCommand) Appendf(format string, args ...interface{}) LinkCommand {
	l.Content = l.Content.Appendf(format, args...)
	return l
}

func (l LinkCommand) AppendStyle(classes ...string) LinkCommand {
	l.Content = l.Content.AppendStyle(classes...)
	return l
}

func (l LinkCommand) Prefix(prefix string) LinkCommand {
	l.Content = l.Content.Prefix(prefix)
	return l
}

func (l LinkCommand) Space() LinkCommand {
	l.Content = l.Content.Space()
	return l
}

func (l LinkCommand) Styles(classes ...string) LinkCommand {
	l.Content = l.Content.Styles(classes...)
	return l
}

func (l LinkCommand) Suffix(suffix string) LinkCommand {
	l.Content = l.Content.Suffix(suffix)
	return l
}

func (l LinkCommand) Tab() LinkCommand {
	l.Content = l.Content.Tab()
	return l
}

func (l LinkCommand) Text(content string, styles ...string) LinkCommand {
	l.Content = l.Content.Text(content, styles...)
	return l
}

func (l LinkCommand) WithArgs(args ...string) LinkCommand {
	l.Args = append([]string(nil), args...)
	return l
}

func (l LinkCommand) WithAutoRun(autoRun bool) LinkCommand {
	l.AutoRun = autoRun
	return l
}

func (l LinkCommand) WithFlag(name, value string) LinkCommand {
	if l.Flags == nil {
		l.Flags = map[string]string{}
	}
	l.Flags[name] = value
	return l
}

func (l LinkCommand) WithFlags(flags map[string]string) LinkCommand {
	if len(flags) == 0 {
		l.Flags = nil
		return l
	}

	l.Flags = make(map[string]string, len(flags))
	for key, value := range flags {
		l.Flags[key] = value
	}
	return l
}

func (l LinkCommand) WithStyles(styles ...string) LinkCommand {
	l.Content = l.Content.WithStyles(styles...)
	return l
}

func (l LinkCommand) WithTarget(target LinkTarget) LinkCommand {
	l.Target = target
	return l
}

func (l LinkCommand) WithTooltip(tooltip Textable) LinkCommand {
	l.Content = l.Content.WithTooltip(tooltip)
	return l
}

func (l LinkCommand) String() string {
	return l.Content.String()
}

func (l LinkCommand) ANSI() string {
	return l.Content.ANSI()
}

func (l LinkCommand) Markdown() string {
	return l.Content.Markdown()
}

func (l LinkCommand) HTML() string {
	return l.Content.HTML()
}

func htmlLinkTargetAttributes(target LinkTarget) (string, string) {
	switch target {
	case LinkTargetSelf:
		return ` target="_self"`, ""
	case LinkTargetWindow, LinkTargetTab:
		return ` target="_blank"`, ` rel="noopener noreferrer"`
	default:
		return "", ""
	}
}
