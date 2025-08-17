package api

// TreeNode represents a node in a hierarchical tree structure
type TreeNode interface {
	GetLabel() string
	GetChildren() []TreeNode
	GetIcon() string
	GetStyle() string
	IsLeaf() bool
}

// PrettyNode interface for nodes that can format themselves with rich text
type PrettyNode interface {
	Pretty() Text
}

// TreeOptions configures how trees are rendered
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

// DefaultTreeOptions returns default tree rendering options
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

// ASCIITreeOptions returns tree options using ASCII characters
func ASCIITreeOptions() *TreeOptions {
	opts := DefaultTreeOptions()
	opts.UseUnicode = false
	opts.BranchPrefix = "+-- "
	opts.LastPrefix = "`-- "
	opts.IndentPrefix = "    "
	opts.ContinuePrefix = "|   "
	return opts
}

// SimpleTreeNode provides a basic tree node implementation
type SimpleTreeNode struct {
	Label    string     `json:"label" yaml:"label"`
	Icon     string     `json:"icon,omitempty" yaml:"icon,omitempty"`
	Style    string     `json:"style,omitempty" yaml:"style,omitempty"`
	Children []TreeNode `json:"children,omitempty" yaml:"children,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}

// GetLabel returns the node's label
func (n *SimpleTreeNode) GetLabel() string {
	return n.Label
}

// GetChildren returns the node's children
func (n *SimpleTreeNode) GetChildren() []TreeNode {
	return n.Children
}

// GetIcon returns the node's icon
func (n *SimpleTreeNode) GetIcon() string {
	return n.Icon
}

// GetStyle returns the node's style
func (n *SimpleTreeNode) GetStyle() string {
	return n.Style
}

// IsLeaf returns true if the node has no children
func (n *SimpleTreeNode) IsLeaf() bool {
	return len(n.Children) == 0
}

// CompactListNode represents a node that renders its children as a compact list
type CompactListNode struct {
	Label    string                 `json:"label" yaml:"label"`
	Icon     string                 `json:"icon,omitempty" yaml:"icon,omitempty"`
	Style    string                 `json:"style,omitempty" yaml:"style,omitempty"`
	Items    []string               `json:"items" yaml:"items"`
	Metadata map[string]interface{} `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}

// GetLabel returns the node's label
func (n *CompactListNode) GetLabel() string {
	return n.Label
}

// GetChildren returns empty slice as items are rendered inline
func (n *CompactListNode) GetChildren() []TreeNode {
	return nil
}

// GetIcon returns the node's icon
func (n *CompactListNode) GetIcon() string {
	return n.Icon
}

// GetStyle returns the node's style
func (n *CompactListNode) GetStyle() string {
	return n.Style
}

// IsLeaf returns true as compact lists are treated as leaf nodes
func (n *CompactListNode) IsLeaf() bool {
	return true
}

// GetItems returns the compact list items
func (n *CompactListNode) GetItems() []string {
	return n.Items
}