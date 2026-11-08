package prompt

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestPromptIdentity(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Prompt identity")
}

var _ = Describe("HTTP answer identity", func() {
	DescribeTable("attributes answers only through the configured identity callback", func(identify func(*http.Request) string, expected string) {
		manager := NewManager(nil)
		manager.SetIdentity(identify)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		answered := make(chan Answer, 1)
		go func() {
			answer, _ := manager.Ask(ctx, Prompt{ID: "identity", Schema: TextSchema("Name", false)})
			answered <- answer
		}()
		Eventually(func() bool { _, ok := manager.Pending("identity"); return ok }).Should(BeTrue())
		request := httptest.NewRequest(http.MethodPost, "/prompts/identity/answer", strings.NewReader(`{"values":{"value":"Alex"}}`))
		request.SetPathValue("id", "identity")
		request.Header.Set("X-Prompt-User", "forged-principal")
		response := httptest.NewRecorder()
		manager.ResolveHandler().ServeHTTP(response, request)
		Expect(response.Code).To(Equal(http.StatusOK), response.Body.String())
		Eventually(answered).Should(Receive(HaveField("By", expected)))
		snapshot, ok := manager.Snapshot("identity")
		Expect(ok).To(BeTrue())
		Expect(snapshot.ResolvedBy).To(Equal(expected))
	}, Entry("untrusted client header", nil, ""),
		Entry("authenticated callback", func(*http.Request) string { return "authenticated-principal" }, "authenticated-principal"))
})
