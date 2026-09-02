package entity

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"unicode/utf8"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ErrorWriter", func() {
	It("writes classified errors as traceable envelopes", func() {
		const traceID = "0123456789abcdef0123456789abcdef"
		w := httptest.NewRecorder()

		NewErrorWriter(ErrorOptions{}).Write(
			ContextWithTraceID(context.Background(), traceID),
			w,
			NewStatusError(http.StatusBadRequest, "invalid_filter", "column is required"),
		)

		Expect(w.Code).To(Equal(http.StatusBadRequest))
		Expect(w.Header().Get("Content-Type")).To(Equal("application/json"))
		Expect(w.Header().Get(ErrorTraceHeader)).To(Equal(traceID))
		Expect(w.Header().Get("Access-Control-Expose-Headers")).To(ContainSubstring(ErrorTraceHeader))
		Expect(decodeErrorResponse(w)).To(Equal(ErrorResponse{
			Code: "invalid_filter", Message: "column is required", Trace: traceID,
		}))
	})

	It("shows sanitized diagnostics by default", func() {
		w := httptest.NewRecorder()
		NewErrorWriter(ErrorOptions{}).Write(context.Background(), w, diagnosticTestError{
			message: "request failed: token=supersecret12345",
			hint:    "inspect token=supersecret12345",
			stack:   "stack password=correct-horse-battery-staple",
			trace:   "fedcba9876543210fedcba9876543210",
			context: map[string]any{
				"attempt":       3,
				"token":         "supersecret12345",
				"request_body":  `{"name":"acme","password":"correct-horse-battery-staple"}`,
				"response_body": []byte{0x00, 0x01, 0x02},
			},
		})

		response := decodeErrorResponse(w)
		Expect(response.Code).To(Equal("internal_error"))
		Expect(response.Message).To(ContainSubstring("request failed"))
		Expect(response.Trace).To(Equal("fedcba9876543210fedcba9876543210"))
		Expect(response.Hint).To(ContainSubstring("inspect"))
		Expect(response.Stacktrace).To(ContainSubstring("stack"))
		Expect(response.Context).To(HaveKeyWithValue("attempt", float64(3)))
		Expect(response.Details).To(HaveLen(2))
		Expect(w.Body.String()).ToNot(ContainSubstring("supersecret12345"))
		Expect(w.Body.String()).ToNot(ContainSubstring("correct-horse-battery-staple"))
	})

	It("hides every unclassified diagnostic when requested", func() {
		w := httptest.NewRecorder()
		NewErrorWriter(ErrorOptions{HideDetails: true}).Write(context.Background(), w, diagnosticTestError{
			message: "private failure",
			public:  "public diagnostic",
			hint:    "private hint",
			stack:   "private stack",
			context: map[string]any{"host": "10.0.0.7", "request_body": "private body"},
		})

		response := decodeErrorResponse(w)
		Expect(response.Message).To(Equal(InternalErrorMessage))
		Expect(response.Hint).To(BeEmpty())
		Expect(response.Context).To(BeEmpty())
		Expect(response.Details).To(BeEmpty())
		Expect(response.Stacktrace).To(BeEmpty())
		Expect(w.Body.String()).ToNot(ContainSubstring("private"))
		Expect(w.Body.String()).ToNot(ContainSubstring("10.0.0.7"))
	})

	It("keeps hiding opt-in while classified messages remain visible", func() {
		writer := NewErrorWriter(ErrorOptions{})
		visible := httptest.NewRecorder()
		writer.Write(context.Background(), visible, errors.New("connection refused"))
		Expect(decodeErrorResponse(visible).Message).To(Equal("connection refused"))

		hiddenWriter := NewErrorWriter(ErrorOptions{HideDetails: true})
		classified := httptest.NewRecorder()
		hiddenWriter.Write(context.Background(), classified,
			NewStatusError(http.StatusNotFound, "profile_not_found", "profile invoices not found"))
		Expect(decodeErrorResponse(classified).Message).To(Equal("profile invoices not found"))
	})

	It("bounds nested diagnostics without mutating their maps", func() {
		nestedBytes := []byte(strings.Repeat("x", 128))
		nested := map[string]any{"raw": nestedBytes, "message": strings.Repeat("y", 128)}
		contextValues := map[string]any{"metadata": nested}
		w := httptest.NewRecorder()

		NewErrorWriter(ErrorOptions{MaxDetailBytes: 32}).Write(context.Background(), w,
			diagnosticTestError{message: "failed", context: contextValues})

		response := decodeErrorResponse(w)
		metadata := response.Context["metadata"].(map[string]any)
		Expect(metadata["raw"]).To(HaveLen(32))
		Expect(metadata["message"]).To(HaveLen(32))
		Expect(nested["raw"]).To(Equal(nestedBytes))
		Expect(nested["message"]).To(Equal(strings.Repeat("y", 128)))
	})

	It("produces valid UTF-8 when truncating invalid diagnostic bytes", func() {
		value := string(append([]byte("caf\xe9-"), []byte(strings.Repeat("界", 32))...))
		w := httptest.NewRecorder()
		NewErrorWriter(ErrorOptions{MaxDetailBytes: 32}).Write(context.Background(), w,
			diagnosticTestError{message: value, context: map[string]any{"request_body": value}})

		response := decodeErrorResponse(w)
		Expect(utf8.ValidString(response.Message)).To(BeTrue())
		Expect(response.Message).To(HaveSuffix(truncationMarker))
		Expect(response.Details).To(HaveLen(1))
		Expect(utf8.ValidString(response.Details[0].Value)).To(BeTrue())
		Expect(response.Details[0].Value).To(HaveSuffix(truncationMarker))
		Expect(len(response.Details[0].Value)).To(BeNumerically("<=", 32))
	})

	It("bounds the full envelope without mutating caller-owned response data", func() {
		originalContext := map[string]any{"keep": strings.Repeat("context", 64)}
		response := ErrorResponse{
			Code: "internal_error", Message: strings.Repeat("message", 64),
			Trace: strings.Repeat("a", 32), Stacktrace: strings.Repeat("stack", 64),
			Context: originalContext,
			Details: []ErrorDetail{{Label: "body", Value: strings.Repeat("detail", 64), ContentType: "text/plain"}},
		}
		body := NewErrorWriter(ErrorOptions{MaxResponseBytes: 180}).marshal(response)

		Expect(json.Valid(body)).To(BeTrue())
		Expect(len(body)).To(BeNumerically("<=", 180))
		Expect(originalContext).To(HaveKey("keep"))
		Expect(response.Details).To(HaveLen(1))
	})

	It("never panics for nil errors or unmarshalable diagnostic values", func() {
		inputs := []error{
			nil,
			diagnosticTestError{message: "failed", context: map[string]any{"channel": make(chan int)}},
		}
		for _, input := range inputs {
			w := httptest.NewRecorder()
			Expect(func() {
				NewErrorWriter(ErrorOptions{MaxResponseBytes: 128}).Write(context.Background(), w, input)
			}).ToNot(Panic())
			Expect(w.Code).To(Equal(http.StatusInternalServerError))
			Expect(json.Valid(w.Body.Bytes())).To(BeTrue())
			Expect(decodeErrorResponse(w).Trace).To(MatchRegexp(`^[0-9a-f]{32}$`))
		}
	})

	It("preserves the legacy writers byte for byte", func() {
		classified := httptest.NewRecorder()
		NewStatusError(http.StatusConflict, "connection_required", "select a connection").Write(classified)
		Expect(classified.Body.String()).To(Equal("{\"code\":\"connection_required\",\"message\":\"select a connection\"}\n"))
		Expect(classified.Header().Get(ErrorTraceHeader)).To(BeEmpty())

		unclassified := httptest.NewRecorder()
		WriteError(unclassified, fmt.Errorf("dial tcp 10.0.0.7:5432: connection refused"))
		Expect(unclassified.Body.String()).To(Equal("{\"code\":\"internal_error\",\"message\":\"internal server error\"}\n"))
		Expect(unclassified.Header().Get(ErrorTraceHeader)).To(BeEmpty())
	})
})

func decodeErrorResponse(w *httptest.ResponseRecorder) ErrorResponse {
	var response ErrorResponse
	Expect(json.Unmarshal(w.Body.Bytes(), &response)).To(Succeed())
	return response
}

type diagnosticTestError struct {
	message string
	public  string
	hint    string
	stack   string
	trace   string
	context map[string]any
}

func (e diagnosticTestError) Error() string           { return e.message }
func (e diagnosticTestError) Context() map[string]any { return e.context }
func (e diagnosticTestError) Hint() string            { return e.hint }
func (e diagnosticTestError) Public() string          { return e.public }
func (e diagnosticTestError) Stacktrace() string      { return e.stack }
func (e diagnosticTestError) Trace() string           { return e.trace }
