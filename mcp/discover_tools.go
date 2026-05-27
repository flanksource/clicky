package mcp

import (
	"fmt"
	"sort"
	"strings"

	"github.com/flanksource/clicky/api"
	"github.com/flanksource/clicky/formatters"
)

// BuildDiscoverToolsText returns a colorized, structured catalogue of the
// registered MCP tools as an api.Text. Caller picks the rendering format
// (ANSI for terminals, Markdown for AI clients, plain for text/no-color).
func BuildDiscoverToolsText(tools map[string]*ToolDefinition) api.Text {
	if len(tools) == 0 {
		return api.Text{}.
			Append("🔧 MCP Tool Discovery\n\n", "font-bold text-blue-500").
			Append("No tools are currently registered.", "italic text-gray-500")
	}

	names := make([]string, 0, len(tools))
	for n := range tools {
		names = append(names, n)
	}
	sort.Strings(names)

	out := api.Text{}.
		Append("🔧 MCP Tool Discovery", "font-bold text-blue-500").
		Append("\n").
		Append(fmt.Sprintf("%d tool(s) available. Each entry lists its purpose, parameters, and constraints.", len(names)), "italic text-gray-500").
		Append("\n\n")

	for _, name := range names {
		out = out.Add(renderToolText(name, tools[name]))
	}

	out = out.
		Append("\n").
		Append("———", "text-gray-400").
		Append("\n").
		Append("To call a tool, invoke it by name with the parameters above. ", "text-gray-500").
		Append("Required parameters must be supplied; optional ones fall back to their defaults.", "text-gray-500")

	return out
}

func renderToolText(name string, tool *ToolDefinition) api.Text {
	t := api.Text{}.
		Append(name, "font-bold text-yellow-400").
		Append("\n")

	if tool == nil {
		return t.Append("No definition available.\n\n", "italic text-gray-500")
	}

	if tool.Description != "" {
		t = t.Append(tool.Description, "text-gray-300").Append("\n")
	}

	if tool.InputSchema == nil {
		return t.Append("No parameters.\n\n", "italic text-gray-500")
	}

	props := tool.InputSchema.Properties
	if len(props) == 0 {
		return t.Append("No parameters.\n\n", "italic text-gray-500")
	}

	required := make(map[string]bool, len(tool.InputSchema.Required))
	for _, r := range tool.InputSchema.Required {
		required[r] = true
	}

	paramNames := make([]string, 0, len(props))
	for p := range props {
		paramNames = append(paramNames, p)
	}
	sort.Slice(paramNames, func(i, j int) bool {
		ri, rj := required[paramNames[i]], required[paramNames[j]]
		if ri != rj {
			return ri
		}
		return paramNames[i] < paramNames[j]
	})

	t = t.Append("Parameters:", "font-bold text-blue-400").Append("\n")
	for _, p := range paramNames {
		t = t.Add(renderParamText(p, props[p], required[p])).Append("\n")
	}
	return t.Append("\n")
}

func renderParamText(name string, prop Property, required bool) api.Text {
	row := api.Text{}.
		Append("  • ", "text-gray-400").
		Append(name, "font-bold text-pink-400")

	if prop.Type != "" {
		row = row.Append(" ").
			Append(prop.Type, "text-gray-300 bg-gray-800")
	}

	row = row.Append(" ")
	if required {
		row = row.Append("required", "font-bold text-white bg-red-600")
	} else {
		row = row.Append("optional", "text-white bg-gray-600")
	}

	if prop.Description != "" {
		row = row.Append(" — ").Append(prop.Description, "text-gray-300")
	}

	if len(prop.Enum) > 0 {
		row = row.Append(" ").
			Append(fmt.Sprintf("(values: %s)", strings.Join(prop.Enum, " | ")), "text-amber-300")
	}

	if prop.Default != nil {
		row = row.Append(" ").
			Append(fmt.Sprintf("(default: %v)", prop.Default), "italic text-green-400")
	}

	return row
}

// RenderDiscoverToolsString renders the catalogue as a plain string in the
// requested format. format is one of "markdown", "pretty"/"ansi", "plain"/"text".
// Empty defaults to Markdown.
func RenderDiscoverToolsString(tools map[string]*ToolDefinition, opts *formatters.FormatOptions) string {
	text := BuildDiscoverToolsText(tools)

	format := pickFormat(opts)
	switch format {
	case "ansi", "pretty":
		return text.ANSI()
	case "plain", "text":
		return text.String()
	case "markdown", "":
		fallthrough
	default:
		return text.Markdown()
	}
}

// pickFormat returns a single-word format key derived from FormatOptions.
// Boolean toggles (Markdown/Pretty/JSON) take precedence; falls back to the
// Format string field; defaults to "markdown".
func pickFormat(opts *formatters.FormatOptions) string {
	if opts == nil {
		return "markdown"
	}
	switch {
	case opts.Markdown:
		return "markdown"
	case opts.Pretty:
		if opts.NoColor {
			return "plain"
		}
		return "ansi"
	case opts.JSON, opts.YAML, opts.CSV, opts.HTML:
		return "markdown"
	}
	if f := strings.TrimSpace(strings.ToLower(opts.Format)); f != "" {
		return f
	}
	if opts.NoColor {
		return "plain"
	}
	return "markdown"
}
