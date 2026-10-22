package rpc

import (
	"net/http"
	"net/http/httptest"
	"sync"

	rpchttp "github.com/flanksource/clicky/rpc/http"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Swagger no-color responses", func() {
	It("renders concurrent JSON and terminal formats without ANSI and measures setup", func() {
		var wg sync.WaitGroup
		for _, format := range []string{"json", "pretty", "tree", "markdown"} {
			format := format
			wg.Go(func() {
				defer GinkgoRecover()
				ctx, timings := rpchttp.WithTimings(GinkgoT().Context())
				r := httptest.NewRequest(http.MethodGet, "/example", nil).WithContext(ctx)
				w := httptest.NewRecorder()
				(&SwaggerServer{}).writeFormattedResponse(w, r, []struct{ Name string }{{"example"}}, formatOptions{Format: format}, http.StatusOK)
				Expect(w.Code).To(Equal(http.StatusOK))
				Expect(w.Body.String()).NotTo(ContainSubstring("\x1b["))
				_, measured := timings.Duration("format_setup")
				Expect(measured).To(BeTrue())
			})
		}
		wg.Wait()
	})
})
