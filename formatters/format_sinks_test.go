package formatters

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestParseFormatSpec(t *testing.T) {
	tests := []struct {
		name    string
		format  string
		booleans func(*FormatOptions)
		want    []FormatSink
		wantErr string
	}{
		{
			name:   "empty spec yields no sinks",
			format: "",
			want:   nil,
		},
		{
			name:   "single stdout format",
			format: "json",
			want:   []FormatSink{{Format: "json"}},
		},
		{
			name:   "single file sink",
			format: "json=out.json",
			want:   []FormatSink{{Format: "json", File: "out.json"}},
		},
		{
			name:   "multiple file sinks",
			format: "json=a.json,markdown=b.md,html=c.html",
			want: []FormatSink{
				{Format: "json", File: "a.json"},
				{Format: "markdown", File: "b.md"},
				{Format: "html", File: "c.html"},
			},
		},
		{
			name:   "md alias canonicalizes to markdown",
			format: "md=summary.md",
			want:   []FormatSink{{Format: "markdown", File: "summary.md"}},
		},
		{
			name:   "bare format mixed with file sinks",
			format: "pretty,json=out.json",
			want: []FormatSink{
				{Format: "pretty"},
				{Format: "json", File: "out.json"},
			},
		},
		{
			name:   "whitespace is tolerated",
			format: "  json = out.json , html = report.html ",
			want: []FormatSink{
				{Format: "json", File: "out.json"},
				{Format: "html", File: "report.html"},
			},
		},
		{
			name:    "two bare formats is an error",
			format:  "json,markdown",
			wantErr: "more than one stdout format",
		},
		{
			name:    "unknown format is an error",
			format:  "xml=out.xml",
			wantErr: "unknown format",
		},
		{
			name:    "empty file after equals is an error",
			format:  "json=",
			wantErr: "empty file path",
		},
		{
			name:    "empty format name is an error",
			format:  "=out.json",
			wantErr: "empty format name",
		},
		{
			name:     "legacy JSON boolean synthesizes a stdout sink",
			format:   "",
			booleans: func(o *FormatOptions) { o.JSON = true },
			want:     []FormatSink{{Format: "json"}},
		},
		{
			name:     "legacy HTML boolean synthesizes a stdout sink",
			format:   "",
			booleans: func(o *FormatOptions) { o.HTML = true },
			want:     []FormatSink{{Format: "html"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := FormatOptions{Format: tt.format}
			if tt.booleans != nil {
				tt.booleans(&opts)
			}
			err := opts.ParseFormatSpec()
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error containing %q, got %q", tt.wantErr, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(opts.Sinks, tt.want) {
				t.Fatalf("sinks mismatch\n got: %#v\nwant: %#v", opts.Sinks, tt.want)
			}
		})
	}
}

func TestParseFormatSpecIdempotent(t *testing.T) {
	opts := FormatOptions{Format: "json=a.json"}
	if err := opts.ParseFormatSpec(); err != nil {
		t.Fatalf("first parse: %v", err)
	}
	first := append([]FormatSink(nil), opts.Sinks...)
	opts.Format = "markdown=b.md"
	if err := opts.ParseFormatSpec(); err != nil {
		t.Fatalf("second parse: %v", err)
	}
	if !reflect.DeepEqual(opts.Sinks, first) {
		t.Fatalf("second parse mutated sinks: got %#v want %#v", opts.Sinks, first)
	}
}

// sinkSample is a trivial struct used as the "render me" payload in the
// integration tests below. It has to be exported fields so every formatter
// can pick it up.
type sinkSample struct {
	Name  string
	Count int
}

func TestFormatManagerWritesEachFileSink(t *testing.T) {
	dir := t.TempDir()
	jsonPath := filepath.Join(dir, "result.json")
	htmlPath := filepath.Join(dir, "result.html")
	mdPath := filepath.Join(dir, "result.md")

	data := sinkSample{Name: "gavel", Count: 3}
	mgr := NewFormatManager()

	for _, s := range []FormatSink{
		{Format: "json", File: jsonPath},
		{Format: "html", File: htmlPath},
		{Format: "markdown", File: mdPath},
	} {
		opts := FormatOptions{Format: s.Format, Output: s.File}
		if err := mgr.FormatToFile(opts, data); err != nil {
			t.Fatalf("FormatToFile(%s): %v", s.Format, err)
		}
	}

	jsonBytes, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("read json: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(jsonBytes, &decoded); err != nil {
		t.Fatalf("json output is not valid JSON: %v\n%s", err, jsonBytes)
	}

	htmlBytes, err := os.ReadFile(htmlPath)
	if err != nil {
		t.Fatalf("read html: %v", err)
	}
	if len(htmlBytes) == 0 {
		t.Fatal("html output is empty")
	}

	mdBytes, err := os.ReadFile(mdPath)
	if err != nil {
		t.Fatalf("read markdown: %v", err)
	}
	if len(mdBytes) == 0 {
		t.Fatal("markdown output is empty")
	}
}

func TestFormatManagerSinkErrorDoesNotAbortOthers(t *testing.T) {
	dir := t.TempDir()
	goodPath := filepath.Join(dir, "good.json")
	// Unwritable: a path whose parent directory does not exist.
	badPath := filepath.Join(dir, "does", "not", "exist", "bad.html")

	mgr := NewFormatManager()
	data := sinkSample{Name: "tolerant", Count: 1}

	goodErr := mgr.FormatToFile(FormatOptions{Format: "json", Output: goodPath}, data)
	badErr := mgr.FormatToFile(FormatOptions{Format: "html", Output: badPath}, data)

	if goodErr != nil {
		t.Fatalf("good sink unexpectedly failed: %v", goodErr)
	}
	if badErr == nil {
		t.Fatal("bad sink was expected to fail but succeeded")
	}
	if _, err := os.Stat(goodPath); err != nil {
		t.Fatalf("good sink file missing: %v", err)
	}
}
