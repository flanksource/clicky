// Package auto starts clicky's telltale and gops agents from the environment
// when imported for side effects:
//
//	import _ "github.com/flanksource/clicky/telltale/auto"
//
// TELLTALE_AGENT=1 or TELLTALE_URL starts both agents (GOPS=0 leaves gops
// off); GOPS=1 on its own starts only the gops agent.
package auto

import (
	"os"

	"github.com/flanksource/commons/logger"
	gops "github.com/google/gops/agent"

	"github.com/flanksource/clicky/telltale"
)

// Agent is the running agent, nil when the environment did not ask for one.
var Agent *telltale.Agent

func init() {
	wantTelltale := os.Getenv("TELLTALE_AGENT") == "1" || os.Getenv("TELLTALE_URL") != ""
	switch {
	case wantTelltale:
		agent, err := telltale.Start(telltale.Options{NoGops: os.Getenv("GOPS") == "0"})
		if err != nil {
			logger.Errorf("telltale auto start: %v", err)
			return
		}
		Agent = agent
	case os.Getenv("GOPS") == "1":
		if err := telltale.StartGops(gops.Options{}); err != nil {
			logger.Errorf("gops auto start: %v", err)
		}
	}
}
