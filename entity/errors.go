package entity

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"sort"

	"github.com/flanksource/commons/logger"
)

const (
	InternalErrorMessage = "internal server error"
	ErrorTraceHeader     = "X-Trace-ID"

	// DefaultMaxErrorDetailBytes bounds each JSON-body diagnostic: the
	// stacktrace and every ErrorDetail value. It is sized for a real language
	// stack trace rather than a log line — a Java printStackTrace() with several
	// "Caused by:" chains runs tens of kilobytes, and because truncation cuts the
	// tail, a small bound removes precisely the innermost cause that names the
	// failure.
	DefaultMaxErrorDetailBytes = 64 * 1024
	// DefaultMaxErrorResponseBytes bounds the whole marshalled envelope, which
	// must hold a full stacktrace plus a body detail plus context with headroom.
	DefaultMaxErrorResponseBytes = 1024 * 1024
	// DefaultMaxErrorHeaderBytes bounds error text copied into an HTTP header or
	// trailer. It is deliberately far smaller than the body caps: intermediaries
	// (nginx, envoy) commonly refuse total header blocks beyond 8-16KB, so a
	// header carrying a body-sized diagnostic would fail the whole response
	// rather than truncate it.
	DefaultMaxErrorHeaderBytes = 2 * 1024

	defaultInternalErrorCode        = "internal_error"
	defaultErrorResponseContentType = "application/json"
)

type StatusError struct {
	Status int `json:"-"`

	Code    string `json:"code"`
	Message string `json:"message"`
}

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

func NewStatusError(status int, code, message string) *StatusError {
	return &StatusError{Status: status, Code: code, Message: message}
}

func NewStatusErrorf(status int, code, format string, args ...any) *StatusError {
	return &StatusError{Status: status, Code: code, Message: fmt.Sprintf(format, args...)}
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *StatusError) StatusCode() int {
	if e.Status == 0 {
		return http.StatusInternalServerError
	}
	return e.Status
}

func (e *StatusError) Write(w http.ResponseWriter) {
	NewErrorWriter(ErrorOptions{}).Write(context.Background(), w, e)
}

// HideErrorsEnv hides unclassified error details from clients when set to
// "true", for a deployment that wants the opaque body without threading a config
// change through whatever constructs its writer. It can only tighten: a caller
// that already asked to hide details is unaffected, and this can never reveal
// details a caller chose to hide.
const HideErrorsEnv = "HIDE_ERRORS"

func NewErrorWriter(options ErrorOptions) *ErrorWriter {
	if options.MaxDetailBytes <= 0 {
		options.MaxDetailBytes = DefaultMaxErrorDetailBytes
	}
	if options.MaxResponseBytes <= 0 {
		options.MaxResponseBytes = DefaultMaxErrorResponseBytes
	}
	// Read here rather than at the transport, so the package-level WriteError
	// honours it too — it is the writer every unclassified failure passes
	// through, whether or not a server was configured at all.
	if !options.HideDetails && os.Getenv(HideErrorsEnv) == "true" {
		options.HideDetails = true
	}
	return &ErrorWriter{options: options}
}

func ContextWithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, errorTraceContextKey{}, traceID)
}

func TraceIDFromContext(ctx context.Context) string {
	traceID, _ := ctx.Value(errorTraceContextKey{}).(string)
	return traceID
}

func (e *ErrorWriter) Write(ctx context.Context, w http.ResponseWriter, err error) {
	if err == nil {
		// A nil error is a caller bug, but the error path must not replace a
		// handled failure with a crashed request: degrade to a traceable 500.
		err = errors.New("nil error passed to ErrorWriter.Write")
	}
	status, response, logMessage := e.response(ctx, err)
	body := e.marshal(response)

	SetCORSHeaders(w)
	w.Header().Set("Content-Type", defaultErrorResponseContentType)
	w.Header().Set(ErrorTraceHeader, response.Trace)
	w.WriteHeader(status)
	_, _ = w.Write(body)

	if status >= http.StatusInternalServerError {
		logger.Errorf("handler error trace=%s: %s", response.Trace, logMessage)
	}
}

func (e *ErrorWriter) SafeMessage(err error) string {
	message, _, _ := sanitizeErrorText(err.Error(), e.options.MaxDetailBytes)
	return message
}

func (e *ErrorWriter) response(ctx context.Context, err error) (int, ErrorResponse, string) {
	traceID := TraceIDFromContext(ctx)
	var diagnostics diagnosticError
	if errors.As(err, &diagnostics) && traceID == "" {
		traceID = diagnostics.Trace()
	}
	if traceID == "" {
		traceID = newTraceID()
	}

	var statusError *StatusError
	if errors.As(err, &statusError) {
		message, _, truncated := sanitizeErrorText(statusError.Message, e.options.MaxDetailBytes)
		return statusError.StatusCode(), ErrorResponse{
			Code: statusError.Code, Message: message, Trace: traceID, Truncated: truncated,
		}, message
	}

	logMessage, _, messageTruncated := sanitizeErrorText(err.Error(), e.options.MaxDetailBytes)
	response := ErrorResponse{Code: defaultInternalErrorCode, Message: logMessage, Trace: traceID, Truncated: messageTruncated}
	if e.options.HideDetails {
		response.Message = InternalErrorMessage
		if diagnostics != nil && diagnostics.Public() != "" {
			response.Message, _, response.Truncated = sanitizeErrorText(diagnostics.Public(), e.options.MaxDetailBytes)
		}
		return http.StatusInternalServerError, response, logMessage
	}
	if diagnostics != nil {
		e.addDiagnostics(&response, diagnostics)
	}
	return http.StatusInternalServerError, response, logMessage
}

func (e *ErrorWriter) addDiagnostics(response *ErrorResponse, diagnostics diagnosticError) {
	response.Hint, _, response.Truncated = mergeSanitizedText(response.Truncated, diagnostics.Hint(), e.options.MaxDetailBytes)
	response.Stacktrace, _, response.Truncated = mergeSanitizedText(response.Truncated, diagnostics.Stacktrace(), e.options.MaxDetailBytes)

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

// marshal bounds the envelope by dropping diagnostics in fixed order —
// stacktrace, context entries, details, then the message — re-checking the
// size after every removal so nothing is dropped beyond what the bound
// requires. It never panics: a response that cannot be marshalled or bounded
// (e.g. an unmarshalable context value, or a MaxResponseBytes below the
// code+message+trace floor) degrades to a minimal always-marshalable envelope.
func (e *ErrorWriter) marshal(response ErrorResponse) []byte {
	if body, ok := e.marshalWithinLimit(response); ok {
		return body
	}

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
	var messageTruncated bool
	response.Message, messageTruncated = truncateUTF8(response.Message, e.options.MaxResponseBytes/2)
	response.Truncated = response.Truncated || messageTruncated
	if body, ok := e.marshalWithinLimit(response); ok {
		return body
	}
	// The code, a fixed message, and the trace are the envelope floor: they
	// always marshal, and a MaxResponseBytes below the floor is served at the
	// floor rather than crashing the request that is already reporting an error.
	body, _ := json.Marshal(ErrorResponse{
		Code: response.Code, Message: InternalErrorMessage, Trace: response.Trace, Truncated: true,
	})
	return body
}

func (e *ErrorWriter) marshalWithinLimit(response ErrorResponse) ([]byte, bool) {
	body, err := json.Marshal(response)
	return body, err == nil && len(body) <= e.options.MaxResponseBytes
}

func newTraceID() string {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		panic(fmt.Sprintf("generate error trace ID: %v", err))
	}
	return hex.EncodeToString(value[:])
}

func WriteError(w http.ResponseWriter, err error) {
	NewErrorWriter(ErrorOptions{}).Write(context.Background(), w, err)
}
