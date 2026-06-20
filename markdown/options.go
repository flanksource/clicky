package markdown

// Option configures markdown parsing.
type Option func(*Options)

// Options controls how markdown source is parsed and preserved.
type Options struct {
	Filename       string
	GFM            bool
	Footnotes      bool
	Frontmatter    bool
	Admonitions    bool
	PreserveHTML   bool
	PreserveSource bool
	SourceSpans    bool
}

func defaultOptions() Options {
	return Options{
		GFM:            true,
		Footnotes:      true,
		Frontmatter:    true,
		Admonitions:    true,
		PreserveHTML:   true,
		PreserveSource: true,
		SourceSpans:    true,
	}
}

func applyOptions(opts ...Option) Options {
	o := defaultOptions()
	for _, opt := range opts {
		if opt != nil {
			opt(&o)
		}
	}
	return o
}

// WithFilename records the source filename in the parsed document metadata.
func WithFilename(filename string) Option {
	return func(o *Options) {
		o.Filename = filename
	}
}

// WithGFM enables or disables GitHub Flavored Markdown extensions.
func WithGFM(enabled bool) Option {
	return func(o *Options) {
		o.GFM = enabled
	}
}

// WithFootnotes enables or disables footnote parsing.
func WithFootnotes(enabled bool) Option {
	return func(o *Options) {
		o.Footnotes = enabled
	}
}

// WithFrontmatter enables or disables leading YAML frontmatter extraction.
func WithFrontmatter(enabled bool) Option {
	return func(o *Options) {
		o.Frontmatter = enabled
	}
}

// WithAdmonitions enables or disables pragmatic "!!! severity title" parsing.
func WithAdmonitions(enabled bool) Option {
	return func(o *Options) {
		o.Admonitions = enabled
	}
}

// WithPreserveHTML controls whether raw HTML nodes are retained.
func WithPreserveHTML(enabled bool) Option {
	return func(o *Options) {
		o.PreserveHTML = enabled
	}
}

// WithPreserveSource controls whether source markdown snippets are attached to nodes.
func WithPreserveSource(enabled bool) Option {
	return func(o *Options) {
		o.PreserveSource = enabled
	}
}

// WithSourceSpans controls whether line start/end metadata is attached to nodes.
func WithSourceSpans(enabled bool) Option {
	return func(o *Options) {
		o.SourceSpans = enabled
	}
}
