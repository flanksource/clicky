package aichat

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// RuntimeSettings are app-owned chat defaults and limits evaluated for one
// request. Zero values leave the corresponding behavior unchanged.
type RuntimeSettings struct {
	// DefaultModel is used only when a request omits an explicit model.
	DefaultModel string `json:"defaultModel,omitempty"`
	// MaxInputTokens caps the estimated prompt/context size for a single turn.
	MaxInputTokens int `json:"maxInputTokens,omitempty"`
	// MonthlyTokenBudget caps the embedding app's current-month token usage.
	MonthlyTokenBudget int `json:"monthlyTokenBudget,omitempty"`
	// CurrentMonthTokens is the usage total compared with MonthlyTokenBudget.
	CurrentMonthTokens int `json:"currentMonthTokens,omitempty"`
	// MonthlyBudgetUSD caps the embedding app's current-month spend.
	MonthlyBudgetUSD float64 `json:"monthlyBudgetUsd,omitempty"`
	// CurrentMonthCostUSD is the spend total compared with MonthlyBudgetUSD.
	CurrentMonthCostUSD float64 `json:"currentMonthCostUsd,omitempty"`
}

type RuntimeSettingsProvider func(context.Context) (RuntimeSettings, error)

type runtimeSettingsError struct {
	status int
	msg    string
}

func (e runtimeSettingsError) Error() string { return e.msg }

func statusForRuntimeSettingsError(err error) int {
	if e, ok := err.(runtimeSettingsError); ok {
		return e.status
	}
	return http.StatusBadRequest
}

func modelIDForRequest(req ChatRequest, settings RuntimeSettings) string {
	if model := strings.TrimSpace(req.Model); model != "" {
		return model
	}
	return strings.TrimSpace(settings.DefaultModel)
}

func validateRequestConfig(req ChatRequest) error {
	if req.Temperature != nil && (*req.Temperature < 0 || *req.Temperature > 2) {
		return runtimeSettingsError{
			status: http.StatusBadRequest,
			msg:    fmt.Sprintf("invalid temperature %v (valid: 0.0-2.0)", *req.Temperature),
		}
	}
	if req.Budget.Cost < 0 {
		return runtimeSettingsError{
			status: http.StatusBadRequest,
			msg:    fmt.Sprintf("invalid budget cost %.4f (must be >= 0)", req.Budget.Cost),
		}
	}
	if req.Budget.MaxTokens < 0 {
		return runtimeSettingsError{
			status: http.StatusBadRequest,
			msg:    fmt.Sprintf("invalid budget maxTokens %d (must be >= 0)", req.Budget.MaxTokens),
		}
	}
	return nil
}

func enforceRuntimeSettings(req ChatRequest, settings RuntimeSettings) error {
	if settings.MonthlyBudgetUSD > 0 && settings.CurrentMonthCostUSD >= settings.MonthlyBudgetUSD {
		return runtimeSettingsError{
			status: http.StatusPaymentRequired,
			msg: fmt.Sprintf(
				"chat monthly cost budget exhausted: $%.4f used of $%.4f",
				settings.CurrentMonthCostUSD,
				settings.MonthlyBudgetUSD,
			),
		}
	}
	if settings.MonthlyTokenBudget > 0 && settings.CurrentMonthTokens >= settings.MonthlyTokenBudget {
		return runtimeSettingsError{
			status: http.StatusPaymentRequired,
			msg: fmt.Sprintf(
				"chat monthly token budget exhausted: %d used of %d",
				settings.CurrentMonthTokens,
				settings.MonthlyTokenBudget,
			),
		}
	}
	if settings.MaxInputTokens > 0 {
		estimated := estimateRequestTokens(req)
		if estimated > settings.MaxInputTokens {
			return runtimeSettingsError{
				status: http.StatusRequestEntityTooLarge,
				msg: fmt.Sprintf(
					"chat input is about %d tokens, exceeding the configured per-turn limit of %d",
					estimated,
					settings.MaxInputTokens,
				),
			}
		}
	}
	return nil
}

func estimateRequestTokens(req ChatRequest) int {
	payload := struct {
		Messages     []UIMessage       `json:"messages,omitempty"`
		Context      string            `json:"context,omitempty"`
		ContextItems []ChatContextItem `json:"contextItems,omitempty"`
	}{
		Messages:     req.Messages,
		Context:      req.Context,
		ContextItems: req.ContextItems,
	}
	raw, err := json.Marshal(payload)
	if err != nil || len(raw) == 0 {
		return 0
	}
	return (len(raw) + 3) / 4
}
