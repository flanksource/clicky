package main

import (
	"testing"
	"time"
)

func TestMatchingStacksLockedHonorsFromAndTo(t *testing.T) {
	store := newDemoStore()

	items := store.matchingStacksLocked(stackListOpts{
		stackWindowOpts: stackWindowOpts{
			From: time.Now().Add(-24 * time.Hour).UTC(),
			To:   time.Now().UTC(),
		},
	}, false)

	if len(items) != 1 {
		t.Fatalf("expected exactly one stack in the last 24h window, got %d", len(items))
	}

	if items[0].ID != "stk-001" {
		t.Fatalf("expected stk-001 in the last 24h window, got %s", items[0].ID)
	}
}
