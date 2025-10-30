package clicky

import (
	"net/http/httptest"

	"github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = ginkgo.Describe("WithHttpRequest", func() {
	ginkgo.Context("when format specified via query param", func() {
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
			tt := tt
			ginkgo.It(tt.name, func() {
				req := httptest.NewRequest("GET", tt.url, nil)
				opts := WithHttpRequest(req)
				Expect(opts.Format).To(Equal(tt.expected))
			})
		}
	})

	ginkgo.Context("when format specified via path extension", func() {
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
			tt := tt
			ginkgo.It(tt.name, func() {
				req := httptest.NewRequest("GET", tt.url, nil)
				opts := WithHttpRequest(req)
				Expect(opts.Format).To(Equal(tt.expected))
			})
		}
	})

	ginkgo.Context("when format specified via Accept header", func() {
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
			{"wildcard accept", "*/*", "json"},
		}

		for _, tt := range tests {
			tt := tt
			ginkgo.It(tt.name, func() {
				req := httptest.NewRequest("GET", "http://example.com/api/users", nil)
				req.Header.Set("Accept", tt.accept)
				opts := WithHttpRequest(req)
				Expect(opts.Format).To(Equal(tt.expected))
			})
		}
	})

	ginkgo.Context("when determining format priority", func() {
		ginkgo.It("should prioritize query param over path extension", func() {
			req := httptest.NewRequest("GET", "http://example.com/api/users.yaml?format=json", nil)
			req.Header.Set("Accept", "text/csv")
			opts := WithHttpRequest(req)
			Expect(opts.Format).To(Equal("json"))
		})

		ginkgo.It("should prioritize path extension over Accept header", func() {
			req := httptest.NewRequest("GET", "http://example.com/api/users.csv", nil)
			req.Header.Set("Accept", "application/json")
			opts := WithHttpRequest(req)
			Expect(opts.Format).To(Equal("csv"))
		})

		ginkgo.It("should use Accept header when no query param or extension", func() {
			req := httptest.NewRequest("GET", "http://example.com/api/users", nil)
			req.Header.Set("Accept", "text/yaml")
			opts := WithHttpRequest(req)
			Expect(opts.Format).To(Equal("yaml"))
		})
	})

	ginkgo.Context("when handling paging parameters", func() {
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
			tt := tt
			ginkgo.It(tt.name, func() {
				req := httptest.NewRequest("GET", tt.url, nil)
				for key, val := range tt.headers {
					req.Header.Set(key, val)
				}
				opts := WithHttpRequest(req)
				Expect(opts.Page).To(Equal(tt.expectedPage))
				Expect(opts.Limit).To(Equal(tt.expectedLimit))
			})
		}
	})

	ginkgo.Context("when no format is specified", func() {
		ginkgo.It("should default to json", func() {
			req := httptest.NewRequest("GET", "http://example.com/api/users", nil)
			opts := WithHttpRequest(req)
			Expect(opts.Format).To(Equal("json"))
		})
	})

	ginkgo.Context("when Accept header has multiple values", func() {
		ginkgo.It("should pick first recognized format", func() {
			req := httptest.NewRequest("GET", "http://example.com/api/users", nil)
			req.Header.Set("Accept", "text/html,application/json;q=0.9,*/*;q=0.8")
			opts := WithHttpRequest(req)
			Expect(opts.Format).To(Equal("html"))
		})
	})
})

var _ = ginkgo.Describe("FormatToContentType", func() {
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
		tt := tt
		ginkgo.It("should convert "+tt.format+" to correct content type", func() {
			result := FormatToContentType(tt.format)
			Expect(result).To(Equal(tt.expected))
		})
	}
})
