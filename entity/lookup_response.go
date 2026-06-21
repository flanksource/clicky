package entity

import "github.com/flanksource/clicky/api"

type entityLookupResponse struct {
	Filters map[string]entityLookupFilter `json:"filters"`
}

type entityLookupFilter struct {
	Label    string                `json:"label,omitempty"`
	Options  map[string]clickyNode `json:"options,omitempty"`
	Selected map[string]clickyNode `json:"selected,omitempty"`
	Multi    bool                  `json:"multi,omitempty"`
	Type     string                `json:"type,omitempty"`
	// Truncated is true when more options exist than are returned in Options:
	// the filter is a SearchableFilter whose distinct count exceeds the option
	// cap. The UI renders the returned head set plus an "… and N more" hint and
	// re-queries via OptionsWithQuery as the user types. Total is the true
	// distinct count behind the head. Both stay zero/false for non-searchable
	// filters whose Options fully enumerate.
	Truncated bool `json:"truncated,omitempty"`
	Total     int  `json:"total,omitempty"`
}

type clickyNode struct {
	Kind     string       `json:"kind"`
	Plain    string       `json:"plain,omitempty"`
	Text     string       `json:"text,omitempty"`
	Style    *clickyStyle `json:"style,omitempty"`
	Children []clickyNode `json:"children,omitempty"`
	Tooltip  *clickyNode  `json:"tooltip,omitempty"`
}

type clickyStyle struct {
	ClassName string `json:"className,omitempty"`
}

func toClickyNodeMap(values map[string]api.Textable) map[string]clickyNode {
	if len(values) == 0 {
		return nil
	}

	nodes := make(map[string]clickyNode, len(values))
	for key, value := range values {
		nodes[key] = toClickyNode(value)
	}
	return nodes
}

func toClickyNode(value api.Textable) clickyNode {
	switch typed := value.(type) {
	case api.Text:
		return textToClickyNode(typed)
	case *api.Text:
		if typed == nil {
			return clickyNode{Kind: "text"}
		}
		return textToClickyNode(*typed)
	default:
		if value == nil {
			return clickyNode{Kind: "text"}
		}
		plain := value.String()
		return clickyNode{
			Kind:  "text",
			Text:  plain,
			Plain: plain,
		}
	}
}

func textToClickyNode(text api.Text) clickyNode {
	node := clickyNode{
		Kind:  "text",
		Text:  text.Content,
		Plain: text.String(),
	}

	if text.Style != "" {
		node.Style = &clickyStyle{ClassName: text.Style}
	}

	if len(text.Children) > 0 {
		node.Children = make([]clickyNode, 0, len(text.Children))
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
