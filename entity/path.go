package entity

import "strings"

// PathSeparator is the wire delimiter for ClickySurface.Path. Producers split
// their own naming convention (dots, slashes, colons — whatever the domain
// uses) with SplitPath and emit the result joined by this, so a frontend never
// has to know, or guess, the producer's convention.
const PathSeparator = "/"

// SplitPath breaks name into hierarchy segments on any rune in delimiters,
// dropping empty segments so repeated or leading separators are harmless.
// Runes outside delimiters are never separators — SplitPath("a-b.c", ".")
// yields ["a-b", "c"], not three segments.
//
// PathSeparator is always a separator, whatever delimiters says (including when
// it is empty): a segment containing it would be re-split by any consumer of the
// JoinPath output, so SplitPath("jms/api.incoming", ".") yields
// ["jms", "api", "incoming"] and the round-trip through JoinPath is stable.
//
// A name with no delimiter yields a single segment, i.e. a root-level leaf.
func SplitPath(name, delimiters string) []string {
	if name == "" {
		return nil
	}
	fields := strings.FieldsFunc(name, func(r rune) bool {
		return strings.ContainsRune(PathSeparator, r) || strings.ContainsRune(delimiters, r)
	})
	if len(fields) == 0 {
		return nil
	}
	return fields
}

// JoinPath renders segments as a ClickySurface.Path. Empty segments are
// dropped so the output always round-trips through SplitPath(_, PathSeparator).
func JoinPath(segments []string) string {
	kept := make([]string, 0, len(segments))
	for _, segment := range segments {
		if segment != "" {
			kept = append(kept, segment)
		}
	}
	return strings.Join(kept, PathSeparator)
}
