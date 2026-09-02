package rpc

import (
	"crypto/rand"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/flanksource/clicky/entity"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

const rpcTracerName = "github.com/flanksource/clicky/rpc"

func (s *SwaggerServer) structuredErrorResponses() bool {
	return s != nil && s.config != nil && s.config.StructuredErrorResponses
}

func (s *SwaggerServer) errorResponseWriter() *entity.ErrorWriter {
	if s.errorWriter != nil {
		return s.errorWriter
	}
	hideDetails := s.config != nil && s.config.HideErrorDetails
	return entity.NewErrorWriter(entity.ErrorOptions{HideDetails: hideDetails})
}

func (s *SwaggerServer) tracedHandler(route string, next http.Handler) http.Handler {
	if !s.structuredErrorResponses() {
		return next
	}
	return s.traceHandler(route, next)
}

func (s *SwaggerServer) traceHandler(route string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := propagation.TraceContext{}.Extract(r.Context(), propagation.HeaderCarrier(r.Header))
		if !trace.SpanContextFromContext(ctx).IsValid() {
			ctx = trace.ContextWithSpanContext(ctx, newServerSpanContext())
		}
		ctx, span := otel.Tracer(rpcTracerName).Start(ctx, route, trace.WithSpanKind(trace.SpanKindServer))
		defer span.End()

		spanContext := span.SpanContext()
		if !spanContext.IsValid() {
			spanContext = trace.SpanContextFromContext(ctx)
		}
		ctx = entity.ContextWithTraceID(ctx, spanContext.TraceID().String())
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *SwaggerServer) writeError(w http.ResponseWriter, r *http.Request, err error) {
	writer := s.errorResponseWriter()
	safeMessage := "nil error passed to SwaggerServer.writeError"
	if err != nil {
		safeMessage = writer.SafeMessage(err)
	}
	safeError := errors.New(safeMessage)
	span := trace.SpanFromContext(r.Context())
	span.RecordError(safeError)
	span.SetStatus(codes.Error, safeError.Error())
	writer.Write(r.Context(), w, err)
}

func (s *SwaggerServer) writeRPCError(w http.ResponseWriter, r *http.Request, err error) {
	if s.structuredErrorResponses() {
		s.writeError(w, r, err)
		return
	}
	entity.WriteError(w, err)
}

func (s *SwaggerServer) writeStatusError(w http.ResponseWriter, r *http.Request, status int, code string, err error) {
	s.writeError(w, r, entity.NewStatusError(status, code, err.Error()))
}

func (s *SwaggerServer) writeOperationError(w http.ResponseWriter, r *http.Request, statusCode int, err error) {
	if statusCode >= http.StatusInternalServerError {
		s.writeError(w, r, err)
		return
	}
	code := "operation_failed"
	switch statusCode {
	case http.StatusBadRequest:
		code = "invalid_request"
	case http.StatusNotFound:
		code = "operation_not_found"
	case http.StatusConflict:
		code = "conflict"
	}
	s.writeStatusError(w, r, statusCode, code, err)
}

func (s *SwaggerServer) clientErrorMessage(err error) string {
	if !s.structuredErrorResponses() {
		return err.Error()
	}
	var statusError *entity.StatusError
	if s.config.HideErrorDetails && !errors.As(err, &statusError) {
		return entity.InternalErrorMessage
	}
	return safeHeaderValue(s.errorResponseWriter().SafeMessage(err), entity.DefaultMaxErrorHeaderBytes)
}

func safeHeaderValue(value string, limit int) string {
	value = strings.ToValidUTF8(value, "�")
	value = strings.ReplaceAll(value, "\r", " ")
	value = strings.ReplaceAll(value, "\n", " ")
	if limit <= 0 || len(value) <= limit {
		return value
	}
	for limit > 0 && !utf8.RuneStart(value[limit]) {
		limit--
	}
	return value[:limit]
}

func newServerSpanContext() trace.SpanContext {
	var value [24]byte
	if _, err := rand.Read(value[:]); err != nil {
		panic(fmt.Sprintf("generate server trace context: %v", err))
	}
	var traceID trace.TraceID
	var spanID trace.SpanID
	copy(traceID[:], value[:16])
	copy(spanID[:], value[16:])
	return trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: traceID, SpanID: spanID, TraceFlags: trace.FlagsSampled,
	})
}
