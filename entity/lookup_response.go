package entity

import "github.com/flanksource/clicky/api"

// LookupResponse is what an operation's lookup func returns: the option sets of
// its filters, keyed by filter key. It is the `__lookup=filters` wire shape
// clicky-ui reads, and what terminal forms (schemaform) resolve lookups from.
type LookupResponse struct {
	Filters map[string]LookupFilter `json:"filters"`
}

// LookupFilter is one filter's option set within a LookupResponse.
type LookupFilter struct {
	Label   string                `json:"label,omitempty"`
	Options map[string]LookupNode `json:"options,omitempty"`
	// Counts maps an option value — the same keys as Options — to the number of
	// rows holding it. It is omitted when the filter's source does not count, and
	// a value it has no count for is absent rather than zero.
	Counts          map[string]int        `json:"counts,omitempty"`
	Selected        map[string]LookupNode `json:"selected,omitempty"`
	Multi           bool                  `json:"multi,omitempty"`
	Type            string                `json:"type,omitempty"`
	Unit            string                `json:"unit,omitempty"`
	DefaultOperator string                `json:"defaultOperator,omitempty"`
	// TimeEnabled offers a clock on a range control. Absent leaves the browser to
	// decide from Type alone, which is what every control that is not a range does.
	TimeEnabled *bool `json:"timeEnabled,omitempty"`
	// Truncated is true when more options exist than are returned in Options:
	// the filter is a SearchableFilter whose distinct count exceeds the option
	// cap. The UI renders the returned head set plus an "… and N more" hint and
	// re-queries via OptionsWithQuery as the user types. Total is the true
	// distinct count behind the head. Both stay zero/false for non-searchable
	// filters whose Options fully enumerate.
	Truncated bool `json:"truncated,omitempty"`
	Total     int  `json:"total,omitempty"`
}

// LookupNode is the serialized api.Textable label of a lookup option; Plain is
// its unstyled text.
type LookupNode struct {
	Kind     string       `json:"kind"`
	Plain    string       `json:"plain,omitempty"`
	Text     string       `json:"text,omitempty"`
	Style    *LookupStyle `json:"style,omitempty"`
	Children []LookupNode `json:"children,omitempty"`
	Tooltip  *LookupNode  `json:"tooltip,omitempty"`
}

// LookupStyle carries a LookupNode's Tailwind class string.
type LookupStyle struct {
	ClassName string `json:"className,omitempty"`
}

func toClickyNodeMap(values map[string]api.Textable) map[string]LookupNode {
	if len(values) == 0 {
		return nil
	}

	nodes := make(map[string]LookupNode, len(values))
	for key, value := range values {
		nodes[key] = toClickyNode(value)
	}
	return nodes
}

func toClickyNode(value api.Textable) LookupNode {
	switch typed := value.(type) {
	case api.Text:
		return textToClickyNode(typed)
	case *api.Text:
		if typed == nil {
			return LookupNode{Kind: "text"}
		}
		return textToClickyNode(*typed)
	default:
		if value == nil {
			return LookupNode{Kind: "text"}
		}
		plain := value.String()
		return LookupNode{
			Kind:  "text",
			Text:  plain,
			Plain: plain,
		}
	}
}

func textToClickyNode(text api.Text) LookupNode {
	node := LookupNode{
		Kind:  "text",
		Text:  text.Content,
		Plain: text.String(),
	}

	if text.Style != "" {
		node.Style = &LookupStyle{ClassName: text.Style}
	}

	if len(text.Children) > 0 {
		node.Children = make([]LookupNode, 0, len(text.Children))
		for _, child := range text.Children {
			node.Children = append(node.Children, toClickyNode(child))
		}
	}

	if text.Tooltip != nil {
		tooltip := toClickyNode(text.Tooltip)
		node.Tooltip = &tooltip
	}

	if node.Text == "" && node.Plain != "" {
		node.Text = node.Plain
	}

	return node
}
