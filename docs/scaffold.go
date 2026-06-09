package docs

import (
	"os"
	"path/filepath"
)

// WriteAction records what happened to one file during scaffolding.
type WriteAction struct {
	Path    string // path relative to the output directory
	Status  string // "written", "regenerated", or "skipped"
	Skipped bool
}

// ScaffoldResult is the outcome of writing a docs site, for reporting.
type ScaffoldResult struct {
	Dir     string
	Actions []WriteAction
}

// Scaffold writes the model's pages into dir using provider, applying the
// two-tier policy: Generated pages are always (re)written; starter pages are
// written only when absent, unless force is set. Returns a per-file report so
// the caller can show exactly what was written vs. skipped (no silent skips).
func Scaffold(m *Model, dir string, provider Provider, force bool) (*ScaffoldResult, error) {
	result := &ScaffoldResult{Dir: dir}

	for _, page := range buildPages(m) {
		rel := provider.RelPath(page)
		abs := filepath.Join(dir, rel)

		exists, err := fileExists(abs)
		if err != nil {
			return nil, err
		}

		// Write-once policy for starter pages.
		if !page.Generated && exists && !force {
			result.Actions = append(result.Actions, WriteAction{Path: rel, Status: "skipped", Skipped: true})
			continue
		}

		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			return nil, err
		}
		content := provider.Frontmatter(page) + page.Body
		if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
			return nil, err
		}

		status := "written"
		if page.Generated && exists {
			status = "regenerated"
		}
		result.Actions = append(result.Actions, WriteAction{Path: rel, Status: status})
	}

	return result, nil
}

func fileExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}
