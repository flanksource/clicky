package aichat

import (
	"context"
	"testing"
)

func TestMemThreadStoreAddUsageAccumulates(t *testing.T) {
	ctx := context.Background()
	store := NewMemThreadStore()
	th, err := store.Create(ctx, "t")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if _, err := store.AddUsage(ctx, th.ID, TurnUsage{InputTokens: 100, OutputTokens: 40, ReasoningTokens: 10, CacheReadTokens: 5, CostUSD: 0.5}); err != nil {
		t.Fatalf("AddUsage 1: %v", err)
	}
	got, err := store.AddUsage(ctx, th.ID, TurnUsage{InputTokens: 250, OutputTokens: 60, ReasoningTokens: 20, CacheReadTokens: 7, CacheWriteTokens: 3, CostUSD: 1.25})
	if err != nil {
		t.Fatalf("AddUsage 2: %v", err)
	}

	if got.TotalInputTokens != 350 || got.TotalOutputTokens != 100 {
		t.Errorf("tokens = %d in / %d out, want 350 / 100", got.TotalInputTokens, got.TotalOutputTokens)
	}
	if got.TotalReasoningTokens != 30 || got.TotalCacheReadTokens != 12 || got.TotalCacheWriteTokens != 3 {
		t.Errorf("extra tokens = reasoning %d cache read %d cache write %d, want 30 / 12 / 3",
			got.TotalReasoningTokens, got.TotalCacheReadTokens, got.TotalCacheWriteTokens)
	}
	if got.TotalCostUsd != 1.75 {
		t.Errorf("TotalCostUsd = %v, want 1.75", got.TotalCostUsd)
	}
	// LastContextTokens reflects only the most recent turn's input.
	if got.LastContextTokens != 250 {
		t.Errorf("LastContextTokens = %d, want 250", got.LastContextTokens)
	}
}

func TestMemThreadStoreAddUsageUnknownThread(t *testing.T) {
	if _, err := NewMemThreadStore().AddUsage(context.Background(), "missing", TurnUsage{}); err == nil {
		t.Fatal("expected error for unknown thread, got nil")
	}
}
