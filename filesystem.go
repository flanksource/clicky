package clicky

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/flanksource/clicky/api"
	"github.com/flanksource/clicky/api/icons"
)

type FileTreeOptions struct {
	MaxDepth     int  `flag:"depth" json:"depth,omitempty"`
	ShowHidden   bool `flag:"hidden" json:"hidden,omitempty"`
	ShowSize     bool `flag:"size" json:"size,omitempty"`
	ShowModified bool `flag:"modified" json:"modified,omitempty"`
	ShowAge      bool `flag:"age" json:"age,omitempty"`
}

func (f FileTreeOptions) Pretty() api.Text {
	t := api.Text{}
	t = t.Append("FileTreeOptions", "font-bold")
	t = t.Space().Append("MaxDepth: ", "font-bold").Append(f.MaxDepth)
	t = t.Space().Append("ShowHidden: ", "font-bold").Append(f.ShowHidden)
	t = t.Space().Append("ShowSize: ", "font-bold").Append(f.ShowSize)
	t = t.Space().Append("ShowModified: ", "font-bold").Append(f.ShowModified)
	t = t.Space().Append("ShowAge: ", "font-bold").Append(f.ShowAge)
	return t

}

// FileTreeNode represents a file or directory with metadata
type FileTreeNode struct {
	Name     string          `json:"name" pretty:"label"`
	Path     string          `json:"path"`
	Size     int64           `json:"size"`
	Modified time.Time       `json:"modified"`
	IsDir    bool            `json:"is_dir"`
	Children []*FileTreeNode `json:"children,omitempty" pretty:"format=tree"`
	options  FileTreeOptions `json:"-"`
}

// GetChildren implements TreeNode interface
func (f *FileTreeNode) GetChildren() []api.TreeNode {
	if f.Children == nil {
		return nil
	}
	nodes := make([]api.TreeNode, len(f.Children))
	for i, child := range f.Children {
		nodes[i] = child
	}
	return nodes
}

// Pretty returns a formatted Text with file info
func (f *FileTreeNode) Pretty() api.Text {
	t := api.Text{}

	if f.IsDir {
		t = t.Add(icons.Folder)
	} else {
		t = t.Add(icons.Filename(f.Name))
	}

	nameStyle := "text-gray-600"
	if f.IsDir {
		nameStyle = "text-blue-600 font-bold"
	} else if isExecutable(f.Path) {
		nameStyle = "text-green-600"
	}
	t = t.Space().Append(f.Name, nameStyle)

	if f.options.ShowAge {
		age := time.Since(f.Modified)
		t = t.Tab().Append(age, "text-gray-500")
	}

	if f.options.ShowModified {
		t = t.Tab().Append(f.Modified, "text-gray-500")
	}

	if !f.IsDir && f.options.ShowSize {
		t = t.Tab().Append(api.HumanizeBytes(f.Size), "text-orange-400")
	}

	return t

}

// FileSystemOption configures NewFileSystem behavior
type FileSystemOption func(*FileTreeOptions)

// WithMaxDepth sets the maximum directory depth to traverse
func WithMaxDepth(depth int) FileSystemOption {
	return func(c *FileTreeOptions) { c.MaxDepth = depth }
}

// WithHiddenFiles controls whether to show hidden files (starting with .)
func WithHiddenFiles(show bool) FileSystemOption {
	return func(c *FileTreeOptions) { c.ShowHidden = show }
}

// NewFileSystem creates a FileTreeNode from a directory path
func NewFileSystem(path string, opts ...FileSystemOption) *FileTreeNode {
	config := &FileTreeOptions{
		MaxDepth: 10,
	}
	for _, opt := range opts {
		opt(config)
	}

	Infof("Listing files in %s (%s)", path, *config)
	tree, err := buildFileTree(path, config.MaxDepth, 0, *config)
	if err != nil {
		return &FileTreeNode{
			Name:    filepath.Base(path),
			Path:    path,
			options: *config,
		}
	}
	return tree
}

func isExecutable(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	mode := info.Mode()
	return mode.IsRegular() && mode.Perm()&0o111 != 0
}

func buildFileTree(path string, maxDepth int, currentDepth int, options FileTreeOptions) (*FileTreeNode, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	node := &FileTreeNode{
		Name:     filepath.Base(path),
		Path:     path,
		Size:     info.Size(),
		Modified: info.ModTime(),
		IsDir:    info.IsDir(),
		options:  options,
	}

	if info.IsDir() && (maxDepth < 0 || currentDepth < maxDepth) {
		entries, err := os.ReadDir(path)
		if err != nil {
			return node, nil
		}

		for _, entry := range entries {
			if !options.ShowHidden && strings.HasPrefix(entry.Name(), ".") {
				continue
			}

			childPath := filepath.Join(path, entry.Name())
			childNode, err := buildFileTree(childPath, maxDepth, currentDepth+1, options)
			if err != nil {
				continue
			}
			node.Children = append(node.Children, childNode)
		}
	}

	return node, nil
}
