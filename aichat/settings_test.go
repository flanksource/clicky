package aichat

import (
	"context"
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

func TestValidateRequestConfigRejectsInvalidBudgetAndTemperature(t *testing.T) {
	temp := 2.1
	err := validateRequestConfig(ChatRequest{Temperature: &temp})
	if err == nil || !strings.Contains(err.Error(), "temperature") {
		t.Fatalf("temperature error = %v", err)
	}
	if statusForRuntimeSettingsError(err) != http.StatusBadRequest {
		t.Fatalf("temperature status = %d", statusForRuntimeSettingsError(err))
	}

	err = validateRequestConfig(ChatRequest{Budget: ChatBudget{Cost: -0.1}})
	if err == nil || !strings.Contains(err.Error(), "budget cost") {
		t.Fatalf("budget cost error = %v", err)
	}

	err = validateRequestConfig(ChatRequest{Budget: ChatBudget{MaxTokens: -1}})
	if err == nil || !strings.Contains(err.Error(), "maxTokens") {
		t.Fatalf("maxTokens error = %v", err)
	}
}

func TestRequestBudgetRejectsExhaustedThread(t *testing.T) {
	ctx := context.Background()
	store := NewMemThreadStore()
	thread, err := store.Create(ctx, "t")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := store.AddUsage(ctx, thread.ID, TurnUsage{CostUSD: 1.25}); err != nil {
		t.Fatalf("AddUsage: %v", err)
	}
	s := NewServer(Options{Threads: store})
	defer s.Close()

	err = s.enforceRequestBudget(ctx, ChatRequest{ThreadID: thread.ID, Budget: ChatBudget{Cost: 1}})
	if err == nil || !strings.Contains(err.Error(), "cost budget exhausted") {
		t.Fatalf("budget error = %v", err)
	}
	if statusForRuntimeSettingsError(err) != http.StatusPaymentRequired {
		t.Fatalf("status = %d", statusForRuntimeSettingsError(err))
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
