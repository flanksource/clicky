package docs

import "fmt"

// RenderCLIReference produces the markdown CLI reference body (no frontmatter).
// It documents every controller in order, each as a section.
func RenderCLIReference(m *Model) string {
	md := &mdBuilder{}
	md.heading(1, "CLI Reference")
	md.para(fmt.Sprintf("Command reference for `%s`, generated from the live command tree.", m.Title))

	for _, ctrl := range m.Controllers {
		md.heading(2, code(ctrl.Name))
		if ctrl.Short != "" {
			md.para(ctrl.Short)
		}
		if ctrl.Long != "" && ctrl.Long != ctrl.Short {
			md.para(ctrl.Long)
		}
		for _, c := range ctrl.Commands {
			renderCommand(md, c, 3)
		}
	}
	return md.String()
}

// RenderController produces the markdown body for a single controller's page
// (no frontmatter): the controller summary followed by each of its commands.
func RenderController(ctrl ControllerDoc) string {
	md := &mdBuilder{}
	md.heading(1, ctrl.Name)
	if ctrl.Short != "" {
		md.para(ctrl.Short)
	}
	if ctrl.Long != "" && ctrl.Long != ctrl.Short {
		md.para(ctrl.Long)
	}
	for _, c := range ctrl.Commands {
		renderCommand(md, c, 2)
	}
	return md.String()
}

func renderCommand(md *mdBuilder, c CommandDoc, level int) {
	md.heading(level, code(c.Path))
	if c.Short != "" {
		md.para(c.Short)
	}
	if c.Long != "" && c.Long != c.Short {
		md.para(c.Long)
	}

	md.line("**Usage:**").blank()
	md.codeBlock("", c.Use)

	if len(c.Aliases) > 0 {
		md.para("**Aliases:** " + code(joinCode(c.Aliases)))
	}

	if len(c.Flags) > 0 {
		md.line("**Flags:**").blank()
		rows := make([][]string, 0, len(c.Flags))
		for _, f := range c.Flags {
			name := "--" + f.Name
			if f.Shorthand != "" {
				name += ", -" + f.Shorthand
			}
			rows = append(rows, []string{
				code(name), code(f.Type), defaultStr(f.Default), boolMark(f.Required), f.Usage,
			})
		}
		md.table([]string{"Flag", "Type", "Default", "Required", "Description"}, rows)
	}

	if c.Example != "" {
		md.line("**Examples:**").blank()
		md.codeBlock("sh", c.Example)
	}
}

func joinCode(items []string) string {
	out := ""
	for i, s := range items {
		if i > 0 {
			out += ", "
		}
		out += s
	}
	return out
}
