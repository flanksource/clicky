package aichat

import (
	"math"
	"testing"

	"github.com/firebase/genkit/go/ai"
)

func TestCostUSD(t *testing.T) {
	const oneM = 1_000_000
	cases := []struct {
		name  string
		model string
		usage *ai.GenerationUsage
		want  float64 // independently computed from published per-MTok rates
	}{
		{
			// captain Sonnet-4 rates: input $3, output $15 per MTok.
			name:  "anthropic sonnet via captain",
			model: "anthropic/claude-sonnet-4-5",
			usage: &ai.GenerationUsage{InputTokens: oneM, OutputTokens: oneM},
			want:  3 + 15,
		},
		{
			// captain Opus-4 rates: input $15, output $75 per MTok.
			name:  "anthropic opus via captain",
			model: "anthropic/claude-opus-4-1",
			usage: &ai.GenerationUsage{InputTokens: oneM, OutputTokens: oneM},
			want:  15 + 75,
		},
		{
			// clicky extraPricing gpt-4o: input $2.5, output $10 per MTok.
			name:  "openai gpt-4o via extraPricing",
			model: "openai/gpt-4o",
			usage: &ai.GenerationUsage{InputTokens: oneM, OutputTokens: oneM},
			want:  2.5 + 10,
		},
		{
			// gemini flash: input $0.3, cache-read $0.075 per MTok; 2M input + 1M cached.
			name:  "gemini flash with cached input",
			model: "googleai/gemini-2.5-flash",
			usage: &ai.GenerationUsage{InputTokens: 2 * oneM, CachedContentTokens: oneM},
			want:  2*0.3 + 0.075,
		},
		{
			// reasoning tokens are billed at the output rate (gpt-4o output $10).
			name:  "thoughts tokens billed as output",
			model: "openai/gpt-4o",
			usage: &ai.GenerationUsage{OutputTokens: oneM, ThoughtsTokens: oneM},
			want:  2 * 10,
		},
		{
			name:  "unpriced model returns zero",
			model: "openai/unknown-model",
			usage: &ai.GenerationUsage{InputTokens: oneM, OutputTokens: oneM},
			want:  0,
		},
		{
			name:  "nil usage returns zero",
			model: "anthropic/claude-sonnet-4-5",
			usage: nil,
			want:  0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := costUSD(tc.model, tc.usage)
			if math.Abs(got-tc.want) > 1e-9 {
				t.Fatalf("costUSD(%q) = %v, want %v", tc.model, got, tc.want)
			}
		})
	}
}
