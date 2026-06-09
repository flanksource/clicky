package docs

// Page is a logical docs page produced by the generator. Each page is written
// as a plain markdown file (<Key>.md) directly under the output directory.
type Page struct {
	Key         string // logical key + filename stem: "index", "stack", "ui-surfaces", ...
	Title       string
	Description string
	Body        string // markdown body
	Generated   bool   // regenerated every run (true) vs write-once starter (false)
}

// pageRelPath returns the page's filename relative to the output directory.
// Controller keys are bare command names (see controllerPageKey), so files land
// flat directly under --output-dir.
func pageRelPath(p Page) string { return p.Key + ".md" }
