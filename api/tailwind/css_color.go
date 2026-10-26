package tailwind

import (
	"fmt"
	"regexp"
	"strings"
)

// semanticColors maps theme tokens (shadcn / clicky-ui vocabulary) onto real
// colours. An empty value means "inherit": the token names the surrounding
// default, so no colour is emitted.
var semanticColors = map[string]string{
	"muted-foreground": "#6b7280", // gray-500
	"foreground":       "",
	"background":       "",
}

var hexColorRegexp = regexp.MustCompile(`^#(?:[0-9a-fA-F]{3,4}|[0-9a-fA-F]{6}|[0-9a-fA-F]{8})$`)

// cssNamedColors is the CSS Color Module Level 4 named-colour keyword list.
var cssNamedColors = map[string]bool{}

func init() {
	for _, name := range strings.Fields(`aliceblue antiquewhite aqua aquamarine azure beige bisque
		blanchedalmond blueviolet brown burlywood cadetblue chartreuse chocolate coral cornflowerblue
		cornsilk crimson darkblue darkcyan darkgoldenrod darkgray darkgreen darkgrey darkkhaki
		darkmagenta darkolivegreen darkorange darkorchid darkred darksalmon darkseagreen darkslateblue
		darkslategray darkslategrey darkturquoise darkviolet deeppink deepskyblue dimgray dimgrey
		dodgerblue firebrick floralwhite forestgreen gainsboro ghostwhite gold goldenrod greenyellow
		grey honeydew hotpink indianred ivory khaki lavender lavenderblush lawngreen lemonchiffon
		lightblue lightcoral lightcyan lightgoldenrodyellow lightgray lightgreen lightgrey lightpink
		lightsalmon lightseagreen lightskyblue lightslategray lightslategrey lightsteelblue lightyellow
		limegreen linen magenta maroon mediumaquamarine mediumblue mediumorchid mediumpurple
		mediumseagreen mediumslateblue mediumspringgreen mediumturquoise mediumvioletred midnightblue
		mintcream mistyrose moccasin navajowhite navy oldlace olive olivedrab orangered orchid
		palegoldenrod palegreen paleturquoise palevioletred papayawhip peachpuff peru plum powderblue
		rebeccapurple saddlebrown salmon sandybrown seagreen seashell sienna silver skyblue slateblue
		slategray slategrey snow springgreen steelblue tan thistle tomato turquoise wheat whitesmoke
		yellowgreen`) {
		cssNamedColors[name] = true
	}
}

// resolveColorName maps a prefix-stripped colour token (e.g. "red-500",
// "muted-foreground", "[#abc]", "navy") onto a CSS colour. Tokens that name
// no colour are an error, so callers never emit them as CSS values.
func resolveColorName(name string) (string, error) {
	if strings.HasPrefix(name, "[") && strings.HasSuffix(name, "]") {
		name = strings.TrimSuffix(strings.TrimPrefix(name, "["), "]")
	}
	if color, ok := TailwindSpecialColors[name]; ok {
		return color, nil
	}
	if color, ok := semanticColors[name]; ok {
		return color, nil
	}
	base, shade, hasShade := strings.Cut(name, "-")
	if palette, ok := TailwindColors[base]; ok {
		if !hasShade {
			shade = "500"
		}
		if color, ok := palette[shade]; ok {
			return color, nil
		}
		return "", fmt.Errorf("invalid shade '%s' for color '%s'", shade, base)
	}
	if isCSSColorLiteral(name) {
		return name, nil
	}
	return "", fmt.Errorf("unknown color %q", name)
}

func isCSSColorLiteral(name string) bool {
	if hexColorRegexp.MatchString(name) || cssNamedColors[strings.ToLower(name)] {
		return true
	}
	for _, fn := range []string{"rgb(", "rgba(", "hsl(", "hsla(", "oklch(", "var("} {
		if strings.HasPrefix(name, fn) && strings.HasSuffix(name, ")") {
			return true
		}
	}
	return false
}

func stripColorPrefix(colorClass string) string {
	for _, prefix := range []string{"bg-", "text-", "border-", "ring-", "decoration-", "divide-", "outline-", "fill-", "stroke-"} {
		if strings.HasPrefix(colorClass, prefix) {
			return strings.TrimPrefix(colorClass, prefix)
		}
	}
	return colorClass
}
