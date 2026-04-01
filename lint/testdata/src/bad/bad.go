package bad

import (
	"fmt"

	"github.com/flanksource/clicky/api"
)

// Rule 1: direct struct literal with fields
var directLiteral = api.Text{Content: "hello", Style: "font-bold"} // want `avoid direct api\.Text struct literal`

// Rule 1: single field still counts
var singleField = api.Text{Content: "hello"} // want `avoid direct api\.Text struct literal`

// Rule 2: non-Pretty function returning api.Text
func MakeLabel() api.Text { // want `MakeLabel returns api\.Text`
	return api.Text{}
}

// Rule 2: method not named Pretty
type Foo struct{}

func (f Foo) Render() api.Text { // want `Render returns api\.Text`
	return api.Text{}
}

// Rule 3: string concatenation in Content (also triggers Rule 1)
var concatContent = api.Text{Content: "hello" + " world"} // want `avoid direct api\.Text struct literal` `avoid string concatenation in Content field`

// Rule 3: fmt.Sprintf in Content (also triggers Rule 1)
var sprintfContent = api.Text{Content: fmt.Sprintf("hello %s", "world")} // want `avoid direct api\.Text struct literal` `avoid fmt\.Sprintf in Content field`

// Rule 4: Children slice literal with api.Text elements
var childrenLiteral = api.Text{ // want `avoid direct api\.Text struct literal`
	Children: []api.Textable{api.Text{Content: "a"}, api.Text{Content: "b"}}, // want `avoid Children slice literal` `avoid direct api\.Text struct literal` `avoid direct api\.Text struct literal`
}
