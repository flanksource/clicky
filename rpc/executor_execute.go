package rpc

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/flanksource/clicky/entity"
	rpchttp "github.com/flanksource/clicky/rpc/http"
)

// ExecuteCommand executes a Cobra command with the given parameters.
func (e *CommandExecutor) ExecuteCommand(op *RPCOperation, req *ExecutionRequest) (any, *ExecutionResponse, error) {
	defer rpchttp.Track(req.ctx(), "command")()
	if !e.config.Enabled {
		resp := &ExecutionResponse{Success: false, Error: "Command execution is disabled", Input: req, CLI: buildCLICommand(op, req)}
		return resp, resp, fmt.Errorf("command execution is disabled")
	}

	if op.ContextDataFunc != nil || op.DataFunc != nil {
		var data any
		var err error
		if op.ContextDataFunc != nil {
			data, err = op.ContextDataFunc(req.ctx(), req.Flags, req.Args)
		} else {
			data, err = op.DataFunc(req.Flags, req.Args)
		}
		response := &ExecutionResponse{
			Success: err == nil, ExitCode: 0, CLI: buildCLICommand(op, req), DataIsStructured: err == nil,
		}
		if err != nil {
			response.Error, response.ExitCode = err.Error(), 1
			return response, response, err
		}
		return data, response, nil
	}

	if op.Command == nil {
		resp := &ExecutionResponse{Success: false, Error: "No command associated with operation", Input: req, CLI: buildCLICommand(op, req)}
		return resp, resp, fmt.Errorf("no command found for operation %s", op.Name)
	}

	stdout, stderr, err := op.Command.Execute(req.ctx(), entity.ExecuteOptions{Args: req.Args, Flags: req.Flags})
	parsed, parseErr := parseCommandOutput(stdout)
	response := &ExecutionResponse{
		Success: err == nil, Stdout: stdout, Stderr: stderr, Output: stdout + stderr,
		ExitCode: extractExitCode(err), CLI: buildCLICommand(op, req),
	}
	if err != nil {
		response.Error, response.Message, response.Input = err.Error(), "Command execution failed", req
		return response, response, err
	}
	if parseErr == nil && parsed != nil {
		return parsed, response, nil
	}
	return stdout, response, nil
}

func parseCommandOutput(stdout string) (any, error) {
	if stdout == "" {
		return nil, fmt.Errorf("no output to parse")
	}
	trimmed := strings.TrimSpace(stdout)
	if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
		var data any
		if err := json.Unmarshal([]byte(trimmed), &data); err == nil {
			return data, nil
		}
	}
	return nil, fmt.Errorf("unable to parse output")
}
