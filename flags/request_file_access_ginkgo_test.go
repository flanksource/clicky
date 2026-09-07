package flags

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync/atomic"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("RPC file-read decoding", func() {
	parseFields := func() []FieldInfo {
		fields, err := ParseStructFields(reflect.TypeOf(rpcFileOpts{}))
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

		err := PopulateFromRequest(
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

		err := PopulateFromRequest(
			reflect.ValueOf(&opts).Elem(),
			parseFields(),
			map[string]string{"name": "@http://example.com/document"},
			nil,
			WithRequestContext(ctx),
		)

		Expect(errors.Is(err, context.Canceled)).To(BeTrue())
	})
})
