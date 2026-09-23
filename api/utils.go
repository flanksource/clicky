package api

import (
	"strings"
	"unicode"
)

// PrettifyFieldName converts field names to readable format
func PrettifyFieldName(name string) string {
	// Convert snake_case and camelCase to Title Case
	var result strings.Builder

	// First try to split on underscores and dashes
	words := strings.FieldsFunc(name, func(r rune) bool {
		return r == '_' || r == '-'
	})

	// If we only got one word (no underscores/dashes), try camelCase splitting
	if len(words) == 1 {
		words = SplitCamelCase(name)
	}

	for i, word := range words {
		if i > 0 {
			result.WriteString(" ")
		}
		result.WriteString(titleWord(word))
	}

	return result.String()
}

// titleWord lowercases word and title-cases the first cased rune of every
// word inside it, byte-identical to the English Unicode title caser of the
// x/text cases package: words break on anything but letters, digits, marks, connector and
// format runes, and a single mid-word rune (' . :) joins, while two in a row
// break.
func titleWord(word string) string {
	var result strings.Builder
	inWord, prevMid := false, false
	for _, r := range strings.ToLower(word) {
		mid := unicode.Is(wordMidRunes, r)
		if prevMid && mid {
			inWord = false
		}
		prevMid = mid
		switch {
		case !unicode.In(r, casedRunes...):
			result.WriteRune(r)
			if !mid && !joinsWord(r) {
				inWord = false
			}
		case inWord:
			result.WriteRune(r)
		case fullTitleCase[r] != "":
			result.WriteString(fullTitleCase[r])
			inWord = true
		default:
			result.WriteRune(unicode.ToTitle(r))
			inWord = true
		}
	}
	return result.String()
}

// joinsWord reports whether an uncased, non-mid rune continues a word, i.e.
// its Unicode word-break category is ALetter, Hebrew_Letter, Numeric, Extend,
// ExtendNumLet, Format or ZWJ.
func joinsWord(r rune) bool {
	switch {
	case unicode.Is(wordJoiningSymbols, r):
		return true
	case unicode.Is(wordBreakingFormats, r):
		return false
	case unicode.In(r, unicode.M, unicode.Nd, unicode.Pc, unicode.Cf):
		return true
	}
	return unicode.In(r, unicode.L, unicode.Nl, unicode.Other_Alphabetic) && !unicode.In(r, wordBreakingScripts...)
}

var (
	casedRunes = []*unicode.RangeTable{unicode.Upper, unicode.Lower, unicode.Title, unicode.Other_Lowercase, unicode.Other_Uppercase}

	// wordBreakingScripts hold letters that are not ALetter: ideographs, kana
	// and the scripts whose words are found by dictionary (Line_Break=SA).
	wordBreakingScripts = []*unicode.RangeTable{
		unicode.Han, unicode.Hiragana, unicode.Katakana, unicode.Tangut, unicode.Khitan_Small_Script, unicode.Nushu,
		unicode.Thai, unicode.Lao, unicode.Myanmar, unicode.Khmer, unicode.Tai_Le, unicode.New_Tai_Lue, unicode.Tai_Tham, unicode.Tai_Viet, unicode.Ahom,
	}

	// wordJoiningSymbols are the modifier symbols, punctuation and iteration
	// marks that the word-break property lists as ALetter, ExtendNumLet or
	// Extend despite their general category or script.
	wordJoiningSymbols = &unicode.RangeTable{
		R16: []unicode.Range16{
			{Lo: 0x02c2, Hi: 0x02c5, Stride: 1},
			{Lo: 0x02d2, Hi: 0x02d7, Stride: 1},
			{Lo: 0x02de, Hi: 0x02df, Stride: 1},
			{Lo: 0x02e5, Hi: 0x02eb, Stride: 1},
			{Lo: 0x02ed, Hi: 0x02ed, Stride: 1},
			{Lo: 0x02ef, Hi: 0x02ff, Stride: 1},
			{Lo: 0x055a, Hi: 0x055c, Stride: 1},
			{Lo: 0x055e, Hi: 0x055e, Stride: 1},
			{Lo: 0x058a, Hi: 0x058a, Stride: 1},
			{Lo: 0x05f3, Hi: 0x05f3, Stride: 1},
			{Lo: 0x066b, Hi: 0x066b, Stride: 1},
			{Lo: 0x202f, Hi: 0x202f, Stride: 1},
			{Lo: 0x3005, Hi: 0x3005, Stride: 1},
			{Lo: 0x303b, Hi: 0x303b, Stride: 1},
			{Lo: 0xa708, Hi: 0xa716, Stride: 1},
			{Lo: 0xa720, Hi: 0xa721, Stride: 1},
			{Lo: 0xa789, Hi: 0xa78a, Stride: 1},
			{Lo: 0xab5b, Hi: 0xab5b, Stride: 1},
		},
		R32: []unicode.Range32{
			{Lo: 0x16fe0, Hi: 0x16fe1, Stride: 1},
			{Lo: 0x16fe3, Hi: 0x16fe3, Stride: 1},
			{Lo: 0x1f3fb, Hi: 0x1f3ff, Stride: 1},
		},
	}

	// wordBreakingFormats are the zero width space and the kana length and
	// repeat marks, which break words despite being Cf or Lm.
	wordBreakingFormats = &unicode.RangeTable{R16: []unicode.Range16{
		{Lo: 0x200b, Hi: 0x200b, Stride: 1},
		{Lo: 0x3006, Hi: 0x3006, Stride: 1},
		{Lo: 0x3031, Hi: 0x3035, Stride: 1},
		{Lo: 0x30fc, Hi: 0x30fc, Stride: 1},
		{Lo: 0xff70, Hi: 0xff70, Stride: 1},
	}}

	// wordMidRunes are the MidLetter, MidNumLet and Single_Quote word-break runes.
	wordMidRunes = &unicode.RangeTable{R16: []unicode.Range16{
		{Lo: 0x0027, Hi: 0x0027, Stride: 1},
		{Lo: 0x002e, Hi: 0x002e, Stride: 1},
		{Lo: 0x003a, Hi: 0x003a, Stride: 1},
		{Lo: 0x00b7, Hi: 0x00b7, Stride: 1},
		{Lo: 0x0387, Hi: 0x0387, Stride: 1},
		{Lo: 0x055f, Hi: 0x055f, Stride: 1},
		{Lo: 0x05f4, Hi: 0x05f4, Stride: 1},
		{Lo: 0x2018, Hi: 0x2019, Stride: 1},
		{Lo: 0x2024, Hi: 0x2024, Stride: 1},
		{Lo: 0x2027, Hi: 0x2027, Stride: 1},
		{Lo: 0xfe13, Hi: 0xfe13, Stride: 1},
		{Lo: 0xfe52, Hi: 0xfe52, Stride: 1},
		{Lo: 0xfe55, Hi: 0xfe55, Stride: 1},
		{Lo: 0xff07, Hi: 0xff07, Stride: 1},
		{Lo: 0xff0e, Hi: 0xff0e, Stride: 1},
		{Lo: 0xff1a, Hi: 0xff1a, Stride: 1},
	}}

	// fullTitleCase holds the unconditional SpecialCasing title mappings that
	// expand beyond unicode.ToTitle's single-rune result.
	fullTitleCase = map[rune]string{
		'\u00DF': "Ss",
		'\u0149': "ʼN",
		'\u01F0': "J̌",
		'\u0390': "Ϊ́",
		'\u03B0': "Ϋ́",
		'\u0587': "Եւ",
		'\u1E96': "H̱",
		'\u1E97': "T̈",
		'\u1E98': "W̊",
		'\u1E99': "Y̊",
		'\u1E9A': "Aʾ",
		'\u1F50': "Υ̓",
		'\u1F52': "Υ̓̀",
		'\u1F54': "Υ̓́",
		'\u1F56': "Υ̓͂",
		'\u1FB2': "Ὰͅ",
		'\u1FB4': "Άͅ",
		'\u1FB6': "Α͂",
		'\u1FB7': "ᾼ͂",
		'\u1FC2': "Ὴͅ",
		'\u1FC4': "Ήͅ",
		'\u1FC6': "Η͂",
		'\u1FC7': "ῌ͂",
		'\u1FD2': "Ϊ̀",
		'\u1FD3': "Ϊ́",
		'\u1FD6': "Ι͂",
		'\u1FD7': "Ϊ͂",
		'\u1FE2': "Ϋ̀",
		'\u1FE3': "Ϋ́",
		'\u1FE4': "Ρ̓",
		'\u1FE6': "Υ͂",
		'\u1FE7': "Ϋ͂",
		'\u1FF2': "Ὼͅ",
		'\u1FF4': "Ώͅ",
		'\u1FF6': "Ω͂",
		'\u1FF7': "ῼ͂",
		'\uFB00': "Ff",
		'\uFB01': "Fi",
		'\uFB02': "Fl",
		'\uFB03': "Ffi",
		'\uFB04': "Ffl",
		'\uFB05': "St",
		'\uFB06': "St",
		'\uFB13': "Մն",
		'\uFB14': "Մե",
		'\uFB15': "Մի",
		'\uFB16': "Վն",
		'\uFB17': "Մխ",
	}
)

// SplitCamelCase splits camelCase strings into words
func SplitCamelCase(s string) []string {
	var words []string
	var current strings.Builder
	runes := []rune(s)

	for i := 0; i < len(runes); i++ {
		r := runes[i]

		// Check if this rune starts a new word
		if i > 0 && r >= 'A' && r <= 'Z' {
			// Look back to see if previous character was lowercase
			prevIsLower := i > 0 && runes[i-1] >= 'a' && runes[i-1] <= 'z'

			// Only split on uppercase if previous was lowercase (simple camelCase like firstName, userID)
			// This keeps acronyms together (HTTPRequest stays as one word)
			if prevIsLower {
				if current.Len() > 0 {
					words = append(words, current.String())
					current.Reset()
				}
			}
		}

		current.WriteRune(r)
	}

	if current.Len() > 0 {
		words = append(words, current.String())
	}

	return words
}
