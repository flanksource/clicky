package exec

import "time"

// SuperviseOptions configures the supervision loop.
type SuperviseOptions struct {
	// ForceStop kills the owned process tree immediately instead of waiting for StopGrace.
	ForceStop bool
	// Limits bounds resource usage; zero values disable individual limits.
	Limits ResourceLimits
	// RestartPolicy controls restart-on-exit; defaults to RestartNo.
	RestartPolicy RestartPolicy
	// MaxRestarts caps automatic restarts (0 = unlimited).
	MaxRestarts int
	// StopGrace is the SIGTERM→SIGKILL window on Stop; defaults to 5s.
	StopGrace time.Duration
	// DetectPorts runs the lsof port-watch loop after each start.
	DetectPorts bool
	// OnStart, if set, is called before each (re)start (e.g. to write a log header).
	OnStart func()
	// OnStarted, if set, is called after each (re)start once the child is running
	// and is the current process — i.e. its Stdin()/StdoutReader() are available.
	// Used by JSON-RPC providers to (re)bind their client to the fresh child's
	// stdio and re-send the initialize handshake. It is invoked synchronously on
	// the supervise loop, so blocking work (e.g. awaiting a handshake response)
	// MUST be done in a goroutine, otherwise resource monitoring stalls.
	OnStarted func(*Process)
	// OnExit, if set, is called once when the supervise loop ends permanently
	// (the process exited and will not be restarted, or it was stopped).
	OnExit func()
	// Task customizes the automatic task run created for every process
	// generation. Zero values use the supervised-process defaults.
	Task SupervisedTaskOptions
}
