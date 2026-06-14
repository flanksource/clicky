package aichat

import (
	"github.com/firebase/genkit/go/ai"
	"github.com/flanksource/captain/pkg/claude"
)

// extraPricing supplements captain's Anthropic-only PricingTable with rates for
// the non-Anthropic models in this catalog. captain.CalculateCost prices the
// Claude models (opus/sonnet/haiku); these rows cover OpenAI + Google. Rates are
// USD per million tokens — keep in sync with the providers' pricing pages.
var extraPricing = map[string]claude.ModelPricing{
	"openai/gpt-4o":             {InputPerMTok: 2.5, OutputPerMTok: 10},
	"openai/o3":                 {InputPerMTok: 2, OutputPerMTok: 8},
	"openai/o4-mini":            {InputPerMTok: 1.1, OutputPerMTok: 4.4},
	"googleai/gemini-2.5-pro":   {InputPerMTok: 1.25, OutputPerMTok: 10, CacheReadPerMTok: 0.31},
	"googleai/gemini-2.5-flash": {InputPerMTok: 0.3, OutputPerMTok: 2.5, CacheReadPerMTok: 0.075},
}

// genkitToCaptainUsage maps a Genkit GenerationUsage onto captain's Usage shape.
// Reasoning ("thoughts") tokens are folded into output; Genkit reports cache
// reads via CachedContentTokens and does not expose cache writes.
func genkitToCaptainUsage(u *ai.GenerationUsage) *claude.Usage {
	return &claude.Usage{
		InputTokens:          u.InputTokens,
		OutputTokens:         u.OutputTokens + u.ThoughtsTokens,
		CacheReadInputTokens: u.CachedContentTokens,
	}
}

// costUSD prices one generation. Anthropic models go through captain's pricing
// table (the source of truth for Claude); other providers use extraPricing with
// captain's same arithmetic. An unpriced model returns 0 rather than guessing.
func costUSD(modelID string, u *ai.GenerationUsage) float64 {
	if u == nil {
		return 0
	}
	cu := genkitToCaptainUsage(u)
	if claude.ClassifyModel(modelID) != claude.ModelFamilyUnknown {
		return claude.CalculateCost(cu, modelID)
	}
	p, ok := extraPricing[modelID]
	if !ok {
		return 0
	}
	return float64(cu.InputTokens)*p.InputPerMTok/1e6 +
		float64(cu.OutputTokens)*p.OutputPerMTok/1e6 +
		float64(cu.CacheReadInputTokens)*p.CacheReadPerMTok/1e6
}
