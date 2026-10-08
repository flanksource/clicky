package sse

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("hub control error responses", func() {
	DescribeTable("escapes reflected markup while preserving the JSON error",
		func(method string) {
			const markup = "<script>alert('example')</script>&"
			server := newEventsTestServer(nil)
			client := openEvents(server.URL, DefaultPrefix)
			var resp *http.Response
			var want string
			if method == http.MethodPost {
				resp = client.subscribe(markup, "/api/test/missing")
				want = fmt.Sprintf("sub id %q must match %s", markup, hubSubIDPattern)
			} else {
				resp = client.unsubscribe(url.PathEscape(markup))
				want = fmt.Sprintf("unknown sub %q on events connection %q", markup, client.conn)
			}
			Expect(resp.StatusCode).To(Equal(map[string]int{
				http.MethodPost: http.StatusBadRequest, http.MethodDelete: http.StatusNotFound,
			}[method]))
			Expect(resp.Header.Get("Content-Type")).To(Equal("application/json"))
			Expect(resp.Header.Get("X-Content-Type-Options")).To(Equal("nosniff"))
			body := readBody(resp)
			Expect(body).NotTo(ContainSubstring("<"))
			Expect(body).NotTo(ContainSubstring(">"))
			Expect(body).NotTo(ContainSubstring("&"))
			var decoded map[string]string
			Expect(json.NewDecoder(strings.NewReader(body)).Decode(&decoded)).To(Succeed())
			Expect(decoded).To(Equal(map[string]string{"error": want}))
		},
		Entry("POST subscription validation", http.MethodPost),
		Entry("DELETE unknown subscription", http.MethodDelete),
	)
})
