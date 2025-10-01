package formatters

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type testUser struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

func TestFormatHandler_JSON(t *testing.T) {
	handler := FormatHandler(func(r *http.Request) (any, error) {
		return testUser{Name: "Alice", Email: "alice@example.com"}, nil
	})

	req := httptest.NewRequest("GET", "http://example.com/users?format=json", nil)
	w := httptest.NewRecorder()

	handler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %s", contentType)
	}

	var user testUser
	if err := json.Unmarshal(w.Body.Bytes(), &user); err != nil {
		t.Errorf("Failed to parse JSON response: %v", err)
	}

	if user.Name != "Alice" {
		t.Errorf("Expected name Alice, got %s", user.Name)
	}
}

func TestFormatHandler_YAML(t *testing.T) {
	handler := FormatHandler(func(r *http.Request) (any, error) {
		return testUser{Name: "Bob", Email: "bob@example.com"}, nil
	})

	req := httptest.NewRequest("GET", "http://example.com/users.yaml", nil)
	w := httptest.NewRecorder()

	handler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/yaml" {
		t.Errorf("Expected Content-Type application/yaml, got %s", contentType)
	}

	var user testUser
	if err := yaml.Unmarshal(w.Body.Bytes(), &user); err != nil {
		t.Errorf("Failed to parse YAML response: %v", err)
	}

	if user.Name != "Bob" {
		t.Errorf("Expected name Bob, got %s", user.Name)
	}
}

func TestFormatHandler_CSV(t *testing.T) {
	handler := FormatHandler(func(r *http.Request) (any, error) {
		return []testUser{
			{Name: "Alice", Email: "alice@example.com"},
			{Name: "Bob", Email: "bob@example.com"},
		}, nil
	})

	req := httptest.NewRequest("GET", "http://example.com/users?format=csv", nil)
	w := httptest.NewRecorder()

	handler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "text/csv" {
		t.Errorf("Expected Content-Type text/csv, got %s", contentType)
	}

	body := w.Body.String()
	if !strings.Contains(body, "Alice") || !strings.Contains(body, "Bob") {
		t.Errorf("CSV output should contain user data, got: %s", body)
	}
}

func TestFormatHandler_AcceptHeader(t *testing.T) {
	handler := FormatHandler(func(r *http.Request) (any, error) {
		return testUser{Name: "Charlie", Email: "charlie@example.com"}, nil
	})

	req := httptest.NewRequest("GET", "http://example.com/users", nil)
	req.Header.Set("Accept", "application/json")
	w := httptest.NewRecorder()

	handler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected Content-Type application/json from Accept header, got %s", contentType)
	}
}

func TestFormatHandler_PagingHeaders(t *testing.T) {
	handler := FormatHandler(func(r *http.Request) (any, error) {
		return []testUser{
			{Name: "User1", Email: "user1@example.com"},
			{Name: "User2", Email: "user2@example.com"},
		}, nil
	})

	req := httptest.NewRequest("GET", "http://example.com/users?format=json&page=2&limit=10", nil)
	w := httptest.NewRecorder()

	handler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	page := w.Header().Get("X-Page")
	if page != "2" {
		t.Errorf("Expected X-Page header to be 2, got %s", page)
	}

	limit := w.Header().Get("X-Per-Page")
	if limit != "10" {
		t.Errorf("Expected X-Per-Page header to be 10, got %s", limit)
	}
}

func TestFormatHandler_Error(t *testing.T) {
	handler := FormatHandler(func(r *http.Request) (any, error) {
		return nil, errors.New("something went wrong")
	})

	req := httptest.NewRequest("GET", "http://example.com/users?format=json", nil)
	w := httptest.NewRecorder()

	handler(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "Internal server error") {
		t.Errorf("Expected error message in response, got: %s", body)
	}
}

func TestFormatHandler_NilData(t *testing.T) {
	handler := FormatHandler(func(r *http.Request) (any, error) {
		return nil, nil
	})

	req := httptest.NewRequest("GET", "http://example.com/users?format=json", nil)
	w := httptest.NewRecorder()

	handler(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404 for nil data, got %d", w.Code)
	}
}

func TestFormatHandler_DefaultFormat(t *testing.T) {
	handler := FormatHandler(func(r *http.Request) (any, error) {
		return testUser{Name: "Dave", Email: "dave@example.com"}, nil
	})

	// No format specified - should default to JSON for HTTP
	req := httptest.NewRequest("GET", "http://example.com/users", nil)
	w := httptest.NewRecorder()

	handler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected default Content-Type application/json, got %s", contentType)
	}
}

func TestExtensionToFormat(t *testing.T) {
	tests := []struct {
		ext      string
		expected string
	}{
		{"json", "json"},
		{"yaml", "yaml"},
		{"yml", "yaml"},
		{"csv", "csv"},
		{"html", "html"},
		{"md", "markdown"},
		{"pdf", "pdf"},
		{"xlsx", "excel"},
		{"txt", "pretty"},
		{"unknown", ""},
	}

	for _, tt := range tests {
		t.Run(tt.ext, func(t *testing.T) {
			result := extensionToFormat(tt.ext)
			if result != tt.expected {
				t.Errorf("extensionToFormat(%v) = %v, want %v", tt.ext, result, tt.expected)
			}
		})
	}
}

func TestGetFormatFromPath(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"/api/users.json", "json"},
		{"/api/users.yaml", "yaml"},
		{"/api/v1/data.csv", "csv"},
		{"/api/users", ""},
		{"/api/users.html", "html"},
		{"/path/to/file.unknown", ""},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := getFormatFromPath(tt.path)
			if result != tt.expected {
				t.Errorf("getFormatFromPath(%v) = %v, want %v", tt.path, result, tt.expected)
			}
		})
	}
}

func TestAcceptToFormat_Quality(t *testing.T) {
	tests := []struct {
		name     string
		accept   string
		expected string
	}{
		{"single type", "application/json", "json"},
		{"with quality", "application/json;q=0.9", "json"},
		{"multiple with quality", "text/html,application/json;q=0.9", "html"},
		{"complex", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8", "html"},
		{"yaml preference", "application/yaml,application/json;q=0.9", "yaml"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := acceptToFormat(tt.accept)
			if result != tt.expected {
				t.Errorf("acceptToFormat(%v) = %v, want %v", tt.accept, result, tt.expected)
			}
		})
	}
}
