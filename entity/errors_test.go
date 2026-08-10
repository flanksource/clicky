package entity

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("StatusError", func() {
	It("renders only the two fields a client branches on", func() {
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

			var body map[string]string
			Expect(json.Unmarshal(w.Body.Bytes(), &body)).To(Succeed())
			Expect(body).To(Equal(map[string]string{"code": code, "message": message}))
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
		var body map[string]string
		Expect(json.Unmarshal(w.Body.Bytes(), &body)).To(Succeed())
		Expect(body["code"]).To(Equal("profile_not_found"))
	})

	It("gives an unclassified error the same shape at 500", func() {
		w := httptest.NewRecorder()
		WriteError(w, errors.New("connection refused"))

		Expect(w.Code).To(Equal(http.StatusInternalServerError))
		var body map[string]string
		Expect(json.Unmarshal(w.Body.Bytes(), &body)).To(Succeed())
		Expect(body).To(Equal(map[string]string{"code": "internal_error", "message": "connection refused"}))
	})
})
