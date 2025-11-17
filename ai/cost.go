package ai

import (
	"fmt"

	"github.com/flanksource/clicky/api"
)

type Costs []Cost

type Cost struct {
	Model        string  `json:"model,omitempty"`
	InputTokens  int     `json:"input_tokens"`
	OutputTokens int     `json:"output_tokens"`
	TotalTokens  int     `json:"total_tokens"`
	InputCost    float64 `json:"input_cost"`
	OutputCost   float64 `json:"output_cost"`
}

func (c Cost) IsEmpty() bool {
	return c.InputTokens == 0 && c.OutputTokens == 0 && c.InputCost == 0 && c.OutputCost == 0
}

func (c Cost) Total() float64 {
	return c.InputCost + c.OutputCost
}

func (c Cost) Pretty() api.Text {
	t := api.Text{}
	if c.Model != "" {
		t = t.Append(c.Model, "font-mono").Space()
	}
	if c.Total() > 0 {
		t = t.Append(fmt.Sprintf("$%.4f", c.Total()))
	}

	if c.InputTokens+c.OutputTokens > 0 {
		t = t.Space().Append(fmt.Sprintf("(%v in, %v out)", api.Human(c.InputTokens), api.Human(c.OutputTokens)), "text-muted")
	}
	return t
}

func (c Costs) AggregateByModel() Costs {
	modelCosts := make(map[string]Cost)
	for _, cost := range c {
		if cost.IsEmpty() {
			continue
		}
		model := cost.Model
		if model == "" {
			model = "unknown"
		}

		existing, ok := modelCosts[model]
		if !ok {
			existing = Cost{Model: model}
		}

		modelCosts[model] = existing.Add(cost)
	}

	aggregated := Costs{}
	for _, cost := range modelCosts {
		aggregated = append(aggregated, cost)
	}

	return aggregated
}

func (c Costs) Sum() Cost {
	total := Cost{}
	for _, cost := range c {
		total = total.Add(cost)
	}
	return total
}

func (c Costs) GetCostsByModel() map[string]Cost {
	modelCosts := make(map[string]Cost)
	for _, cost := range c {
		model := cost.Model
		if model == "" {
			model = "unknown"
		}

		existing := modelCosts[model]
		existing.Model = model
		existing.InputTokens += cost.InputTokens
		existing.OutputTokens += cost.OutputTokens
		existing.TotalTokens += cost.TotalTokens
		existing.InputCost += cost.InputCost
		existing.OutputCost += cost.OutputCost
		modelCosts[model] = existing
	}
	return modelCosts
}

func (c Costs) Pretty() api.Text {

	modelCosts := c.GetCostsByModel()

	t := api.Text{}
	t = t.Append("Session Costs", "font-bold").NewLine()

	// Display each model's costs
	for _, cost := range modelCosts {
		t = t.Append("  ").Add(cost.Pretty()).NewLine()
	}

	if len(modelCosts) > 1 {
		// Display total
		t = t.Append("  Total: ", "font-bold").Add(c.Sum().Pretty())
	}
	return t

}

func (c Cost) TotalCost() float64 {
	return c.InputCost + c.OutputCost
}
