package clicky

import (
	"net/http/httptest"
	"testing"
)

func TestWithHttpRequest_QueryParam(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected string
	}{
		{"json query param", "http://example.com/api/users?format=json", "json"},
		{"yaml query param", "http://example.com/api/users?format=yaml", "yaml"},
		{"csv query param", "http://example.com/api/users?format=csv", "csv"},
		{"html query param", "http://example.com/api/users?format=html", "html"},
		{"markdown query param", "http://example.com/api/users?format=markdown", "markdown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.url, nil)
			opts := WithHttpRequest(req)
			if opts.Format != tt.expected {
				t.Errorf("WithHttpRequest() format = %v, want %v", opts.Format, tt.expected)
			}
		})
	}
}

func TestWithHttpRequest_PathExtension(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected string
	}{
		{"json extension", "http://example.com/api/users.json", "json"},
		{"yaml extension", "http://example.com/api/users.yaml", "yaml"},
		{"yml extension", "http://example.com/api/users.yml", "yaml"},
		{"csv extension", "http://example.com/api/users.csv", "csv"},
		{"html extension", "http://example.com/api/users.html", "html"},
		{"markdown extension", "http://example.com/api/users.md", "markdown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.url, nil)
			opts := WithHttpRequest(req)
			if opts.Format != tt.expected {
				t.Errorf("WithHttpRequest() format = %v, want %v", opts.Format, tt.expected)
			}
		})
	}
}

func TestWithHttpRequest_AcceptHeader(t *testing.T) {
	tests := []struct {
		name     string
		accept   string
		expected string
	}{
		{"json accept", "application/json", "json"},
		{"yaml accept", "application/yaml", "yaml"},
		{"text yaml accept", "text/yaml", "yaml"},
		{"csv accept", "text/csv", "csv"},
		{"html accept", "text/html", "html"},
		{"markdown accept", "text/markdown", "markdown"},
		{"plain text accept", "text/plain", "pretty"},
		{"wildcard accept", "*/*", "json"}, // Falls back to default
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "http://example.com/api/users", nil)
			req.Header.Set("Accept", tt.accept)
			opts := WithHttpRequest(req)
			if opts.Format != tt.expected {
				t.Errorf("WithHttpRequest() format = %v, want %v", opts.Format, tt.expected)
			}
		})
	}
}

func TestWithHttpRequest_Priority(t *testing.T) {
	// Query param should override path extension
	req := httptest.NewRequest("GET", "http://example.com/api/users.yaml?format=json", nil)
	req.Header.Set("Accept", "text/csv")
	opts := WithHttpRequest(req)
	if opts.Format != "json" {
		t.Errorf("Query param should have highest priority, got %v", opts.Format)
	}

	// Path extension should override Accept header
	req = httptest.NewRequest("GET", "http://example.com/api/users.csv", nil)
	req.Header.Set("Accept", "application/json")
	opts = WithHttpRequest(req)
	if opts.Format != "csv" {
		t.Errorf("Path extension should override Accept header, got %v", opts.Format)
	}

	// Accept header should be used when no query param or extension
	req = httptest.NewRequest("GET", "http://example.com/api/users", nil)
	req.Header.Set("Accept", "text/yaml")
	opts = WithHttpRequest(req)
	if opts.Format != "yaml" {
		t.Errorf("Accept header should be used, got %v", opts.Format)
	}
}

func TestWithHttpRequest_Paging(t *testing.T) {
	tests := []struct {
		name          string
		url           string
		headers       map[string]string
		expectedPage  int
		expectedLimit int
	}{
		{
			name:          "query params",
			url:           "http://example.com/api/users?page=2&limit=50",
			expectedPage:  2,
			expectedLimit: 50,
		},
		{
			name: "headers",
			url:  "http://example.com/api/users",
			headers: map[string]string{
				"X-Page":  "3",
				"X-Limit": "100",
			},
			expectedPage:  3,
			expectedLimit: 100,
		},
		{
			name: "query overrides headers",
			url:  "http://example.com/api/users?page=5",
			headers: map[string]string{
				"X-Page":  "1",
				"X-Limit": "20",
			},
			expectedPage:  5,
			expectedLimit: 20,
		},
		{
			name:          "no paging",
			url:           "http://example.com/api/users",
			expectedPage:  0,
			expectedLimit: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.url, nil)
			for key, val := range tt.headers {
				req.Header.Set(key, val)
			}
			opts := WithHttpRequest(req)
			if opts.Page != tt.expectedPage {
				t.Errorf("WithHttpRequest() page = %v, want %v", opts.Page, tt.expectedPage)
			}
			if opts.Limit != tt.expectedLimit {
				t.Errorf("WithHttpRequest() limit = %v, want %v", opts.Limit, tt.expectedLimit)
			}
		})
	}
}

func TestWithHttpRequest_DefaultFormat(t *testing.T) {
	// No format specified should default to json
	req := httptest.NewRequest("GET", "http://example.com/api/users", nil)
	opts := WithHttpRequest(req)
	if opts.Format != "json" {
		t.Errorf("Default format should be json, got %v", opts.Format)
	}
}

func TestFormatToContentType(t *testing.T) {
	tests := []struct {
		format   string
		expected string
	}{
		{"json", "application/json"},
		{"yaml", "application/yaml"},
		{"csv", "text/csv"},
		{"html", "text/html; charset=utf-8"},
		{"markdown", "text/markdown; charset=utf-8"},
		{"pdf", "application/pdf"},
		{"excel", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"},
		{"pretty", "text/plain; charset=utf-8"},
		{"unknown", "application/octet-stream"},
	}

	for _, tt := range tests {
		t.Run(tt.format, func(t *testing.T) {
			result := FormatToContentType(tt.format)
			if result != tt.expected {
				t.Errorf("FormatToContentType(%v) = %v, want %v", tt.format, result, tt.expected)
			}
		})
	}
}

func TestAcceptToFormat_MultipleValues(t *testing.T) {
	// Test multiple Accept values with quality
	req := httptest.NewRequest("GET", "http://example.com/api/users", nil)
	req.Header.Set("Accept", "text/html,application/json;q=0.9,*/*;q=0.8")
	opts := WithHttpRequest(req)
	// Should pick first recognized format (html)
	if opts.Format != "html" {
		t.Errorf("Should pick first format from Accept header, got %v", opts.Format)
	}
}
