package api

import "html"

// plainCodeANSI renders code for a terminal without syntax highlighting. It is
// the wasm build's Code.ANSI; properties files keep the native key/value
// styling because that formatting does not depend on chroma.
func plainCodeANSI(c Code) string {
	switch {
	case c.Content == "":
		return ""
	case isPropertiesLanguage(c.Language):
		return formatProperties(c.Content).ANSI()
	}
	return c.Content
}

// plainCodeHTML renders code as an escaped <pre><code> block tagged with a
// `language-*` class (omitted when the language is unknown), the convention
// client-side highlighters such as highlight.js and Prism pick up. It is the
// wasm build's Code.HTML.
func plainCodeHTML(c Code) string {
	switch {
	case c.Content == "":
		return ""
	case isPropertiesLanguage(c.Language):
		return formatProperties(c.Content).HTML()
	}
	open := "<pre><code>"
	if c.Language != "" {
		open = `<pre><code class="language-` + html.EscapeString(c.Language) + `">`
	}
	return open + html.EscapeString(c.Content) + "</code></pre>"
}
