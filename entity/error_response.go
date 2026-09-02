package entity

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"github.com/flanksource/commons/logger"
)

const (
	ErrorTraceHeader = "X-Trace-ID"
	HideErrorsEnv    = "HIDE_ERRORS"

	DefaultMaxErrorDetailBytes   = 64 * 1024
	DefaultMaxErrorResponseBytes = 1024 * 1024
	DefaultMaxErrorHeaderBytes   = 2 * 1024

	defaultInternalErrorCode = "internal_error"
)

type ErrorDetail struct {
	Label       string `json:"label"`
	Value       string `json:"value"`
	ContentType string `json:"content_type"`
	Truncated   bool   `json:"truncated,omitempty"`
}

type ErrorResponse struct {
	Code       string         `json:"code"`
	Message    string         `json:"message"`
	Trace      string         `json:"trace"`
	Hint       string         `json:"hint,omitempty"`
	Context    map[string]any `json:"context,omitempty"`
	Details    []ErrorDetail  `json:"details,omitempty"`
	Stacktrace string         `json:"stacktrace,omitempty"`
	Truncated  bool           `json:"truncated,omitempty"`
}

type ErrorOptions struct {
	HideDetails      bool
	MaxDetailBytes   int
	MaxResponseBytes int
}

type ErrorWriter struct {
	options ErrorOptions
}

type errorTraceContextKey struct{}

type diagnosticError interface {
	error
	Context() map[string]any
	Hint() string
	Public() string
	Stacktrace() string
	Trace() string
}

var fallbackTraceSequence atomic.Uint64

func NewErrorWriter(options ErrorOptions) *ErrorWriter {
	if options.MaxDetailBytes <= 0 {
		options.MaxDetailBytes = DefaultMaxErrorDetailBytes
	}
	if options.MaxResponseBytes <= 0 {
		options.MaxResponseBytes = DefaultMaxErrorResponseBytes
	}
	if !options.HideDetails && strings.EqualFold(os.Getenv(HideErrorsEnv), "true") {
		options.HideDetails = true
	}
	return &ErrorWriter{options: options}
}

func ContextWithTraceID(ctx context.Context, traceID string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, errorTraceContextKey{}, traceID)
}

func TraceIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	traceID, _ := ctx.Value(errorTraceContextKey{}).(string)
	return traceID
}

func (e *ErrorWriter) Write(ctx context.Context, w http.ResponseWriter, err error) {
	if err == nil {
		err = errors.New("nil error passed to ErrorWriter.Write")
	}
	status, response, logMessage := e.response(ctx, err)
	body := e.marshal(response)

	SetCORSHeaders(w)
	exposeHeader(w.Header(), ErrorTraceHeader)
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set(ErrorTraceHeader, response.Trace)
	w.WriteHeader(status)
	_, _ = w.Write(body)

	if status >= http.StatusInternalServerError {
		logger.Errorf("handler error trace=%s: %s", response.Trace, logMessage)
	}
}

func (e *ErrorWriter) SafeMessage(err error) string {
	if err == nil {
		return InternalErrorMessage
	}
	message, _, _ := sanitizeErrorText(safeErrorMessage(err), e.options.MaxDetailBytes)
	return message
}

func (e *ErrorWriter) response(ctx context.Context, err error) (int, ErrorResponse, string) {
	traceID := sanitizeTraceID(TraceIDFromContext(ctx))
	var diagnostics diagnosticError
	if errors.As(err, &diagnostics) && diagnostics != nil && traceID == "" {
		traceID = sanitizeTraceID(diagnostics.Trace())
	}
	if traceID == "" {
		traceID = newTraceID()
	}

	var statusError *StatusError
	if errors.As(err, &statusError) && statusError != nil {
		message, _, truncated := sanitizeErrorText(statusError.Message, e.options.MaxDetailBytes)
		return statusError.StatusCode(), ErrorResponse{
			Code: statusError.Code, Message: message, Trace: traceID, Truncated: truncated,
		}, message
	}

	logMessage, _, messageTruncated := sanitizeErrorText(safeErrorMessage(err), e.options.MaxDetailBytes)
	response := ErrorResponse{
		Code: defaultInternalErrorCode, Message: logMessage, Trace: traceID, Truncated: messageTruncated,
	}
	if e.options.HideDetails {
		response.Message = InternalErrorMessage
		response.Truncated = false
		return http.StatusInternalServerError, response, logMessage
	}
	if diagnostics != nil {
		e.addDiagnostics(&response, diagnostics)
	}
	return http.StatusInternalServerError, response, logMessage
}

func (e *ErrorWriter) addDiagnostics(response *ErrorResponse, diagnostics diagnosticError) {
	response.Hint, _, response.Truncated = mergeSanitizedText(
		response.Truncated, diagnostics.Hint(), e.options.MaxDetailBytes,
	)
	response.Stacktrace, _, response.Truncated = mergeSanitizedText(
		response.Truncated, diagnostics.Stacktrace(), e.options.MaxDetailBytes,
	)

	contextValues := diagnostics.Context()
	keys := make([]string, 0, len(contextValues))
	for key := range contextValues {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		value := contextValues[key]
		if isBodyDiagnostic(key, value) {
			detail := sanitizeErrorDetail(key, value, e.options.MaxDetailBytes)
			response.Details = append(response.Details, detail)
			response.Truncated = response.Truncated || detail.Truncated
			continue
		}
		if response.Context == nil {
			response.Context = make(map[string]any)
		}
		response.Context[key] = sanitizeDiagnosticValue(key, value, e.options.MaxDetailBytes)
	}
}

func mergeSanitizedText(truncated bool, value string, limit int) (string, string, bool) {
	value, contentType, valueTruncated := sanitizeErrorText(value, limit)
	return value, contentType, truncated || valueTruncated
}

func (e *ErrorWriter) marshal(input ErrorResponse) []byte {
	response := cloneErrorResponse(input)
	if body, ok := e.marshalWithinLimit(response); ok {
		return body
	}

	response.Truncated = true
	response.Stacktrace = ""
	if body, ok := e.marshalWithinLimit(response); ok {
		return body
	}

	keys := make([]string, 0, len(response.Context))
	for key := range response.Context {
		keys = append(keys, key)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(keys)))
	for _, key := range keys {
		delete(response.Context, key)
		if body, ok := e.marshalWithinLimit(response); ok {
			return body
		}
	}
	response.Context = nil

	for len(response.Details) > 0 {
		response.Details = response.Details[:len(response.Details)-1]
		if body, ok := e.marshalWithinLimit(response); ok {
			return body
		}
	}

	response.Message, _ = truncateUTF8(response.Message, e.options.MaxResponseBytes/2)
	if body, ok := e.marshalWithinLimit(response); ok {
		return body
	}

	minimal := ErrorResponse{
		Code: response.Code, Message: InternalErrorMessage, Trace: response.Trace, Truncated: true,
	}
	if body, ok := e.marshalWithinLimit(minimal); ok {
		return body
	}
	body, err := json.Marshal(minimal)
	if err == nil {
		return body
	}
	return []byte(`{"code":"internal_error","message":"internal server error","trace":"","truncated":true}`)
}

func (e *ErrorWriter) marshalWithinLimit(response ErrorResponse) ([]byte, bool) {
	body, err := json.Marshal(response)
	return body, err == nil && len(body) <= e.options.MaxResponseBytes
}

func cloneErrorResponse(response ErrorResponse) ErrorResponse {
	if response.Context != nil {
		cloned := make(map[string]any, len(response.Context))
		for key, value := range response.Context {
			cloned[key] = value
		}
		response.Context = cloned
	}
	response.Details = append([]ErrorDetail(nil), response.Details...)
	return response
}

func exposeHeader(header http.Header, name string) {
	for _, exposed := range strings.Split(header.Get("Access-Control-Expose-Headers"), ",") {
		if strings.EqualFold(strings.TrimSpace(exposed), name) {
			return
		}
	}
	if header.Get("Access-Control-Expose-Headers") == "" {
		header.Set("Access-Control-Expose-Headers", name)
		return
	}
	header.Set("Access-Control-Expose-Headers", header.Get("Access-Control-Expose-Headers")+", "+name)
}

func sanitizeTraceID(traceID string) string {
	traceID = strings.Map(func(r rune) rune {
		if r == '\r' || r == '\n' || r == 0 {
			return -1
		}
		return r
	}, logger.StripSecrets(traceID))
	traceID, _ = truncateUTF8(traceID, 128)
	return traceID
}

func safeErrorMessage(err error) (message string) {
	defer func() {
		if recover() != nil {
			message = InternalErrorMessage
		}
	}()
	return err.Error()
}

func newTraceID() string {
	var value [16]byte
	if _, err := rand.Read(value[:]); err == nil {
		return hex.EncodeToString(value[:])
	}
	fallback := sha256.Sum256([]byte(fmt.Sprintf("%d:%d", time.Now().UnixNano(), fallbackTraceSequence.Add(1))))
	return hex.EncodeToString(fallback[:16])
}
