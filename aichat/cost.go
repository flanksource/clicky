package aichat

import (
	"strings"

	"github.com/firebase/genkit/go/ai"
	capapi "github.com/flanksource/captain/pkg/api"
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

type usageBreakdown struct {
	InputTokens      int `json:"inputTokens"`
	OutputTokens     int `json:"outputTokens"`
	ReasoningTokens  int `json:"reasoningTokens"`
	CacheReadTokens  int `json:"cacheReadTokens"`
	CacheWriteTokens int `json:"cacheWriteTokens"`
	TotalTokens      int `json:"totalTokens"`
}

type costBreakdown struct {
	Model         string  `json:"model,omitempty"`
	InputUsd      float64 `json:"inputUsd"`
	OutputUsd     float64 `json:"outputUsd"`
	ReasoningUsd  float64 `json:"reasoningUsd"`
	CacheReadUsd  float64 `json:"cacheReadUsd"`
	CacheWriteUsd float64 `json:"cacheWriteUsd"`
	TotalUsd      float64 `json:"totalUsd"`
}

func (u usageBreakdown) withTotal() usageBreakdown {
	u.TotalTokens = u.InputTokens + u.OutputTokens + u.ReasoningTokens + u.CacheReadTokens + u.CacheWriteTokens
	return u
}

// genkitUsageBreakdown maps Genkit usage onto the canonical token buckets.
// Thoughts stay in reasoning tokens; Genkit exposes cache reads but not writes.
func genkitUsageBreakdown(u *ai.GenerationUsage) usageBreakdown {
	if u == nil {
		return usageBreakdown{}
	}
	return usageBreakdown{
		InputTokens:     u.InputTokens,
		OutputTokens:    u.OutputTokens,
		ReasoningTokens: u.ThoughtsTokens,
		CacheReadTokens: u.CachedContentTokens,
	}.withTotal()
}

func captainUsageBreakdown(u *capapi.Usage) usageBreakdown {
	if u == nil {
		return usageBreakdown{}
	}
	return usageBreakdown{
		InputTokens:      u.InputTokens,
		OutputTokens:     u.OutputTokens,
		ReasoningTokens:  u.ReasoningTokens,
		CacheReadTokens:  u.CacheReadTokens,
		CacheWriteTokens: u.CacheWriteTokens,
	}.withTotal()
}

// costUSD prices one generation. Anthropic models go through captain's pricing
// table (the source of truth for Claude); other providers use extraPricing with
// captain's same arithmetic. An unpriced model returns 0 rather than guessing.
func costUSD(modelID string, u *ai.GenerationUsage) float64 {
	return costForUsage(modelID, genkitUsageBreakdown(u)).TotalUsd
}

func costForUsage(modelID string, u usageBreakdown) costBreakdown {
	p, ok := pricingForModel(modelID)
	if !ok {
		return costBreakdown{Model: modelID}
	}
	breakdown := costBreakdown{
		Model:         modelID,
		InputUsd:      float64(u.InputTokens) * p.InputPerMTok / 1e6,
		OutputUsd:     float64(u.OutputTokens) * p.OutputPerMTok / 1e6,
		ReasoningUsd:  float64(u.ReasoningTokens) * p.OutputPerMTok / 1e6,
		CacheReadUsd:  float64(u.CacheReadTokens) * p.CacheReadPerMTok / 1e6,
		CacheWriteUsd: float64(u.CacheWriteTokens) * p.CacheWritePerMTok / 1e6,
	}
	breakdown.TotalUsd = breakdown.InputUsd + breakdown.OutputUsd + breakdown.ReasoningUsd + breakdown.CacheReadUsd + breakdown.CacheWriteUsd
	return breakdown
}

func pricingForModel(modelID string) (claude.ModelPricing, bool) {
	if family := claude.ClassifyModel(modelID); family != claude.ModelFamilyUnknown {
		return claude.PricingTable[family], true
	}
	if p, ok := extraPricing[modelID]; ok {
		return p, true
	}
	if strings.HasPrefix(modelID, "google/") {
		p, ok := extraPricing["googleai/"+strings.TrimPrefix(modelID, "google/")]
		return p, ok
	}
	return claude.ModelPricing{}, false
}
