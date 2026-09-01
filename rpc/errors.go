package rpc

import (
	"crypto/rand"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/flanksource/clicky/entity"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

const rpcTracerName = "github.com/flanksource/clicky/rpc"

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
	safeError := errors.New(s.errorWriter.SafeMessage(err))
	span := trace.SpanFromContext(r.Context())
	span.RecordError(safeError)
	span.SetStatus(codes.Error, safeError.Error())
	s.errorWriter.Write(r.Context(), w, err)
}

func (s *SwaggerServer) writeStatusError(w http.ResponseWriter, r *http.Request, status int, code string, err error) {
	s.writeError(w, r, entity.NewStatusError(status, code, err.Error()))
}

// clientErrorMessage renders an error for a header or trailer, not for a body.
// It is bounded by the header cap rather than the body cap: a diagnostic sized
// for the JSON envelope would overflow what proxies accept in a header block and
// cost the whole response.
func (s *SwaggerServer) clientErrorMessage(err error) string {
	var statusError *entity.StatusError
	if s.config.HideErrorDetails && !errors.As(err, &statusError) {
		return entity.InternalErrorMessage
	}
	return safeHeaderValue(s.errorWriter.SafeMessage(err), entity.DefaultMaxErrorHeaderBytes)
}

func safeHeaderValue(value string, limit int) string {
	value = strings.ReplaceAll(value, "\r", " ")
	value = strings.ReplaceAll(value, "\n", " ")
	if len(value) > limit {
		value = value[:limit]
	}
	return value
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
