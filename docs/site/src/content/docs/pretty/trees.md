---
title: Trees
description: Render hierarchies with TreeNode, TreeMixin or explicit clicky.Tree values.
---

## TreeNode

Implement `api.TreeNode` on a hierarchical type. `Pretty()` renders **only the current node**, and `GetChildren()` exposes the hierarchy:

```go
type Dir struct {
	Name     string
	Children []Dir
}

func (d Dir) Pretty() api.Text { return clicky.Text(d.Name, "font-bold") }

func (d Dir) GetChildren() []api.TreeNode {
	children := make([]api.TreeNode, 0, len(d.Children))
	for _, c := range d.Children {
		children = append(children, c)
	}
	return children
}

clicky.MustPrint(api.NewTree(root))
```

```text
src
├── api
│   ╰── text.go
╰── main.go
```

- Return `nil` from `GetChildren()` for leaves.
- Never render children inside `Pretty()`. The tree formatter owns indentation and connectors.
- `api.NewTree(nodes...)` converts `TreeNode`s to an `api.TextTree`. A single root is returned as that root.

## TreeMixin

When a type is not itself hierarchical but has a tree view, implement `Tree() api.TreeNode`:

```go
func (p Plan) Tree() api.TreeNode { return p.rootStep() }
```

## Explicit trees

Build a tree directly from `Textable` nodes:

```go
clicky.Tree(clicky.Text("cluster", "font-bold"),
	clicky.Tree(clicky.Text("node-1")),
	clicky.Tree(clicky.Text("node-2"),
		clicky.Tree(clicky.Text("pod-a")),
	),
)
```

## Trees inside structs

Tag a field whose value is a tree with `pretty:"tree"`, and tune it with tree options:

```go
type Report struct {
	Name  string
	Steps Step `pretty:"tree,max_depth=3,ascii"`
}
```

| Option | Effect |
| --- | --- |
| `max_depth=N` | stop descending after N levels |
| `indent=N` | indentation per level |
| `ascii` | `+--` / `` `-- `` connectors instead of Unicode |
| `no_icons` | hide node icons |

The `tree` output format (`--format tree`) renders the first field tagged `tree`. A value with no tree field reports "No tree data found" together with the fields it did find.

## Format support

Trees render with connectors in the terminal and as nested collapsible lists in HTML. The `--filter` CEL expression also applies to tree nodes. In Markdown, deep trees come out flat, so prefer a table or nested lists when the tree is primarily read as Markdown.
