package api

// TreeNode defines the interface for hierarchical tree structures with visual metadata.
// Implementations provide display labels, child relationships, and styling information
// for consistent tree rendering across different output formats.
type TreeNode interface {
	GetLabel() string
	GetChildren() []TreeNode
	GetIcon() string
	GetStyle() string
	IsLeaf() bool
}

// PrettyNode extends TreeNode with rich text formatting capabilities.
type PrettyNode interface {
	Pretty() Text
}

// TreeOptions controls tree rendering behavior including visual styling,
// depth limits, and character sets for drawing tree connections.
type TreeOptions struct {
	ShowIcons      bool            `json:"show_icons,omitempty" yaml:"show_icons,omitempty"`
	IndentSize     int             `json:"indent_size,omitempty" yaml:"indent_size,omitempty"`
	UseUnicode     bool            `json:"use_unicode,omitempty" yaml:"use_unicode,omitempty"`
	Compact        bool            `json:"compact,omitempty" yaml:"compact,omitempty"`
	MaxDepth       int             `json:"max_depth,omitempty" yaml:"max_depth,omitempty"`
	CollapsedNodes map[string]bool `json:"collapsed_nodes,omitempty" yaml:"collapsed_nodes,omitempty"`
	// Prefix characters for tree rendering
	BranchPrefix   string `json:"branch_prefix,omitempty" yaml:"branch_prefix,omitempty"`
	LastPrefix     string `json:"last_prefix,omitempty" yaml:"last_prefix,omitempty"`
	IndentPrefix   string `json:"indent_prefix,omitempty" yaml:"indent_prefix,omitempty"`
	ContinuePrefix string `json:"continue_prefix,omitempty" yaml:"continue_prefix,omitempty"`
}

// DefaultTreeOptions creates configuration for Unicode tree rendering
// with standard indentation and unlimited depth.
func DefaultTreeOptions() *TreeOptions {
	return &TreeOptions{
		ShowIcons:      true,
		IndentSize:     2,
		UseUnicode:     true,
		Compact:        false,
		MaxDepth:       -1, // No limit
		CollapsedNodes: make(map[string]bool),
		// Unicode box drawing characters
		BranchPrefix:   "├── ",
		LastPrefix:     "└── ",
		IndentPrefix:   "    ",
		ContinuePrefix: "│   ",
	}
}

// ASCIITreeOptions creates configuration for ASCII-only tree rendering,
// suitable for environments without Unicode support.
func ASCIITreeOptions() *TreeOptions {
	opts := DefaultTreeOptions()
	opts.UseUnicode = false
	opts.BranchPrefix = "+-- "
	opts.LastPrefix = "`-- "
	opts.IndentPrefix = "    "
	opts.ContinuePrefix = "|   "
	return opts
}

// SimpleTreeNode provides a straightforward TreeNode implementation
// with support for labels, icons, styling, and arbitrary metadata.
type SimpleTreeNode struct {
	Label    string                 `json:"label" yaml:"label"`
	Icon     string                 `json:"icon,omitempty" yaml:"icon,omitempty"`
	Style    string                 `json:"style,omitempty" yaml:"style,omitempty"`
	Children []TreeNode             `json:"children,omitempty" yaml:"children,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}

func (n *SimpleTreeNode) GetLabel() string {
	return n.Label
}

func (n *SimpleTreeNode) GetChildren() []TreeNode {
	return n.Children
}

func (n *SimpleTreeNode) GetIcon() string {
	return n.Icon
}

func (n *SimpleTreeNode) GetStyle() string {
	return n.Style
}

func (n *SimpleTreeNode) IsLeaf() bool {
	return len(n.Children) == 0
}

// CompactListNode renders multiple items inline rather than as nested children,
// useful for displaying arrays or lists within tree structures.
type CompactListNode struct {
	Label    string                 `json:"label" yaml:"label"`
	Icon     string                 `json:"icon,omitempty" yaml:"icon,omitempty"`
	Style    string                 `json:"style,omitempty" yaml:"style,omitempty"`
	Items    []string               `json:"items" yaml:"items"`
	Metadata map[string]interface{} `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}

func (n *CompactListNode) GetLabel() string {
	return n.Label
}

func (n *CompactListNode) GetChildren() []TreeNode {
	return nil
}

func (n *CompactListNode) GetIcon() string {
	return n.Icon
}

func (n *CompactListNode) GetStyle() string {
	return n.Style
}

func (n *CompactListNode) IsLeaf() bool {
	return true
}

func (n *CompactListNode) GetItems() []string {
	return n.Items
}
