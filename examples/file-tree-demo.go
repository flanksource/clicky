package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/flanksource/clicky/api"
	"github.com/flanksource/clicky/formatters"

	"github.com/spf13/cobra"
)

// FileTreeNode represents a file or directory with metadata
type FileTreeNode struct {
	Name     string          `json:"name" pretty:"label"`
	Path     string          `json:"path"`
	Size     int64           `json:"size"`
	Modified time.Time       `json:"modified"`
	IsDir    bool            `json:"is_dir"`
	Children []*FileTreeNode `json:"children,omitempty" pretty:"format=tree"`
}

// Implement TreeNode interface
func (f FileTreeNode) GetLabel() string {
	return f.Name
}

func (f FileTreeNode) GetChildren() []api.TreeNode {
	if f.Children == nil {
		return nil
	}
	nodes := make([]api.TreeNode, len(f.Children))
	for i, child := range f.Children {
		nodes[i] = child // Return pointer (which implements both TreeNode and Pretty)
	}
	return nodes
}

func (f FileTreeNode) GetIcon() string {
	// Icons are included in Pretty() method
	return ""
}

func (f FileTreeNode) GetStyle() string {
	// Style is handled in Pretty() method
	return ""
}

func (f FileTreeNode) IsLeaf() bool {
	return len(f.Children) == 0
}

// Pretty returns a formatted Text with file info
func (f *FileTreeNode) Pretty() api.Text {
	// Choose icon based on file type
	var icon string
	if f.IsDir {
		icon = "📁"
	} else {
		ext := strings.ToLower(filepath.Ext(f.Name))
		switch ext {
		case ".go":
			icon = "🐹"
		case ".py":
			icon = "🐍"
		case ".js", ".ts", ".jsx", ".tsx":
			icon = "📜"
		case ".json", ".yaml", ".yml":
			icon = "🔧"
		case ".md", ".txt":
			icon = "📝"
		case ".zip", ".tar", ".gz", ".rar":
			icon = "🗜️"
		case ".jpg", ".jpeg", ".png", ".gif", ".svg":
			icon = "🖼️"
		case ".mp4", ".avi", ".mov", ".mkv":
			icon = "🎬"
		case ".mp3", ".wav", ".flac":
			icon = "🎵"
		case ".pdf":
			icon = "📕"
		case ".html", ".css":
			icon = "🌐"
		case ".sh", ".bash":
			icon = "🔨"
		case ".exe", ".app":
			icon = "⚙️"
		default:
			icon = "📄"
		}
	}

	// Build components with adaptive colors
	// The theme system will automatically adjust colors based on terminal background
	nameStyle := "text-gray-600"
	if f.IsDir {
		nameStyle = "text-blue-600 font-bold"
	} else if isExecutable(f.Path) {
		nameStyle = "text-green-600"
	}

	// Format size
	sizeStr := ""
	if !f.IsDir {
		sizeStr = formatFileSize(f.Size)
	}

	// Format time
	timeStr := formatRelativeTime(f.Modified)

	// Create main text with icon and name
	mainText := api.Text{
		Content: fmt.Sprintf("%s %s", icon, f.Name),
		Style:   nameStyle,
	}

	// Add metadata as children
	var children []api.Text

	if sizeStr != "" {
		children = append(children, api.Text{
			Content: fmt.Sprintf("  %s", sizeStr),
			Style:   "text-gray-500 text-sm",
		})
	}

	children = append(children, api.Text{
		Content: fmt.Sprintf("  %s", timeStr),
		Style:   "text-gray-400 text-sm italic",
	})

	mainText.Children = children
	return mainText
}

// Helper functions

func formatFileSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(size)/float64(div), "KMGTPE"[exp])
}

func formatRelativeTime(t time.Time) string {
	duration := time.Since(t)

	switch {
	case duration < time.Minute:
		return "just now"
	case duration < time.Hour:
		mins := int(duration.Minutes())
		if mins == 1 {
			return "1 min ago"
		}
		return fmt.Sprintf("%d mins ago", mins)
	case duration < 24*time.Hour:
		hours := int(duration.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	case duration < 7*24*time.Hour:
		days := int(duration.Hours() / 24)
		if days == 1 {
			return "yesterday"
		}
		return fmt.Sprintf("%d days ago", days)
	case duration < 30*24*time.Hour:
		weeks := int(duration.Hours() / (24 * 7))
		if weeks == 1 {
			return "1 week ago"
		}
		return fmt.Sprintf("%d weeks ago", weeks)
	case duration < 365*24*time.Hour:
		months := int(duration.Hours() / (24 * 30))
		if months == 1 {
			return "1 month ago"
		}
		return fmt.Sprintf("%d months ago", months)
	default:
		years := int(duration.Hours() / (24 * 365))
		if years == 1 {
			return "1 year ago"
		}
		return fmt.Sprintf("%d years ago", years)
	}
}

func isExecutable(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	mode := info.Mode()
	return mode.IsRegular() && mode.Perm()&0111 != 0
}

func main() {
	if err := newRootCommand().Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func newRootCommand() *cobra.Command {
	var formatOpts formatters.FormatOptions
	var maxDepth int
	var showHidden bool

	cmd := &cobra.Command{
		Use:   "file-tree-demo [directory]",
		Short: "Display a directory tree with rich formatting",
		Long: `Display a directory tree with rich formatting.

Supports multiple output formats and adaptive terminal coloring.
The colors will automatically adjust based on your terminal's background
(light or dark) for optimal readability.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Determine directory to scan
			scanDir := "."
			if len(args) > 0 {
				scanDir = args[0]
			}

			if formatOpts.Verbose {
				fmt.Fprintf(os.Stderr, "Scanning directory: %s\n", scanDir)
			}

			// Build file tree
			tree, err := buildFileTree(scanDir, maxDepth, 0, showHidden)
			if err != nil {
				return fmt.Errorf("failed to build file tree: %w", err)
			}

			// Wrap in a struct for formatting
			// The pretty:"tree" tag ensures the tree formatter is used
			type FileSystem struct {
				Root *FileTreeNode `json:"root" yaml:"root" pretty:"tree"`
			}

			fs := FileSystem{Root: tree}

			// Use FormatManager for all formats - now supports tree rendering consistently
			manager := formatters.NewFormatManager()
			if err := manager.FormatToFile(formatOpts, fs); err != nil {
				return err
			}

			// Show summary for pretty format with verbose
			if formatOpts.Format == "pretty" && formatOpts.Verbose {
				showSummary(tree)
			}

			return nil
		},
	}

	// Use clicky's built-in BindPFlags for format options
	formatters.BindPFlags(cmd.Flags(), &formatOpts)

	// Add additional flags specific to file-tree
	cmd.Flags().IntVar(&maxDepth, "max-depth", 3, "Maximum depth to traverse")
	cmd.Flags().BoolVar(&showHidden, "show-hidden", false, "Show hidden files and directories")

	return cmd
}

// buildFileTree builds a FileTreeNode from a directory
func buildFileTree(path string, maxDepth int, currentDepth int, showHidden bool) (*FileTreeNode, error) {
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
	}

	// If it's a directory and we haven't reached max depth, read children
	if info.IsDir() && (maxDepth < 0 || currentDepth < maxDepth) {
		entries, err := os.ReadDir(path)
		if err != nil {
			// Skip directories we can't read
			return node, nil
		}

		for _, entry := range entries {
			// Skip hidden files unless requested
			if !showHidden && strings.HasPrefix(entry.Name(), ".") {
				continue
			}

			childPath := filepath.Join(path, entry.Name())
			childNode, err := buildFileTree(childPath, maxDepth, currentDepth+1, showHidden)
			if err != nil {
				// Skip files we can't stat
				continue
			}
			node.Children = append(node.Children, childNode)
		}
	}

	return node, nil
}

// showSummary displays statistics about the file tree
func showSummary(tree *FileTreeNode) {
	var totalFiles, totalDirs int
	var totalSize int64

	var countNode func(*FileTreeNode)
	countNode = func(n *FileTreeNode) {
		if n.IsDir {
			totalDirs++
		} else {
			totalFiles++
			totalSize += n.Size
		}
		for _, child := range n.Children {
			countNode(child)
		}
	}

	countNode(tree)

	fmt.Fprintf(os.Stderr, "\nSummary:\n")
	fmt.Fprintf(os.Stderr, "  Directories: %d\n", totalDirs)
	fmt.Fprintf(os.Stderr, "  Files: %d\n", totalFiles)
	fmt.Fprintf(os.Stderr, "  Total Size: %s\n", formatFileSize(totalSize))
}
