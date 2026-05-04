package mcp

import (
	"strings"
	"testing"

	"github.com/flanksource/clicky/formatters"
)

func TestBuildDiscoverToolsText_Empty(t *testing.T) {
	out := RenderDiscoverToolsString(map[string]*ToolDefinition{}, nil)
	if !strings.Contains(out, "No tools are currently registered") {
		t.Fatalf("expected empty-state message, got %q", out)
	}
	if !strings.Contains(out, "MCP Tool Discovery") {
		t.Fatalf("expected header, got %q", out)
	}
}

func TestBuildDiscoverToolsText_OrdersAndBadges(t *testing.T) {
	tools := map[string]*ToolDefinition{
		"zeta": {
			Name:        "zeta",
			Description: "zeta does z things",
			InputSchema: Schema{
				Type:     "object",
				Required: []string{"id"},
				Properties: map[string]Property{
					"id":     {Type: "string", Description: "identifier"},
					"format": {Type: "string", Description: "output", Enum: []string{"json", "yaml"}, Default: "json"},
				},
			},
		},
		"alpha": {
			Name:        "alpha",
			Description: "alpha is first alphabetically",
			InputSchema: Schema{Type: "object"},
		},
	}

	// Plain string format keeps assertions simple — no markdown/HTML escapes.
	plain := RenderDiscoverToolsString(tools, &formatters.FormatOptions{NoColor: true})

	idxAlpha := strings.Index(plain, "alpha")
	idxZeta := strings.Index(plain, "zeta")
	if idxAlpha == -1 || idxZeta == -1 || idxAlpha > idxZeta {
		t.Fatalf("expected alpha before zeta, got:\n%s", plain)
	}

	zetaSection := plain[idxZeta:]
	idxRequired := strings.Index(zetaSection, "id")
	idxOptional := strings.Index(zetaSection, "format")
	if idxRequired == -1 || idxOptional == -1 {
		t.Fatalf("expected both params in zeta section, got:\n%s", zetaSection)
	}
	if idxRequired > idxOptional {
		t.Fatalf("expected required param 'id' to render before optional 'format'")
	}

	if !strings.Contains(zetaSection, "required") {
		t.Fatalf("expected required badge in output:\n%s", zetaSection)
	}
	if !strings.Contains(zetaSection, "optional") {
		t.Fatalf("expected optional badge in output:\n%s", zetaSection)
	}
	if !strings.Contains(zetaSection, "values: json | yaml") {
		t.Fatalf("expected enum values in output:\n%s", zetaSection)
	}
	if !strings.Contains(zetaSection, "default: json") {
		t.Fatalf("expected default value in output:\n%s", zetaSection)
	}

	alphaSection := plain[idxAlpha:idxZeta]
	if !strings.Contains(alphaSection, "No parameters") {
		t.Fatalf("expected no-parameters note for alpha:\n%s", alphaSection)
	}
}

func TestPickFormat(t *testing.T) {
	cases := []struct {
		name string
		opts *formatters.FormatOptions
		want string
	}{
		{"nil", nil, "markdown"},
		{"markdown bool", &formatters.FormatOptions{Markdown: true}, "markdown"},
		{"pretty bool", &formatters.FormatOptions{Pretty: true}, "ansi"},
		{"pretty no-color", &formatters.FormatOptions{Pretty: true, NoColor: true}, "plain"},
		{"format string", &formatters.FormatOptions{Format: "Markdown"}, "markdown"},
		{"no-color only", &formatters.FormatOptions{NoColor: true}, "plain"},
	}
	for _, c := range cases {
		if got := pickFormat(c.opts); got != c.want {
			t.Errorf("%s: pickFormat = %q, want %q", c.name, got, c.want)
		}
	}
}
