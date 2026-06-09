package docs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

// Scaffold writes the model's pages as flat markdown files directly under dir,
// applying the two-tier policy: Generated pages are always (re)written; starter
// pages are written only when absent, unless force is set. Returns a per-file
// report so the caller can show exactly what was written vs. skipped (no silent
// skips).
func Scaffold(m *Model, dir string, force bool) (*ScaffoldResult, error) {
	result := &ScaffoldResult{Dir: dir}

	pages := buildPages(m)
	if err := assertNoPathCollisions(pages); err != nil {
		return nil, err
	}

	for _, page := range pages {
		rel := pageRelPath(page)
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
		if err := os.WriteFile(abs, []byte(page.Body), 0o644); err != nil {
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

// assertNoPathCollisions fails loudly when two pages map to the same on-disk
// path — e.g. a controller named "index" would otherwise clobber the landing
// page. Better a clear error than a silent overwrite.
func assertNoPathCollisions(pages []Page) error {
	seen := map[string]string{}
	for _, p := range pages {
		if err := validatePageKey(p.Key); err != nil {
			return err
		}
		rel := pageRelPath(p)
		if other, ok := seen[rel]; ok {
			return fmt.Errorf("docs page collision: %q and %q both map to %s (rename the conflicting command)", other, p.Key, rel)
		}
		seen[rel] = p.Key
	}
	return nil
}

func validatePageKey(key string) error {
	if key == "" || key == "." || key == ".." {
		return fmt.Errorf("invalid docs page key %q", key)
	}
	if strings.ContainsAny(key, `/\`) || filepath.Clean(key) != key {
		return fmt.Errorf("invalid docs page key %q: page keys must be flat filenames", key)
	}
	return nil
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
