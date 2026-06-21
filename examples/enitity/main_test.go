package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestServeMarkdownPreviewFormats(t *testing.T) {
	source := "# Rollout\n\n| Stack | Status |\n| ----- | ------ |\n| api | healthy |\n"

	tests := []struct {
		name        string
		format      string
		contentType string
		contains    string
	}{
		{
			name:        "clicky json",
			format:      "clicky-json",
			contentType: "application/json+clicky",
			contains:    `"kind": "heading"`,
		},
		{
			name:        "markdown",
			format:      "markdown",
			contentType: "text/markdown; charset=utf-8",
			contains:    "| Stack | Status |",
		},
		{
			name:        "csv",
			format:      "csv",
			contentType: "text/csv",
			contains:    "heading",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/examples/markdown-preview?format="+tt.format, strings.NewReader(source))
			rec := httptest.NewRecorder()

			serveMarkdownPreview(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
			}
			if got := rec.Header().Get("Content-Type"); got != tt.contentType {
				t.Fatalf("expected content type %q, got %q", tt.contentType, got)
			}
			if body := rec.Body.String(); !strings.Contains(body, tt.contains) {
				t.Fatalf("expected response to contain %q, got:\n%s", tt.contains, body)
			}
		})
	}
}

func TestServeMarkdownPreviewExcel(t *testing.T) {
	source := "# Rollout\n\n| Stack | Status |\n| ----- | ------ |\n| api | healthy |\n"
	req := httptest.NewRequest(http.MethodPost, "/api/examples/markdown-preview?format=excel", strings.NewReader(source))
	rec := httptest.NewRecorder()

	serveMarkdownPreview(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" {
		t.Fatalf("expected excel content type, got %q", got)
	}
	if body := rec.Body.Bytes(); len(body) < 4 || string(body[:2]) != "PK" {
		t.Fatalf("expected xlsx zip payload, got %d bytes", len(body))
	}
}

func TestServeMarkdownPreviewJsonBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/examples/markdown-preview?format=json", strings.NewReader("# Rollout\n"))
	rec := httptest.NewRecorder()

	serveMarkdownPreview(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected JSON payload: %v\n%s", err, rec.Body.String())
	}
	if payload["version"] == nil || payload["root"] == nil {
		t.Fatalf("expected markdown document JSON, got %#v", payload)
	}
}
