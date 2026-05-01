package clicky

// MultiFilter is a string-list flag whose values are interpreted with
// collections.MatchItems semantics: plain values include, !value excludes.
type MultiFilter []string
