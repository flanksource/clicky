package entity

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/samber/oops"
)

var _ = Describe("StatusError", func() {
	It("renders only the two fields a client branches on before transport metadata is added", func() {
		// The frontend reads code and switches on it; anything else in the body
		// is a field it would have to learn to ignore. The status belongs to the
		// status line, so it must not appear here and be able to disagree.
		body, err := json.Marshal(NewStatusError(http.StatusConflict, "connection_required", "select a connection"))

		Expect(err).ToNot(HaveOccurred())
		Expect(string(body)).To(Equal(`{"code":"connection_required","message":"select a connection"}`))
	})

	DescribeTable("writes its code at its status",
		func(status int, code, message string) {
			w := httptest.NewRecorder()
			NewStatusError(status, code, message).Write(w)

			Expect(w.Code).To(Equal(status))
			Expect(w.Header().Get("Content-Type")).To(Equal("application/json"))

			var body ErrorResponse
			Expect(json.Unmarshal(w.Body.Bytes(), &body)).To(Succeed())
			Expect(body.Code).To(Equal(code))
			Expect(body.Message).To(Equal(message))
			Expect(body.Trace).To(BeEmpty())
			Expect(w.Header().Get(ErrorTraceHeader)).To(BeEmpty())
		},
		Entry("a stale cursor", http.StatusBadRequest, "cursor_stale", "the cursor no longer resolves"),
		Entry("an unknown profile", http.StatusNotFound, "profile_not_found", `profile "invoices" not found`),
		Entry("a missing connection", http.StatusConflict, "connection_required", "select a connection"),
		Entry("a refused representation", http.StatusNotAcceptable, "not_acceptable", "no acceptable representation"),
	)

	It("reports its code and message when read as a plain error", func() {
		Expect(NewStatusErrorf(http.StatusNotFound, "profile_not_found", "profile %q not found", "invoices").Error()).
			To(Equal(`profile_not_found: profile "invoices" not found`))
	})

	It("falls back to 500 rather than writing an invalid status line", func() {
		w := httptest.NewRecorder()
		(&StatusError{Code: "unset", Message: "no status was stated"}).Write(w)

		Expect(w.Code).To(Equal(http.StatusInternalServerError))
	})

	It("survives wrapping so a handler can add context to it", func() {
		w := httptest.NewRecorder()
		WriteError(w, fmt.Errorf("export invoices: %w", NewStatusError(http.StatusNotFound, "profile_not_found", "no such profile")))

		Expect(w.Code).To(Equal(http.StatusNotFound))
		var body ErrorResponse
		Expect(json.Unmarshal(w.Body.Bytes(), &body)).To(Succeed())
		Expect(body.Code).To(Equal("profile_not_found"))
	})

	It("returns sanitized unclassified details by default", func() {
		w := httptest.NewRecorder()
		NewErrorWriter(ErrorOptions{}).Write(context.Background(), w,
			errors.New("request failed: token=supersecret12345"))

		Expect(w.Code).To(Equal(http.StatusInternalServerError))
		var body ErrorResponse
		Expect(json.Unmarshal(w.Body.Bytes(), &body)).To(Succeed())
		Expect(body.Code).To(Equal("internal_error"))
		Expect(body.Message).To(ContainSubstring("request failed"))
		Expect(w.Body.String()).ToNot(ContainSubstring("supersecret12345"))
	})

	It("makes hiding unclassified details opt-in without hiding classified errors", func() {
		writer := NewErrorWriter(ErrorOptions{HideDetails: true})

		unclassified := httptest.NewRecorder()
		writer.Write(context.Background(), unclassified, diagnosticFixture{
			message: "dial tcp 10.0.0.7:5432: connection refused",
			hint:    "inspect the private service", stack: "private stack",
			context: map[string]any{"request_body": `{"private":"value"}`, "host": "10.0.0.7"},
		})
		var hidden ErrorResponse
		Expect(json.Unmarshal(unclassified.Body.Bytes(), &hidden)).To(Succeed())
		Expect(hidden.Message).To(Equal(InternalErrorMessage))
		Expect(hidden.Hint).To(BeEmpty())
		Expect(hidden.Context).To(BeEmpty())
		Expect(hidden.Details).To(BeEmpty())
		Expect(hidden.Stacktrace).To(BeEmpty())
		Expect(unclassified.Body.String()).ToNot(ContainSubstring("10.0.0.7"))

		classified := httptest.NewRecorder()
		writer.Write(context.Background(), classified, NewStatusError(http.StatusBadRequest, "invalid_filter", "column is required"))
		var visible ErrorResponse
		Expect(json.Unmarshal(classified.Body.Bytes(), &visible)).To(Succeed())
		Expect(visible.Message).To(Equal("column is required"))
	})

	It("hides unclassified details when HIDE_ERRORS is set", func() {
		GinkgoT().Setenv(HideErrorsEnv, "true")

		w := httptest.NewRecorder()
		NewErrorWriter(ErrorOptions{}).Write(
			context.Background(), w, errors.New("dial tcp 10.0.0.7:5432: connection refused"),
		)

		var body ErrorResponse
		Expect(json.Unmarshal(w.Body.Bytes(), &body)).To(Succeed())
		Expect(body.Message).To(Equal(InternalErrorMessage))
		Expect(body.Trace).ToNot(BeEmpty(), "an opaque body still has to be traceable to its log line")
		Expect(w.Body.String()).ToNot(ContainSubstring("10.0.0.7"))
	})

	It("reports unclassified details when HIDE_ERRORS is unset", func() {
		w := httptest.NewRecorder()
		NewErrorWriter(ErrorOptions{}).Write(
			context.Background(), w, errors.New("dial tcp 10.0.0.7:5432: connection refused"),
		)

		var body ErrorResponse
		Expect(json.Unmarshal(w.Body.Bytes(), &body)).To(Succeed())
		Expect(body.Message).To(ContainSubstring("connection refused"))
	})

	It("cannot be reopened by the environment once a caller has hidden details", func() {
		GinkgoT().Setenv(HideErrorsEnv, "false")
		writer := NewErrorWriter(ErrorOptions{HideDetails: true})

		w := httptest.NewRecorder()
		writer.Write(context.Background(), w, errors.New("dial tcp 10.0.0.7:5432: connection refused"))

		var body ErrorResponse
		Expect(json.Unmarshal(w.Body.Bytes(), &body)).To(Succeed())
		Expect(body.Message).To(Equal(InternalErrorMessage))
	})

	It("uses one request trace in the response header and body", func() {
		const traceID = "0123456789abcdef0123456789abcdef"
		w := httptest.NewRecorder()
		NewErrorWriter(ErrorOptions{}).Write(ContextWithTraceID(context.Background(), traceID), w, errors.New("boom"))

		var body ErrorResponse
		Expect(json.Unmarshal(w.Body.Bytes(), &body)).To(Succeed())
		Expect(body.Trace).To(Equal(traceID))
		Expect(w.Header().Get(ErrorTraceHeader)).To(Equal(traceID))
		Expect(w.Header().Get("Access-Control-Expose-Headers")).To(ContainSubstring(ErrorTraceHeader))
	})

	It("flattens and sanitizes Oops diagnostics while classifying body values", func() {
		err := oops.With(
			"request_body", `{"name":"acme","password":"correct-horse-battery-staple"}`,
			"token", "supersecret12345",
			"attempt", 3,
		).Trace("fedcba9876543210fedcba9876543210").Hint("check the upstream response").Errorf("request failed")
		w := httptest.NewRecorder()

		NewErrorWriter(ErrorOptions{}).Write(context.Background(), w, err)

		var body ErrorResponse
		Expect(json.Unmarshal(w.Body.Bytes(), &body)).To(Succeed())
		Expect(body.Trace).To(Equal("fedcba9876543210fedcba9876543210"))
		Expect(body.Hint).To(Equal("check the upstream response"))
		Expect(body.Context).To(HaveKeyWithValue("attempt", float64(3)))
		Expect(fmt.Sprint(body.Context["token"])).ToNot(ContainSubstring("supersecret12345"))
		Expect(body.Details).To(HaveLen(1))
		Expect(body.Details[0].ContentType).To(Equal("application/json"))
		Expect(body.Details[0].Value).To(ContainSubstring(`"name":"acme"`))
		Expect(body.Details[0].Value).ToNot(ContainSubstring("correct-horse-battery-staple"))
	})

	It("detects binary diagnostic bodies and bounds details and the full envelope", func() {
		diagnostics := []any{
			"response_body", append([]byte{0x00, 0x01, 0x02}, []byte(strings.Repeat("x", 70*1024))...),
		}
		for index := range 20 {
			diagnostics = append(diagnostics, fmt.Sprintf("field_%02d", index), strings.Repeat("context", 1024))
		}
		err := oops.With(diagnostics...).Errorf("%s", strings.Repeat("failure ", 2048))
		w := httptest.NewRecorder()

		NewErrorWriter(ErrorOptions{}).Write(context.Background(), w, err)

		var body ErrorResponse
		Expect(json.Unmarshal(w.Body.Bytes(), &body)).To(Succeed())
		Expect(body.Truncated).To(BeTrue())
		Expect(body.Details).To(HaveLen(1))
		Expect(body.Details[0].ContentType).To(Equal("application/octet-stream"))
		Expect(body.Details[0].Value).To(ContainSubstring("binary"))
		Expect(w.Body.Len()).To(BeNumerically("<=", DefaultMaxErrorResponseBytes))
	})

	DescribeTable("detects and sanitizes textual diagnostic body types",
		func(raw, contentType, secret string) {
			w := httptest.NewRecorder()
			NewErrorWriter(ErrorOptions{}).Write(context.Background(), w,
				oops.With("request_body", raw).Errorf("request failed"))

			var body ErrorResponse
			Expect(json.Unmarshal(w.Body.Bytes(), &body)).To(Succeed())
			Expect(body.Details).To(HaveLen(1))
			Expect(body.Details[0].ContentType).To(Equal(contentType))
			Expect(body.Details[0].Value).ToNot(ContainSubstring(secret))
		},
		Entry("form", "username=alice&token=supersecret12345", "application/x-www-form-urlencoded", "supersecret12345"),
		Entry("plain text", "backend says password=correct-horse-battery-staple", "text/plain", "correct-horse-battery-staple"),
	)

	// A language stack trace is truncated from the tail, and its innermost
	// "Caused by:" — the line that actually names the failure — is the last
	// thing in it. A detail cap smaller than a real stack therefore drops the
	// only part worth reading, so the cap is asserted against a realistic size.
	It("keeps the innermost cause of a stack trace larger than a log line", func() {
		const rootCause = "Caused by: EngineException: Could not find definition for variable: Policy:CommencementDate"
		stack := "java.lang.RuntimeException: Exception in doMath()\n" +
			strings.Repeat("\tat com.acme.pas.uip.PolicyActivityUip.processProcessAction(PolicyActivityUip.java:1032)\n", 200) +
			rootCause
		Expect(len(stack)).To(BeNumerically(">", 4*1024), "the fixture must exceed the old 4KB cap to be meaningful")

		w := httptest.NewRecorder()
		NewErrorWriter(ErrorOptions{}).Write(context.Background(), w, stackTraceError{stack: stack})

		var body ErrorResponse
		Expect(json.Unmarshal(w.Body.Bytes(), &body)).To(Succeed())
		Expect(body.Stacktrace).To(HaveSuffix(rootCause))
		Expect(body.Truncated).To(BeFalse())

		// The cap is what decides this, not the writer: at the old 4KB bound the
		// same stack loses the only line that explains the failure.
		narrow := httptest.NewRecorder()
		NewErrorWriter(ErrorOptions{MaxDetailBytes: 4 * 1024}).
			Write(context.Background(), narrow, stackTraceError{stack: stack})

		var narrowed ErrorResponse
		Expect(json.Unmarshal(narrow.Body.Bytes(), &narrowed)).To(Succeed())
		Expect(narrowed.Stacktrace).ToNot(ContainSubstring(rootCause))
		Expect(narrowed.Truncated).To(BeTrue())
	})

	It("keeps non-ASCII diagnostics printable when truncating", func() {
		w := httptest.NewRecorder()
		value := string(append([]byte("caf\xe9-"), []byte(strings.Repeat("x", 64))...))
		NewErrorWriter(ErrorOptions{MaxDetailBytes: 32}).Write(context.Background(), w,
			diagnosticFixture{message: "failed", context: map[string]any{"request_body": value}})

		var body ErrorResponse
		Expect(json.Unmarshal(w.Body.Bytes(), &body)).To(Succeed())
		Expect(body.Details).To(HaveLen(1))
		Expect(body.Details[0].Value).To(ContainSubstring("caf"))
		Expect(body.Details[0].Value).To(HaveSuffix(truncationMarker))
	})

	It("applies the configured detail limit to nested byte diagnostics", func() {
		w := httptest.NewRecorder()
		NewErrorWriter(ErrorOptions{MaxDetailBytes: 32}).Write(context.Background(), w,
			diagnosticFixture{message: "failed", context: map[string]any{
				"metadata": map[string]any{"raw": []byte(strings.Repeat("x", 128))},
			}})

		var body ErrorResponse
		Expect(json.Unmarshal(w.Body.Bytes(), &body)).To(Succeed())
		metadata := body.Context["metadata"].(map[string]any)
		Expect(metadata["raw"]).To(HaveLen(32))
	})

	It("writes a minimal envelope for nil errors and tiny configured responses", func() {
		for _, err := range []error{nil, errors.New(strings.Repeat("failure ", 128))} {
			w := httptest.NewRecorder()
			Expect(func() {
				NewErrorWriter(ErrorOptions{MaxResponseBytes: 128}).Write(context.Background(), w, err)
			}).ToNot(Panic())
			Expect(w.Code).To(Equal(http.StatusInternalServerError))
			var body ErrorResponse
			Expect(json.Unmarshal(w.Body.Bytes(), &body)).To(Succeed())
			Expect(body.Trace).To(MatchRegexp(`^[0-9a-f]{32}$`))
		}
	})

	It("rechecks the response size before dropping the next diagnostic field", func() {
		response := ErrorResponse{
			Code: "internal_error", Message: "message must survive", Trace: strings.Repeat("a", 32),
			Stacktrace: strings.Repeat("s", 256), Context: map[string]any{"keep": "context"},
		}
		withoutStack := response
		withoutStack.Stacktrace = ""
		withoutStack.Truncated = true
		capBody, err := json.Marshal(withoutStack)
		Expect(err).ToNot(HaveOccurred())

		body := NewErrorWriter(ErrorOptions{MaxResponseBytes: len(capBody)}).marshal(response)
		var bounded ErrorResponse
		Expect(json.Unmarshal(body, &bounded)).To(Succeed())
		Expect(bounded.Context).To(HaveKeyWithValue("keep", "context"))
		Expect(bounded.Message).To(Equal("message must survive"))
	})
})

// stackTraceError is a diagnosticError whose stack size is under the test's
// control — oops derives its own from live Go frames, which cannot be grown to
// the size a JVM trace reaches.
type stackTraceError struct{ stack string }

func (e stackTraceError) Error() string           { return "process failed" }
func (e stackTraceError) Context() map[string]any { return nil }
func (e stackTraceError) Hint() string            { return "" }
func (e stackTraceError) Public() string          { return "" }
func (e stackTraceError) Stacktrace() string      { return e.stack }
func (e stackTraceError) Trace() string           { return "" }

type diagnosticFixture struct {
	message string
	hint    string
	stack   string
	context map[string]any
}

func (e diagnosticFixture) Error() string           { return e.message }
func (e diagnosticFixture) Context() map[string]any { return e.context }
func (e diagnosticFixture) Hint() string            { return e.hint }
func (e diagnosticFixture) Public() string          { return "" }
func (e diagnosticFixture) Stacktrace() string      { return e.stack }
func (e diagnosticFixture) Trace() string           { return "" }
