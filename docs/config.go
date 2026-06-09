// Package docs provides a reusable cobra command group that generates a
// markdown CLI reference and a clicky-ui surface catalog from a CLI's command
// tree, as one markdown file per high-level command controller.
//
// It mirrors the rpc.NewOpenAPICommand / mcp.NewCommand factory pattern and is
// wired into a host CLI via extensions.CobraExtensions(root).DocsCommand().
package docs

const (
	// defaultDepth includes each controller plus its direct subcommands.
	defaultDepth = 1
	// unlimitedDepth includes a controller's entire subtree.
	unlimitedDepth = -1
)

// DocsConfig configures the docs command group. All fields are optional; unset
// values fall back to metadata derived from the root cobra command.
type DocsConfig struct {
	// Title is the docs-site title. Defaults to the root command's name.
	Title string
	// Description is the intro/landing-page blurb. Defaults to the root
	// command's Long (or Short) text.
	Description string
	// Exclude lists command paths (space-delimited, e.g. "admin secret") that
	// should be omitted from the generated CLI reference and UI catalog.
	Exclude []string
	// Depth limits how many command levels below each high-level controller
	// (direct child of root) are included in that controller's page. Depth 1
	// (the default) includes the controller and its direct subcommands; depth 2
	// descends one level further. Depth 0 uses the default; negative values mean
	// unlimited (the whole subtree).
	Depth int
}

// depth returns the effective controller depth, defaulting to 1.
func (c *DocsConfig) depth() int {
	if c == nil || c.Depth == 0 {
		return defaultDepth
	}
	if c.Depth < 0 {
		return unlimitedDepth
	}
	return c.Depth
}

// excluded reports whether the command path is in the Exclude list.
func (c *DocsConfig) excluded(cmdPath string) bool {
	if c == nil {
		return false
	}
	for _, e := range c.Exclude {
		if e == cmdPath {
			return true
		}
	}
	return false
}
