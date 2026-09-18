---
title: AI tools & MCP
description: Tool groups, approval defaults and MCP annotations on entity operations.
---

Every generated operation can also be exposed as an MCP tool (`mcp serve`) and to clicky's `aichat` layer. You describe how an AI client should treat the operations using tool metadata on the entity and its actions.

## Tool group

```go
clicky.NewEntity[Stack, StackListOpts, Stack]("stack").
	ToolGroup("infrastructure")
```

Every generated operation inherits the group. Tool-preference UIs use groups to enable or disable related tools together. An action can move to a different group with `.WithToolGroup("danger-zone")`.

## Default permission

```go
.ToolPermission(clicky.ToolPermissionAsk)
```

| Value | Meaning |
| --- | --- |
| `ToolPermissionAuto` (`auto`) | run without asking |
| `ToolPermissionAsk` (`ask`) | ask the user before each call |
| `ToolPermissionOn` (`on`) | enabled |
| `ToolPermissionOff` (`off`) | disabled by default |

The entity's permission applies to every operation. `WithToolPermission` on an action or bulk action overrides it. When nothing is set, the decision is left to the consuming application's approval policy. That policy usually looks at the safety hints below.

## MCP tool hints

```go
yes, no := true, false
clicky.NewEntity[Stack, StackListOpts, Stack]("stack").
	ToolHints(clicky.MCPToolHints{Icon: "server", Group: "infrastructure"}).
	WithAction(clicky.Action("restart", restart).WithToolHints(clicky.MCPToolHints{
		Title:           "Restart stack",
		DestructiveHint: &yes,
		IdempotentHint:  &no,
	})).
	WithBulkAction(clicky.BulkAction("purge", purge).
		WithToolPermission(clicky.ToolPermissionAsk).
		WithToolHints(clicky.MCPToolHints{Icon: "trash", DestructiveHint: &yes}))
```

| Field | Use |
| --- | --- |
| `Title` | display title for the tool |
| `ReadOnlyHint`, `DestructiveHint`, `IdempotentHint`, `OpenWorldHint` | MCP annotations. They are pointers because `false` is meaningful |
| `Icon` | icon name for the tool and for UI buttons (e.g. in the bulk selection toolbar) |
| `Group` | same as `ToolGroup` |
| `Parent` | parent tool for nesting in tool browsers |
| `DefaultPermission` | same as `ToolPermission` |
| `Strict` | request strict schema adherence from clients that support it |

Hints merge field by field: entity hints are inherited, and each non-empty action hint replaces the inherited value.

## Commands outside entities

For a command registered with `AddCommand` or built by hand, use `clicky.AnnotateTool(cmd, hints)`. `clicky.MarkLocalOnly(cmd)` keeps process-administration commands off the remote surfaces.

## Exposing tools

```go
root.AddCommand(
	mcp.NewMcpServer(root).
		AutoExpose().
		WithExclude("serve", "serve-ui").
		IgnoreParams("*", "--host", "--port").
		WithFormat(formatters.FormatOptions{Markdown: true, NoColor: true}).
		Command(),
)
```

Each operation's JSON schema comes from the same `ListOpts` and flags structs that generate the CLI, so the tool's arguments always match the CLI flags.
