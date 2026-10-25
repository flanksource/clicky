package task

import (
	"os"
	"testing"
)

func TestForceInteractiveBypassesTestEnvGuard(t *testing.T) {
	origForce := IsForceInteractive()
	origNoProgress := global.noProgress.Load()
	origNoColor := global.noColor.Load()
	t.Cleanup(func() {
		SetForceInteractive(origForce)
		global.noProgress.Store(origNoProgress)
		global.noColor.Store(origNoColor)
	})

	SetForceInteractive(true)
	global.noProgress.Store(false)
	global.noColor.Store(false)

	if !IsForceInteractive() {
		t.Fatalf("SetForceInteractive(true) then IsForceInteractive()=false")
	}

	if isTestEnvironment() && !IsForceInteractive() {
		t.Fatalf("guard should short-circuit via IsForceInteractive() in init()")
	}

	if global.noProgress.Load() {
		t.Fatalf("expected noProgress to remain false under force-interactive")
	}
	if global.noColor.Load() {
		t.Fatalf("expected noColor to remain false under force-interactive")
	}
	if !global.isInteractive.Load() {
		t.Fatalf("expected isInteractive=true after SetForceInteractive(true)")
	}
}

func TestForceInteractiveEnvVarSetsFlag(t *testing.T) {
	origForce := IsForceInteractive()
	t.Cleanup(func() { SetForceInteractive(origForce) })

	SetForceInteractive(false)
	t.Setenv("CLICKY_FORCE_INTERACTIVE", "1")

	if os.Getenv("CLICKY_FORCE_INTERACTIVE") != "" {
		SetForceInteractive(true)
	}

	if !IsForceInteractive() {
		t.Fatalf("CLICKY_FORCE_INTERACTIVE=1 should enable force-interactive")
	}
}

func TestDefaultStillAutoDisablesUnderGoTest(t *testing.T) {
	if os.Getenv("GO_TEST") == "" {
		t.Setenv("GO_TEST", "1")
	}
	if !isTestEnvironment() {
		t.Fatalf("isTestEnvironment() must return true when GO_TEST is set")
	}

	origForce := IsForceInteractive()
	t.Cleanup(func() { SetForceInteractive(origForce) })

	SetForceInteractive(false)
	if IsForceInteractive() {
		t.Fatalf("force-interactive should be off by default in this subtest")
	}

	if isTestEnvironment() && !IsForceInteractive() {
		return
	}
	t.Fatalf("default guard branch not reachable")
}

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
