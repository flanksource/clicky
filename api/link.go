package api

import (
	"encoding/json"
	"fmt"
)

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
	// JSON is an optional structured payload carried alongside the link. Unlike
	// Text, a Link serializes to a structured object (see MarshalJSON), so this
	// payload survives JSON encoding and reaches structured-JSON consumers (e.g.
	// a web UI that renders an <a> from the href and uses the payload for
	// client-side navigation). It is ignored by String/ANSI/Markdown/HTML.
	JSON any
}

func NewLink(href string) Link {
	return Link{Href: href}
}

// WithJSON attaches a structured payload to the link. The payload is emitted by
// MarshalJSON under the "json" key; it does not affect text rendering.
func (l Link) WithJSON(v any) Link {
	l.JSON = v
	return l
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
	return l.MarkdownWithOptions(MarkdownOptions{})
}

func (l Link) MarkdownWithOptions(options MarkdownOptions) string {
	label := RenderMarkdown(l.Content, options)
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

// MarshalJSON serializes a Link as a structured object so its href and payload
// survive JSON encoding. This is deliberately richer than Text.MarshalJSON
// (which flattens to a plain string): a structured-JSON consumer can render the
// link as an anchor and use the payload for navigation. Only non-empty fields
// are emitted.
func (l Link) MarshalJSON() ([]byte, error) {
	out := map[string]any{"text": l.Content.String()}
	if l.Href != "" {
		out["href"] = l.Href
	}
	if l.Target != "" {
		out["target"] = string(l.Target)
	}
	if l.Content.Tooltip != nil {
		if tip := l.Content.Tooltip.String(); tip != "" {
			out["tooltip"] = tip
		}
	}
	if l.JSON != nil {
		out["json"] = l.JSON
	}
	return json.Marshal(out)
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
