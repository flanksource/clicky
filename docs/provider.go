package docs

import (
	"fmt"
	"sort"
	"strings"
)

const providerAstro = "astro"

// Page is a logical docs page produced by the generator. The Provider maps it
// to an on-disk relative path and prepends provider-specific frontmatter.
type Page struct {
	Key         string // logical key: "index", "cli", "ui-surfaces", "getting-started", "clicky-ui-integration"
	Title       string
	Description string
	Order       int    // sidebar order
	Body        string // markdown body without frontmatter
	Generated   bool   // regenerated every run (true) vs write-once starter (false)
}

// Provider maps logical pages onto a docs-site convention: where each page
// lives on disk and what frontmatter it carries.
type Provider interface {
	Name() string
	// RelPath returns the page's path relative to the output directory.
	RelPath(p Page) string
	// Frontmatter returns the leading frontmatter block (including trailing
	// newline separation), or "" if the provider uses none.
	Frontmatter(p Page) string
}

// providerFor returns the Provider for name, or an error listing supported
// providers. basePath places pages under a subdirectory of the provider's
// content root (e.g. "reference" to slot into an existing Starlight site).
// Fails loudly on unknown providers (no silent default).
func providerFor(name, basePath string) (Provider, error) {
	switch strings.ToLower(name) {
	case providerAstro:
		return astroProvider{basePath: cleanBasePath(basePath)}, nil
	default:
		return nil, fmt.Errorf("unsupported docs provider %q (supported: %s)", name, strings.Join(supportedProviders(), ", "))
	}
}

func supportedProviders() []string {
	names := []string{providerAstro}
	sort.Strings(names)
	return names
}

// cleanBasePath normalizes a user-supplied base path to a forward-slashed
// relative segment with no leading/trailing slashes.
func cleanBasePath(p string) string {
	p = strings.ReplaceAll(p, "\\", "/")
	return strings.Trim(p, "/")
}

// astroProvider targets Astro Starlight: pages live under src/content/docs/
// (optionally a basePath subdir) and carry YAML frontmatter with
// title/description and a sidebar order.
type astroProvider struct {
	basePath string
}

func (astroProvider) Name() string { return providerAstro }

func (a astroProvider) RelPath(p Page) string {
	parts := []string{"src", "content", "docs"}
	if a.basePath != "" {
		parts = append(parts, a.basePath)
	}
	parts = append(parts, p.Key+".md")
	return strings.Join(parts, "/")
}

func (astroProvider) Frontmatter(p Page) string {
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString("title: " + yamlScalar(p.Title) + "\n")
	if p.Description != "" {
		b.WriteString("description: " + yamlScalar(p.Description) + "\n")
	}
	b.WriteString(fmt.Sprintf("sidebar:\n  order: %d\n", p.Order))
	b.WriteString("---\n\n")
	return b.String()
}

// yamlScalar quotes a scalar when it contains characters that would otherwise
// break YAML parsing.
func yamlScalar(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if s == "" {
		return `""`
	}
	if strings.ContainsAny(s, ":#{}[],&*!|>'\"%@`") || strings.HasPrefix(s, " ") || strings.HasSuffix(s, " ") {
		return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"`
	}
	return s
}
