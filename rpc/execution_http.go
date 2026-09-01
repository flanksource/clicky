package rpc

import (
	"encoding/json"
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
		return data, metadata, http.StatusInternalServerError, err
	}
	return data, metadata, http.StatusOK, nil
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
				s.writeStatusError(w, r, http.StatusNotFound, "lookup_not_found",
					fmt.Errorf("No lookup found for %s %s", r.Method, r.URL.Path))
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
			s.writeStatusError(w, r, http.StatusNotFound, "operation_not_found",
				fmt.Errorf("No operation found for %s %s", r.Method, r.URL.Path))
		} else {
			http.Error(w, fmt.Sprintf("No operation found for %s %s", r.Method, r.URL.Path), http.StatusNotFound)
		}
		return
	}

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
		if !s.structuredErrorResponses() {
			if metadata.Error != "" {
				w.Header().Set("X-Error", metadata.Error)
			}
			if metadata.Stderr != "" {
				w.Header().Set("X-Stderr", metadata.Stderr)
			}
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
	if strings.EqualFold(r.Method, http.MethodHead) {
		w.WriteHeader(http.StatusOK)
		return
	}
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		http.Error(w, fmt.Sprintf("failed to encode lookup response: %v", err), http.StatusInternalServerError)
	}
}
