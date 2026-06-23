package rpc

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/flanksource/clicky/entity"
	"github.com/flanksource/commons/logger"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// globalCaptureMu serializes process-wide os.Stdout/os.Stderr replacement across
// ALL CobraExecutableCommand executions. It replaces CommandExecutor.mutex and
// the MCP server's per-server execution mutex — any code path that swaps the
// global file descriptors (or mutates a shared command's flags before running it)
// must hold this lock.
var globalCaptureMu sync.Mutex

// CobraExecutableCommand adapts a *cobra.Command to entity.ExecutableCommand.
type CobraExecutableCommand struct {
	cmd *cobra.Command
}

// NewCobraExecutableCommand wraps cmd as an entity.ExecutableCommand. It returns
// a nil interface for a nil command so callers can keep their existing
// `if op.Command == nil` guards.
func NewCobraExecutableCommand(cmd *cobra.Command) entity.ExecutableCommand {
	if cmd == nil {
		return nil
	}
	return &CobraExecutableCommand{cmd: cmd}
}

func (c *CobraExecutableCommand) Path() string     { return getCommandPath(c.cmd) }
func (c *CobraExecutableCommand) Name() string     { return c.cmd.Name() }
func (c *CobraExecutableCommand) RootName() string { return rootCommand(c.cmd).Name() }
func (c *CobraExecutableCommand) Runnable() bool   { return c.cmd.Runnable() }
func (c *CobraExecutableCommand) Hidden() bool     { return c.cmd.Hidden }

func (c *CobraExecutableCommand) IsBoolFlag(name string) bool {
	f := c.cmd.Flags().Lookup(name)
	return f != nil && f.Value.Type() == "bool"
}

// Execute applies opts onto the wrapped command and runs a child-less copy with
// the full cobra lifecycle (flag/arg parsing, required-flag validation, RunE),
// capturing everything written to os.Stdout/os.Stderr. The whole call is
// serialized by globalCaptureMu so concurrent callers neither corrupt each
// other's captured streams nor race on the shared command's flag state.
func (c *CobraExecutableCommand) Execute(ctx context.Context, opts entity.ExecuteOptions) (string, string, error) {
	globalCaptureMu.Lock()
	defer globalCaptureMu.Unlock()

	cmd := c.cmd
	resetCommandState(cmd)

	for name, val := range opts.Flags {
		if flag := lookupAnyFlag(cmd, name); flag != nil {
			if err := flag.Value.Set(val); err != nil {
				return "", "", fmt.Errorf("invalid value for flag %s: %w", name, err)
			}
			flag.Changed = true
		}
	}
	for name, val := range opts.FormatOverrides {
		if flag := lookupAnyFlag(cmd, name); flag != nil {
			_ = flag.Value.Set(val)
			flag.Changed = true
		}
	}

	// A child-less copy runs RunE directly (no subcommand dispatch, no inherited
	// pre-run hooks) while sharing the source command's flag pointers so
	// required-flag validation sees the values applied above.
	execCmd := &cobra.Command{
		Use:   cmd.Use,
		Short: cmd.Short,
		Long:  cmd.Long,
		Run:   cmd.Run,
		RunE:  cmd.RunE,
		Args:  cmd.Args,
	}
	cmd.Flags().VisitAll(func(flag *pflag.Flag) {
		execCmd.Flags().AddFlag(flag)
	})
	if ctx != nil {
		execCmd.SetContext(ctx)
	}

	return captureGlobal(execCmd, opts.Args)
}

// captureGlobal swaps os.Stdout/os.Stderr to pipes, runs execCmd, and returns the
// captured streams. The caller MUST hold globalCaptureMu.
func captureGlobal(execCmd *cobra.Command, args []string) (stdout, stderr string, err error) {
	var stdoutBuf, stderrBuf bytes.Buffer

	originalStdout := os.Stdout
	originalStderr := os.Stderr
	originalArgs := os.Args
	defer func() {
		os.Stdout = originalStdout
		os.Stderr = originalStderr
		os.Args = originalArgs
	}()

	stdoutReader, stdoutWriter, perr := os.Pipe()
	if perr != nil {
		return "", "", fmt.Errorf("failed to create stdout pipe: %w", perr)
	}
	defer func() {
		if cerr := stdoutReader.Close(); cerr != nil {
			logger.Errorf("failed to close stdout reader: %v", cerr)
		}
	}()

	stderrReader, stderrWriter, perr := os.Pipe()
	if perr != nil {
		_ = stdoutWriter.Close()
		return "", "", fmt.Errorf("failed to create stderr pipe: %w", perr)
	}
	defer func() {
		if cerr := stderrReader.Close(); cerr != nil {
			logger.Errorf("failed to close stderr reader: %v", cerr)
		}
	}()

	os.Stdout = stdoutWriter
	os.Stderr = stderrWriter
	execCmd.SetOut(stdoutWriter)
	execCmd.SetErr(stderrWriter)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		if _, cerr := io.Copy(&stdoutBuf, stdoutReader); cerr != nil {
			logger.Errorf("failed to copy stdout: %v", cerr)
		}
	}()
	go func() {
		defer wg.Done()
		if _, cerr := io.Copy(&stderrBuf, stderrReader); cerr != nil {
			logger.Errorf("failed to copy stderr: %v", cerr)
		}
	}()

	// Cobra falls back to parsing os.Args[1:] when a command's args are nil (see
	// Command.ExecuteC). For a server invocation that would leak the server's own
	// process flags (e.g. --port) into the executed command, so always pass a
	// non-nil slice — an empty request must run the command with no args, not the
	// host process's args.
	if args == nil {
		args = []string{}
	}
	execCmd.SetArgs(args)
	cmdErr := execCmd.Execute()

	_ = stdoutWriter.Close()
	_ = stderrWriter.Close()
	wg.Wait()

	return stdoutBuf.String(), stderrBuf.String(), cmdErr
}

// resetCommandState clears any flag values/args left over from a previous
// invocation so a reused command starts each call from its declared defaults.
func resetCommandState(cmd *cobra.Command) {
	cmd.SetArgs(nil)
	resetFlagSet(cmd.Flags())
	resetFlagSet(cmd.PersistentFlags())
	resetFlagSet(cmd.InheritedFlags())
}

func resetFlagSet(flags *pflag.FlagSet) {
	if flags == nil {
		return
	}
	flags.VisitAll(func(flag *pflag.Flag) {
		_ = flag.Value.Set(flag.DefValue)
		flag.Changed = false
	})
}

func lookupAnyFlag(cmd *cobra.Command, name string) *pflag.Flag {
	if flag := cmd.Flags().Lookup(name); flag != nil {
		return flag
	}
	if flag := cmd.PersistentFlags().Lookup(name); flag != nil {
		return flag
	}
	return cmd.InheritedFlags().Lookup(name)
}

// rootCommand walks up to the top-level command.
func rootCommand(cmd *cobra.Command) *cobra.Command {
	for cmd.Parent() != nil {
		cmd = cmd.Parent()
	}
	return cmd
}
