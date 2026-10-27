package rpc

import (
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/flanksource/clicky/formatters"
)

var _ = Describe("custom formatter content type", func() {
	const (
		format      = "binary-report"
		contentType = "application/pdf"
	)
	// A PDF carries arbitrary bytes: an ANSI-looking escape run and invalid
	// UTF-8 must both survive the NoColor strip the response path applies.
	binary := "%PDF-1.7\n\x1b[31m\xff\xfe\x00stream\x1b[0m\n%%EOF"

	BeforeEach(func() {
		formatters.RegisterFormatter(format, func(any, formatters.FormatOptions) (string, error) {
			return binary, nil
		}, formatters.WithContentType(contentType))
		DeferCleanup(formatters.UnregisterFormatter, format)
	})

	It("serves a registered format with its declared content type", func() {
		Expect(formatToContentType(format)).To(Equal(contentType))
	})

	It("writes a binary body byte for byte", func() {
		w := httptest.NewRecorder()
		(&SwaggerServer{}).writeFormattedResponse(w, httptest.NewRequest("GET", "/x?format="+format, nil),
			map[string]string{"ok": "true"}, formatOptions{Format: format}, http.StatusOK)

		Expect(w.Code).To(Equal(http.StatusOK))
		Expect(w.Header().Get("Content-Type")).To(Equal(contentType))
		Expect(w.Body.String()).To(Equal(binary))
	})

	It("keeps JSON for a custom format registered without a content type", func() {
		formatters.RegisterFormatter("plain-custom", func(any, formatters.FormatOptions) (string, error) { return "", nil })
		DeferCleanup(formatters.UnregisterFormatter, "plain-custom")
		Expect(formatToContentType("plain-custom")).To(Equal("application/json"))
	})
})
