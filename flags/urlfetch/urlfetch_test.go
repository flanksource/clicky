package urlfetch_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync/atomic"
	"testing"

	"github.com/flanksource/clicky/flags"
	"github.com/flanksource/clicky/flags/urlfetch"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type rpcFileOpts struct {
	Name string `flag:"name" clicky:"cli-file-read rpc-file-read"`
}

var _ = BeforeEach(func() {
	urlfetch.Use()
	DeferCleanup(func() { flags.SetURLFetcher(nil) })
})

var _ = Describe("OpenFileReference over HTTP", func() {
	It("reads a URL without losing its extension", func() {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte("Name\nAlice\n"))
		}))
		DeferCleanup(server.Close)

		opened, err := flags.OpenFileReference(context.Background(), "@"+server.URL+"/data.csv?version=1", flags.FileReferenceOptions{})
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(opened.Body.Close)
		body, err := io.ReadAll(opened.Body)
		Expect(err).NotTo(HaveOccurred())
		Expect(opened.Name).To(Equal("data.csv"))
		Expect(string(body)).To(Equal("Name\nAlice\n"))
	})

	It("rejects loopback URLs for remote callers without exposing the query", func() {
		var requests atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			requests.Add(1)
		}))
		DeferCleanup(server.Close)

		_, err := flags.OpenFileReference(context.Background(), "@"+server.URL+"/data.csv?token=private-value", flags.FileReferenceOptions{Remote: true})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).NotTo(ContainSubstring("private-value"))
		Expect(requests.Load()).To(BeZero())
	})
})

var _ = Describe("RPC file-read decoding", func() {
	parseFields := func() []flags.FieldInfo {
		fields, err := flags.ParseStructFields(reflect.TypeOf(rpcFileOpts{}))
		Expect(err).ToNot(HaveOccurred())
		return fields
	}

	It("uses the Commons defensive client for request-supplied URLs", func() {
		var requests atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			requests.Add(1)
		}))
		DeferCleanup(server.Close)
		var opts rpcFileOpts

		err := flags.PopulateFromRequest(
			reflect.ValueOf(&opts).Elem(),
			parseFields(),
			map[string]string{"name": "@" + server.URL},
			nil,
		)

		Expect(err).To(MatchError(ContainSubstring("localhost address")))
		Expect(requests.Load()).To(BeZero())
	})

	It("propagates the request context into URL expansion", func() {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		var opts rpcFileOpts

		err := flags.PopulateFromRequest(
			reflect.ValueOf(&opts).Elem(),
			parseFields(),
			map[string]string{"name": "@http://example.com/document"},
			nil,
			flags.WithRequestContext(ctx),
		)

		Expect(errors.Is(err, context.Canceled)).To(BeTrue())
	})
})

func TestURLFetch(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "URL Fetch Suite")
}
