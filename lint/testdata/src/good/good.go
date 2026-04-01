package good

import "github.com/flanksource/clicky/api"

// Rule 1 exception: empty literal for chaining
var chained = api.Text{}.Append("hello", "font-bold")

// Rule 1 exception: empty literal as zero value
var empty = api.Text{}

// Rule 2 exception: Pretty method
type MyType struct{}

func (m MyType) Pretty() api.Text {
	return api.Text{}.Append("hello")
}

// Rule 2 exception: PrettyFull method
func (m MyType) PrettyFull() api.Textable {
	return api.Text{}.Append("hello")
}

// Rule 2 exception: PrettyRow method
func (m MyType) PrettyRow(opts interface{}) map[string]api.Text {
	return nil
}

// Using builder pattern - fine
var built = api.NewText("hello").Bold().Build()

// Using helper functions - fine
var success = api.SuccessText("done")

// Fluent chaining - fine
var fluent = api.Text{}.Append("label", "font-bold").Space().Append("value")
