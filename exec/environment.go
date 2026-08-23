package exec

import (
	"fmt"
	"os"
	"sort"
)

// WithExactEnv replaces the host environment for this process. Subsequent
// WithEnv calls merge into this environment and override matching keys.
func (p *Process) WithExactEnv(env map[string]string) *Process {
	p.Env = make(map[string]string, len(env))
	for key, value := range env {
		p.Env[key] = value
	}
	p.exactEnvironment = true
	return p
}

func (p *Process) environment() []string {
	if !p.exactEnvironment {
		env := append([]string(nil), os.Environ()...)
		for key, value := range p.Env {
			env = append(env, fmt.Sprintf("%s=%s", key, value))
		}
		return env
	}

	keys := make([]string, 0, len(p.Env))
	for key := range p.Env {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	env := make([]string, 0, len(keys))
	for _, key := range keys {
		env = append(env, fmt.Sprintf("%s=%s", key, p.Env[key]))
	}
	return env
}
