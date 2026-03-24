package formatters

import (
	"testing"
)

func TestResolveNoColor(t *testing.T) {
	tests := []struct {
		name     string
		env      map[string]string
		flagSet  bool
		expected bool
	}{
		{"no env, no flag", nil, false, false},
		{"--no-color flag", nil, true, true},
		{"NO_COLOR=true", map[string]string{"NO_COLOR": "true"}, false, true},
		{"NO_COLOR=1", map[string]string{"NO_COLOR": "1"}, false, true},
		{"NO_COLOR=empty string is ignored", map[string]string{}, false, false},
		{"COLOR=no", map[string]string{"COLOR": "no"}, false, true},
		{"COLOR=false", map[string]string{"COLOR": "false"}, false, true},
		{"COLOR=FALSE", map[string]string{"COLOR": "FALSE"}, false, true},
		{"COLOR=yes is ignored", map[string]string{"COLOR": "yes"}, false, false},
		{"TERM=dumb", map[string]string{"TERM": "dumb"}, false, true},
		{"TERM=xterm-256color", map[string]string{"TERM": "xterm-256color"}, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.env {
				t.Setenv(k, v)
			}

			opts := FormatOptions{NoColor: tt.flagSet}
			opts.ResolveNoColor()

			if opts.NoColor != tt.expected {
				t.Errorf("NoColor = %v, want %v", opts.NoColor, tt.expected)
			}
		})
	}
}
