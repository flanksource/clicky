package icons

import (
	"path/filepath"
	"strings"
)

func Filename(name string) Icon {

	switch filepath.Base(name) {
	case "Dockerfile":
		return Docker
	case "kustomization.yaml", "kustomization.yml":
		return Kustomize
	case "Makefile", "Taskfile":
		return Makefile
	case "package.json", "package-lock.json", "yarn.lock", "pnpm-lock.yaml":
		return NPM
	case "go.mod", "go.sum":
		return Golang
	case "requirements.txt", "Pipfile", "Pipfile.lock":
		return Python
	case "README.md", "README.txt", "README":
		return Docs
	}
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".go":
		return Golang
	case ".py":
		return Python

	case ".yaml", ".yml":
		return YAML
	case ".json":
		return JSON
	case ".md", ".txt":
		return Markdown
	case ".zip", ".tar", ".gz", ".rar":
		return Archive
	case ".jpg", ".jpeg", ".png", ".gif", ".svg":
		return Image
	case ".mp4", ".avi", ".mov", ".mkv":
		return Video
	case ".mp3", ".wav", ".flac":
		return Audio
	case ".jsx", ".tsx":
		return React
	case ".js":
		return JS
	case ".ts":
		return TypeScript
	case ".java":
		return Java
	case ".rb":
		return Ruby

	case ".xml":
		return XML
	case ".csv":
		return CSV

	case ".pdf":
		return PDF
	case ".css", ".scss", ".less":
		return CSS
	case ".html", ".htm":
		return HTML
	case ".sh", ".bash":
		return Terminal
	case ".exe", ".app":
		return Executable
	}

	return File
}
