package task

import "testing"

func TestDefaultNoColor(t *testing.T) {
	tests := []struct {
		name          string
		isInteractive bool
		env           map[string]string
		want          bool
	}{
		{name: "interactive with no env", isInteractive: true, want: false},
		{name: "non-interactive", isInteractive: false, want: true},
		{name: "NO_COLOR set", isInteractive: true, env: map[string]string{"NO_COLOR": "1"}, want: true},
		{name: "TERM=dumb", isInteractive: true, env: map[string]string{"TERM": "dumb"}, want: true},
		{name: "COLOR=no", isInteractive: true, env: map[string]string{"COLOR": "no"}, want: true},
		{name: "COLOR=FALSE", isInteractive: true, env: map[string]string{"COLOR": "FALSE"}, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("NO_COLOR", "")
			t.Setenv("COLOR", "")
			t.Setenv("TERM", "xterm-256color")
			for k, v := range tt.env {
				t.Setenv(k, v)
			}
			if got := defaultNoColor(tt.isInteractive); got != tt.want {
				t.Errorf("defaultNoColor(%v) with env %v = %v, want %v", tt.isInteractive, tt.env, got, tt.want)
			}
		})
	}
}
