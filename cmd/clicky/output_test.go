package main

import (
	"path/filepath"
	"testing"
)

func TestResolveInputOutputPath(t *testing.T) {
	tests := []struct {
		name          string
		outputPattern string
		sourceName    string
		format        string
		want          string
	}{
		{
			name:          "wildcard without extension gets the format extension",
			outputPattern: "out/*",
			sourceName:    "data.json",
			format:        "yaml",
			want:          "out/data.yaml",
		},
		{
			name:          "wildcard with explicit extension is preserved",
			outputPattern: "out/*.txt",
			sourceName:    "data.json",
			format:        "yaml",
			want:          "out/data.txt",
		},
		{
			name:          "wildcard with stdin source uses stdin base",
			outputPattern: "*",
			sourceName:    "",
			format:        "csv",
			want:          "stdin.csv",
		},
		{
			name:          "directory pattern joins base plus extension",
			outputPattern: "out",
			sourceName:    "data.json",
			format:        "json",
			want:          filepath.Join("out", "data.json"),
		},
		{
			name:          "explicit file path is returned unchanged",
			outputPattern: "out/report.pdf",
			sourceName:    "data.json",
			format:        "json",
			want:          "out/report.pdf",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolveInputOutputPath(tt.outputPattern, tt.sourceName, tt.format); got != tt.want {
				t.Fatalf("resolveInputOutputPath(%q, %q, %q) = %q, want %q",
					tt.outputPattern, tt.sourceName, tt.format, got, tt.want)
			}
		})
	}
}
