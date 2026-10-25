package main

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/flanksource/clicky/api/filter/celfilter"
	"github.com/flanksource/clicky/examples/enitity/internal/entitydemo"
	"github.com/flanksource/clicky/formatters/excel"
	"github.com/flanksource/clicky/formatters/pdf"
)

//go:embed webapp/dist
var webappFS embed.FS

func main() {
	celfilter.Use()
	excel.Use()
	pdf.Use()
	rootCmd := entitydemo.NewCommand(entitydemo.CommandOptions{
		EmbeddedWebapp:      webappFS,
		ResolveWebappDevDir: resolveWebappDevDir,
	})
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func resolveWebappDevDir() (string, error) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("cannot locate webapp/ for --dev: runtime caller unavailable")
	}
	dir := filepath.Join(filepath.Dir(thisFile), "webapp")
	if _, err := os.Stat(filepath.Join(dir, "package.json")); err != nil {
		return "", fmt.Errorf("webapp/package.json not found at %s: %w", dir, err)
	}
	return dir, nil
}
