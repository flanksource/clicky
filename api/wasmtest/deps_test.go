//go:build !wasm

package wasmtest

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"
)

// browserPackages are the api packages a browser (GOOS=js) build may import.
var browserPackages = []string{
	"github.com/flanksource/clicky/api",
	"github.com/flanksource/clicky/api/icons",
	"github.com/flanksource/clicky/api/tailwind",
}

// forbiddenModules are heavy or terminal-only modules the browser build of
// api must not reach; each has a plain replacement behind the wasm build tag.
var forbiddenModules = []string{
	"github.com/alecthomas/chroma",
	"github.com/charmbracelet/lipgloss",
	"github.com/google/cel-go",
	"github.com/fatih/color",
	"github.com/flanksource/commons",
	"github.com/flanksource/gomplate",
	"github.com/google/uuid",
	"github.com/olekukonko/tablewriter",
	"github.com/samber/lo",
	"golang.org/x/text",
}

type listedPackage struct {
	ImportPath string
	Imports    []string
}

func TestBrowserBuildOfAPIAvoidsForbiddenModules(t *testing.T) {
	packages := listJSDeps(t)
	for _, pkg := range packages {
		for _, module := range forbiddenModules {
			if pkg.ImportPath == module || strings.HasPrefix(pkg.ImportPath, module+"/") {
				t.Errorf("js/wasm build imports %s: %s", module, strings.Join(importChain(packages, pkg.ImportPath), " -> "))
			}
		}
	}
}

func listJSDeps(t *testing.T) map[string]listedPackage {
	t.Helper()
	cmd := exec.Command("go", append([]string{"list", "-deps", "-json=ImportPath,Imports"}, browserPackages...)...)
	cmd.Env = append(os.Environ(), "GOOS=js", "GOARCH=wasm")
	cmd.Stderr = os.Stderr
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	packages := map[string]listedPackage{}
	decoder := json.NewDecoder(out)
	for {
		var pkg listedPackage
		err := decoder.Decode(&pkg)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("decode go list output: %v", err)
		}
		packages[pkg.ImportPath] = pkg
	}
	if err := cmd.Wait(); err != nil {
		t.Fatalf("GOOS=js GOARCH=wasm go list -deps %v: %v", browserPackages, err)
	}
	return packages
}

// importChain returns the shortest import path from a browser package to target.
func importChain(packages map[string]listedPackage, target string) []string {
	parent := map[string]string{}
	queue := slices.Clone(browserPackages)
	for _, root := range browserPackages {
		parent[root] = ""
	}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		if current == target {
			chain := []string{}
			for at := current; at != ""; at = parent[at] {
				chain = append(chain, at)
			}
			slices.Reverse(chain)
			return chain
		}
		for _, next := range packages[current].Imports {
			if _, seen := parent[next]; !seen {
				parent[next] = current
				queue = append(queue, next)
			}
		}
	}
	return []string{target}
}
