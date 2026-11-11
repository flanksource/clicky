// Package telltale makes a clicky process observable: it embeds the telltale
// agent with probes for clicky exec, entity operations and tasks, commons
// logs and HTTP traffic through http.DefaultTransport, and a gops agent so the
// gops CLI and `telltale gops` keep working against the same process.
package telltale

import (
	"errors"
	"fmt"
	"sync"

	"github.com/flanksource/clicky/shutdown"
	"github.com/flanksource/commons/logger"
	gops "github.com/google/gops/agent"
	sdk "github.com/flanksource/telltale/sdk"

	_ "github.com/flanksource/telltale/sdk/commonslog"
)

func init() {
	sdk.RegisterProbe(execProbe{})
	sdk.RegisterProbe(entityProbe{})
	sdk.RegisterProbe(taskProbe{})
	registerOps()
}

// Options configures the embedded agents. Dial and Token default to
// TELLTALE_URL and TELLTALE_TOKEN; without a Dial the agent serves a local
// socket advertised under <UserConfigDir>/telltale/agents/<pid>.json.
type Options struct {
	Service, Version string
	Dial, Token      string
	Labels           map[string]string
	AllowMutation    bool
	// NoGops leaves the gops agent stopped; Gops configures it otherwise.
	NoGops bool
	Gops   gops.Options
}

// Agent is a running telltale agent and, unless disabled, a gops agent.
type Agent struct {
	telltale *sdk.Agent
	gops     bool
	once     sync.Once
	err      error
}

var instrumentHTTP sync.Once

// Start instruments http.DefaultTransport, starts the telltale agent and the
// gops agent, and stops both on clicky shutdown. Call it first thing in main:
// a commons client that cloned its transport before instrumentation is not
// captured.
func Start(options Options) (*Agent, error) {
	instrumentHTTP.Do(sdk.InstrumentDefaultTransport)
	agent, err := sdk.Start(sdk.Options{Service: options.Service, Version: options.Version, Dial: options.Dial,
		Token: options.Token, Labels: options.Labels, AllowMutation: options.AllowMutation})
	if err != nil {
		return nil, fmt.Errorf("telltale agent: %w", err)
	}
	a := &Agent{telltale: agent}
	if !options.NoGops {
		if err := StartGops(options.Gops); err != nil {
			return nil, errors.Join(err, agent.Close())
		}
		a.gops = true
	}
	shutdown.AddHook("Stopping telltale agent", func() {
		if err := a.Close(); err != nil {
			logger.Errorf("stopping telltale agent: %v", err)
		}
	})
	return a, nil
}

// StartGops starts only the gops agent, for processes not traced by telltale.
// Signal cleanup is clicky's shutdown hooks' job, never gops's own handler.
func StartGops(options gops.Options) error {
	options.ShutdownCleanup = false
	if err := gops.Listen(options); err != nil {
		return fmt.Errorf("gops agent: %w", err)
	}
	return nil
}

// Close stops both agents; later calls return the first call's result.
func (a *Agent) Close() error {
	a.once.Do(func() {
		if a.gops {
			gops.Close()
		}
		a.err = a.telltale.Close()
	})
	return a.err
}
