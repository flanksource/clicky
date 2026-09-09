package rpc

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/flanksource/clicky/entity"
	"github.com/flanksource/commons/logger"
)

func (s *SwaggerServer) executeCommandCore(r *http.Request) (any, *ExecutionResponse, int, error) {
	operation := s.executor.FindOperation(r.Method, r.URL.EscapedPath())
	if operation == nil {
		response := &ExecutionResponse{
			Success: false, Error: fmt.Sprintf("No operation found for %s %s", r.Method, r.URL.Path),
		}
		return response, response, http.StatusNotFound, fmt.Errorf("operation not found")
	}
	return s.executeOperation(r, operation)
}

func (s *SwaggerServer) executeOperation(r *http.Request, operation *RPCOperation) (any, *ExecutionResponse, int, error) {
	request, err := s.executor.ExtractRequestFromHTTP(r, operation)
	if err != nil {
		response := &ExecutionResponse{
			Success: false, Error: fmt.Sprintf("Failed to extract parameters: %v", err), Input: request,
		}
		return response, response, http.StatusBadRequest, err
	}
	if err := s.executor.ValidateParameters(request, operation); err != nil {
		response := &ExecutionResponse{
			Success: false, Error: fmt.Sprintf("Parameter validation failed: %v", err), Input: request,
		}
		return response, response, http.StatusBadRequest, err
	}

	data, metadata, err := s.executor.ExecuteCommand(operation, request)
	if err != nil {
		return data, metadata, statusForError(err), err
	}
	return data, metadata, http.StatusOK, nil
}

// statusForError honors the status an error classified itself with. A handler
// that rejected a malformed request with entity.NewStatusError(400, …) said 400,
// and that is a fact about the request rather than about which error format the
// server serves — the legacy envelope carries it in the status line just as the
// structured one does. Only an unclassified failure is the server's own 500.
func statusForError(err error) int {
	var status *entity.StatusError
	if errors.As(err, &status) {
		return status.StatusCode()
	}
	return http.StatusInternalServerError
}

func (s *SwaggerServer) handleExecuteCommand(w http.ResponseWriter, r *http.Request) {
	// The origin policy lives in entity.SetCORSHeaders; only the preflight
	// metadata (allowed methods and headers) is specific to this handler.
	entity.SetCORSHeaders(w)
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, HEAD, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	lookupRequested := isLookupRequest(r)
	operation := s.executor.FindOperation(r.Method, r.URL.EscapedPath())
	if operation == nil {
		if family, name, matched := s.matchDynamicFamily(r.URL.Path); matched {
			s.handleDynamicFamily(w, r, family, name)
			return
		}
	}
	if paged := s.pagedOperation(r, operation); paged != nil && !explicitLookup(r) {
		s.handlePagedCommand(w, r, paged, paged.PagedFunc, exportName(paged))
		return
	}
	if lookupRequested {
		if !hasLookup(operation) {
			operation = s.executor.FindLookupOperation(r.Method, r.URL.EscapedPath())
		}
		if !hasLookup(operation) {
			if s.structuredErrorResponses() {
				// The message is the legacy one verbatim, capital and all: the
				// envelope changes the shape a client parses, not the sentence.
				s.writeError(w, r, entity.NewStatusErrorf(http.StatusNotFound, "lookup_not_found",
					"No lookup found for %s %s", r.Method, r.URL.Path))
			} else {
				http.Error(w, fmt.Sprintf("No lookup found for %s %s", r.Method, r.URL.Path), http.StatusNotFound)
			}
			return
		}
		s.handleLookupCommand(w, r, operation)
		return
	}
	if operation == nil {
		if s.structuredErrorResponses() {
			s.writeError(w, r, entity.NewStatusErrorf(http.StatusNotFound, "operation_not_found",
				"No operation found for %s %s", r.Method, r.URL.Path))
		} else {
			http.Error(w, fmt.Sprintf("No operation found for %s %s", r.Method, r.URL.Path), http.StatusNotFound)
		}
		return
	}

	// Only the structured surface turns a failed execution into an error
	// response; the legacy one reports it inside the execution envelope, which
	// is the body every existing client parses.
	data, metadata, statusCode, err := s.executeCommandCore(r)
	if err != nil && s.structuredErrorResponses() {
		s.writeOperationError(w, r, statusCode, err)
		return
	}
	s.writeExecutionResult(w, r, data, metadata, statusCode)
}

func (s *SwaggerServer) writeExecutionResult(w http.ResponseWriter, r *http.Request, data any, metadata *ExecutionResponse, statusCode int) {
	if metadata != nil {
		w.Header().Set("X-CLI-Command", safeHeaderValue(logger.StripSecrets(metadata.CLI), 1024))
		w.Header().Set("X-Exit-Code", strconv.Itoa(metadata.ExitCode))
		w.Header().Set("X-Execution-Success", strconv.FormatBool(metadata.Success))
		// A failed execution reports itself in these headers and in the envelope
		// body, which is the whole diagnostic surface a client that has not opted
		// into structured errors has.
		if metadata.Error != "" {
			w.Header().Set("X-Error", safeHeaderValue(metadata.Error, entity.DefaultMaxErrorHeaderBytes))
		}
		if metadata.Stderr != "" {
			w.Header().Set("X-Stderr", safeHeaderValue(metadata.Stderr, entity.DefaultMaxErrorHeaderBytes))
		}
	}

	options := extractFormatOpts(r)
	body := data
	if isStructuredWireFormat(options.Format) && metadata != nil && !metadata.DataIsStructured {
		body = metadata
	}
	s.writeFormattedResponse(w, r, body, options, statusCode)
}

func isLookupRequest(r *http.Request) bool {
	return strings.EqualFold(r.Method, http.MethodHead) || r.URL.Query().Get("__lookup") == "filters"
}

func hasLookup(operation *RPCOperation) bool {
	return operation != nil && (operation.LookupFunc != nil || operation.ContextLookupFunc != nil)
}

func (s *SwaggerServer) handleLookupCommand(w http.ResponseWriter, r *http.Request, operation *RPCOperation) {
	request, err := s.executor.ExtractRequestFromHTTP(r, operation)
	if err != nil {
		if s.structuredErrorResponses() {
			s.writeStatusError(w, r, http.StatusBadRequest, "invalid_parameters", fmt.Errorf("extract parameters: %w", err))
		} else {
			http.Error(w, fmt.Sprintf("Failed to extract parameters: %v", err), http.StatusBadRequest)
		}
		return
	}
	if value := r.URL.Query().Get("__lookup_filter"); value != "" {
		request.Flags["__lookup_filter"] = value
	}
	if value := r.URL.Query().Get("__lookup_q"); value != "" {
		request.Flags["__lookup_q"] = value
	}
	for key, values := range r.URL.Query() {
		if strings.HasPrefix(key, "__") || len(values) == 0 {
			continue
		}
		if _, declared := request.Flags[key]; !declared {
			request.Flags[key] = values[0]
		}
	}

	var data any
	if operation.ContextLookupFunc != nil {
		data, err = operation.ContextLookupFunc(r.Context(), request.Flags, request.Args)
	} else {
		data, err = operation.LookupFunc(request.Flags, request.Args)
	}
	if err != nil {
		if s.structuredErrorResponses() {
			// writeError preserves a *entity.StatusError's status and defaults the
			// rest to 500: a lookup that failed in the backend is not the caller's 400.
			s.writeError(w, r, err)
		} else {
			http.Error(w, err.Error(), http.StatusBadRequest)
		}
		return
	}
	s.writeLookupResponse(w, r, data)
}

// writeLookupResponse writes a filter-lookup payload in the clicky JSON wire
// shape shared by registered operations and dynamic family instances.
func (s *SwaggerServer) writeLookupResponse(w http.ResponseWriter, r *http.Request, data any) {
	if s.structuredErrorResponses() {
		// Marshalled before the status line, so an encoding failure can still be
		// reported as an error instead of truncating a 200.
		w.Header().Set("Content-Type", "application/json+clicky")
		if strings.EqualFold(r.Method, http.MethodHead) {
			w.WriteHeader(http.StatusOK)
			return
		}
		body, err := json.Marshal(data)
		if err != nil {
			s.writeError(w, r, fmt.Errorf("encode lookup response: %w", err))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(append(body, '\n'))
		return
	}

	w.Header().Set("Content-Type", "application/json+clicky")
	w.WriteHeader(http.StatusOK)

	if strings.EqualFold(r.Method, http.MethodHead) {
		return
	}

	if err := json.NewEncoder(w).Encode(data); err != nil {
		http.Error(w, fmt.Sprintf("failed to encode lookup response: %v", err), http.StatusInternalServerError)
	}
}
