package entitydemo

import (
	"path"
	"strings"

	"github.com/flanksource/clicky"
	"github.com/flanksource/clicky/api"
	"github.com/flanksource/clicky/markdown"
)

func normalizeMarkdownPreviewFormat(format string) string {
	switch strings.ToLower(format) {
	case "react":
		return "clicky-json"
	case "md":
		return "markdown"
	case "xlsx":
		return "excel"
	default:
		return strings.ToLower(format)
	}
}

func markdownPreviewContentType(format string) string {
	if format == "clicky-json" {
		return "application/json+clicky"
	}
	return clicky.FormatToContentType(format)
}

type markdownPreviewRow struct {
	Index int    `json:"index" pretty:"label=Index"`
	Kind  string `json:"kind" pretty:"label=Kind"`
	Level int    `json:"level,omitempty" pretty:"label=Level"`
	Text  string `json:"text" pretty:"label=Text"`
}

func markdownPreviewRows(doc *markdown.Document) []markdownPreviewRow {
	var rows []markdownPreviewRow
	var visit func(node markdown.Node)
	visit = func(node markdown.Node) {
		if node.Kind != "" && node.Kind != "document" {
			rows = append(rows, markdownPreviewRow{
				Index: len(rows) + 1,
				Kind:  node.Kind,
				Level: node.Level,
				Text:  strings.TrimSpace(node.String()),
			})
		}
		for _, child := range node.Children {
			visit(child)
		}
		for _, item := range node.Items {
			visit(item)
		}
	}
	if doc != nil {
		visit(doc.Root)
	}
	return rows
}

func linkExamplesDocument() api.DescriptionList {
	return api.DescriptionList{
		Items: []api.KeyValuePair{
			{
				Key: "Plain link targets",
				Value: api.DescriptionList{
					Items: []api.KeyValuePair{
						{
							Key: "default",
							Value: linkExampleValue(
								clicky.Link("/stacks").Append("Open the stacks surface", "text-sky-700 underline underline-offset-4"),
								"Uses a normal in-app anchor without forcing a specific browser target.",
							),
						},
						{
							Key: "_self",
							Value: linkExampleValue(
								clicky.Link("/clusters").WithTarget(clicky.LinkTargetSelf).
									Append("Navigate to clusters in this tab", "text-sky-700 underline underline-offset-4"),
								"Sets target=_self explicitly for same-tab navigation.",
							),
						},
						{
							Key: "_window",
							Value: linkExampleValue(
								clicky.Link("/explorer").WithTarget(clicky.LinkTargetWindow).
									Append("Open the API explorer in a new window", "text-sky-700 underline underline-offset-4"),
								"Renders as a browser new-context link using the _window target hint.",
							),
						},
						{
							Key: "_tab",
							Value: linkExampleValue(
								clicky.Link("/admin-stacks").WithTarget(clicky.LinkTargetTab).
									Append("Open admin stacks in a new tab", "text-sky-700 underline underline-offset-4"),
								"Uses the same browser new-context flow but advertises the _tab target intent.",
							),
						},
					},
				},
			},
			{
				Key: "LinkCommand targets",
				Value: api.DescriptionList{
					Items: []api.KeyValuePair{
						{
							Key: "Dialog auto-run",
							Value: linkExampleValue(
								clicky.LinkCommand("stack/get").
									WithTarget(clicky.LinkTargetDialog).
									WithArgs("stk-001").
									WithFlag("events", "4").
									WithAutoRun(true).
									Append("Open a stack detail dialog", "text-cyan-700 underline underline-offset-4"),
								"Prefills id + events and runs immediately because every required parameter is already satisfied.",
							),
						},
						{
							Key: "Dialog waits for params",
							Value: linkExampleValue(
								clicky.LinkCommand("stack/get").
									WithTarget(clicky.LinkTargetDialog).
									WithAutoRun(true).
									Append("Show the form before running", "text-cyan-700 underline underline-offset-4"),
								"Leaves required params empty so the dialog opens prefilled but waits for a manual run.",
							),
						},
						{
							Key: "Hover",
							Value: linkExampleValue(
								clicky.LinkCommand("stack/get").
									WithTarget(clicky.LinkTargetHover).
									WithArgs("stk-002").
									WithFlag("events", "2").
									Append("Hover stack detail", "text-cyan-700 underline underline-offset-4"),
								"Resolves and executes lazily inside a hover preview.",
							),
						},
						{
							Key: "Expand",
							Value: linkExampleValue(
								clicky.LinkCommand("stack/get").
									WithTarget(clicky.LinkTargetExpand).
									WithArgs("stk-001").
									WithFlag("events", "1").
									Append("Expand stack detail", "text-cyan-700 underline underline-offset-4"),
								"Loads inline beneath the trigger without leaving the page.",
							),
						},
						{
							Key: "_clicky",
							Value: linkExampleValue(
								clicky.LinkCommand("stack/get").
									WithTarget(clicky.LinkTargetClicky).
									WithArgs("stk-001").
									WithFlag("events", "3").
									WithAutoRun(true).
									Append("Navigate inside Clicky", "text-cyan-700 underline underline-offset-4"),
								"Delegates navigation to the React host via commandRuntime.onNavigate.",
							),
						},
						{
							Key: "_self",
							Value: linkExampleValue(
								clicky.LinkCommand("stack/get").
									WithTarget(clicky.LinkTargetSelf).
									WithArgs("stk-002").
									WithFlag("events", "2").
									WithAutoRun(true).
									Append("Navigate in this tab", "text-cyan-700 underline underline-offset-4"),
								"Builds a deep-link URL that lands on a prefilled command page and auto-runs there.",
							),
						},
						{
							Key: "_window",
							Value: linkExampleValue(
								clicky.LinkCommand("stack/get").
									WithTarget(clicky.LinkTargetWindow).
									WithArgs("stk-001").
									WithFlag("events", "5").
									WithAutoRun(true).
									Append("Open in new window", "text-cyan-700 underline underline-offset-4"),
								"Uses the same deep-link URL builder but asks the browser for a new window context.",
							),
						},
						{
							Key: "_tab",
							Value: linkExampleValue(
								clicky.LinkCommand("stack/get").
									WithTarget(clicky.LinkTargetTab).
									WithArgs("stk-002").
									WithFlag("events", "6").
									WithAutoRun(true).
									Append("Open in new tab", "text-cyan-700 underline underline-offset-4"),
								"Produces a deep-link URL that opens the command page in a new tab.",
							),
						},
					},
				},
			},
		},
	}
}

func linkExampleValue(link api.Textable, note string) api.Text {
	return clicky.Text("").Add(link).Append(" ").Append(note, "text-slate-600")
}

// looksLikeAssetRequest returns true when the request targets a file with a
// known extension, so we don't swallow a genuine 404 (e.g. a missing image
// reference) with the SPA fallback.
func looksLikeAssetRequest(requested string) bool {
	ext := strings.ToLower(path.Ext(requested))
	switch ext {
	case ".js", ".mjs", ".css", ".map", ".ico", ".png", ".jpg", ".jpeg",
		".gif", ".svg", ".webp", ".woff", ".woff2", ".ttf", ".eot", ".txt":
		return true
	}
	return false
}
