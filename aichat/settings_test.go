package aichat

import (
	"errors"
	"net/http"
	"strings"
	"testing"
)

func TestModelIDForRequestUsesRuntimeDefaultOnlyWhenRequestOmitsModel(t *testing.T) {
	settings := RuntimeSettings{DefaultModel: "openai/o4-mini"}
	if got := modelIDForRequest(ChatRequest{}, settings); got != "openai/o4-mini" {
		t.Fatalf("modelIDForRequest(empty) = %q", got)
	}
	if got := modelIDForRequest(ChatRequest{Model: "googleai/gemini-2.5-flash"}, settings); got != "googleai/gemini-2.5-flash" {
		t.Fatalf("modelIDForRequest(explicit) = %q", got)
	}
	if got := modelIDForRequest(ChatRequest{Model: "  openai/o4-mini  "}, settings); got != "openai/o4-mini" {
		t.Fatalf("modelIDForRequest(padded) = %q, want trimmed", got)
	}
}

func TestEnforceRuntimeSettingsRejectsBudgetExhaustion(t *testing.T) {
	err := enforceRuntimeSettings(ChatRequest{}, RuntimeSettings{
		MonthlyBudgetUSD:    10,
		CurrentMonthCostUSD: 10,
	})
	if err == nil {
		t.Fatal("expected budget error")
	}
	if statusForRuntimeSettingsError(err) != http.StatusPaymentRequired {
		t.Fatalf("status = %d", statusForRuntimeSettingsError(err))
	}
	if !strings.Contains(err.Error(), "cost budget exhausted") {
		t.Fatalf("error = %v", err)
	}

	err = enforceRuntimeSettings(ChatRequest{}, RuntimeSettings{
		MonthlyTokenBudget: 10_000,
		CurrentMonthTokens: 10_000,
	})
	if err == nil || !strings.Contains(err.Error(), "token budget exhausted") {
		t.Fatalf("token budget error = %v", err)
	}
}

func TestEnforceRuntimeSettingsRejectsOversizedInput(t *testing.T) {
	err := enforceRuntimeSettings(ChatRequest{
		Messages: []UIMessage{{
			Role:  "user",
			Parts: []UIPart{{Type: "text", Text: strings.Repeat("x", 200)}},
		}},
	}, RuntimeSettings{MaxInputTokens: 10})
	if err == nil {
		t.Fatal("expected max input tokens error")
	}
	if statusForRuntimeSettingsError(err) != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d", statusForRuntimeSettingsError(err))
	}
}

func TestStatusForRuntimeSettingsErrorDefaultsToBadRequest(t *testing.T) {
	if got := statusForRuntimeSettingsError(errors.New("other")); got != http.StatusBadRequest {
		t.Fatalf("status = %d", got)
	}
}
