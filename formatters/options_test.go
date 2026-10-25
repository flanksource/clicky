package formatters

import (
	"testing"

	"github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestResolveNoColor(t *testing.T) {
	tests := []struct {
		name     string
		env      map[string]string
		flagSet  bool
		redirect bool
		expected bool
	}{
		{"no env, no flag", nil, false, false, false},
		{"--no-color flag", nil, true, false, true},
		{"NO_COLOR=true", map[string]string{"NO_COLOR": "true"}, false, false, true},
		{"NO_COLOR=1", map[string]string{"NO_COLOR": "1"}, false, false, true},
		{"NO_COLOR=empty string is ignored", map[string]string{}, false, false, false},
		{"COLOR=no", map[string]string{"COLOR": "no"}, false, false, true},
		{"COLOR=false", map[string]string{"COLOR": "false"}, false, false, true},
		{"COLOR=FALSE", map[string]string{"COLOR": "FALSE"}, false, false, true},
		{"COLOR=yes is ignored", map[string]string{"COLOR": "yes"}, false, false, false},
		{"TERM=dumb", map[string]string{"TERM": "dumb"}, false, false, true},
		{"TERM=xterm-256color", map[string]string{"TERM": "xterm-256color"}, false, false, false},
		{"stdout redirected", nil, false, true, true},
		{"stdout redirected, CLICKY_FORCE_INTERACTIVE", map[string]string{"CLICKY_FORCE_INTERACTIVE": "1"}, false, true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("NO_COLOR", "")
			t.Setenv("COLOR", "")
			t.Setenv("TERM", "xterm-256color")
			t.Setenv("CLICKY_FORCE_INTERACTIVE", "")
			for k, v := range tt.env {
				t.Setenv(k, v)
			}
			restore := stdoutIsTerminal
			redirected := tt.redirect
			stdoutIsTerminal = func() bool { return !redirected }
			t.Cleanup(func() { stdoutIsTerminal = restore })

			opts := FormatOptions{NoColor: tt.flagSet}
			opts.ResolveNoColor()

			if opts.NoColor != tt.expected {
				t.Errorf("NoColor = %v, want %v", opts.NoColor, tt.expected)
			}
		})
	}
}

func TestFormatters(t *testing.T) {
	RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "Formatters Suite")
}
