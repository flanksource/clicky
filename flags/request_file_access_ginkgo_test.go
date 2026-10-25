package flags

import (
	"context"
	"io"
	"net/http"
	"reflect"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type recordedFetch struct {
	url       string
	defensive bool
}

type recordingFetcher struct {
	calls []recordedFetch
	body  string
}

func (f *recordingFetcher) Fetch(_ context.Context, url string, defensive bool) (*http.Response, error) {
	f.calls = append(f.calls, recordedFetch{url: url, defensive: defensive})
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(f.body))}, nil
}

var _ = Describe("URL file references", func() {
	const documentURL = "http://example.com/document"

	parseFields := func() []FieldInfo {
		fields, err := ParseStructFields(reflect.TypeOf(rpcFileOpts{}))
		Expect(err).ToNot(HaveOccurred())
		return fields
	}

	AfterEach(func() {
		SetURLFetcher(nil)
	})

	Context("when no URL fetcher is configured", func() {
		It("OpenFileReference returns ErrNoURLFetcher", func() {
			_, err := OpenFileReference(context.Background(), "@"+documentURL, FileReferenceOptions{})
			Expect(err).To(MatchError(ErrNoURLFetcher))
		})

		It("request decoding returns ErrNoURLFetcher", func() {
			var opts rpcFileOpts
			err := PopulateFromRequest(reflect.ValueOf(&opts).Elem(), parseFields(), map[string]string{"name": "@" + documentURL}, nil)
			Expect(err).To(MatchError(ErrNoURLFetcher))
		})
	})

	It("passes remote callers' URLs to the fetcher as defensive", func() {
		fetcher := &recordingFetcher{body: "remote document"}
		SetURLFetcher(fetcher)
		var opts rpcFileOpts

		err := PopulateFromRequest(reflect.ValueOf(&opts).Elem(), parseFields(), map[string]string{"name": "@" + documentURL}, nil)

		Expect(err).NotTo(HaveOccurred())
		Expect(opts.Name).To(Equal("remote document"))
		Expect(fetcher.calls).To(Equal([]recordedFetch{{url: documentURL, defensive: true}}))
	})

	It("passes local file references to the fetcher as non-defensive", func() {
		fetcher := &recordingFetcher{body: "local document"}
		SetURLFetcher(fetcher)

		opened, err := OpenFileReference(context.Background(), "@"+documentURL+".csv", FileReferenceOptions{})

		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(opened.Body.Close)
		Expect(opened.Name).To(Equal("document.csv"))
		Expect(fetcher.calls).To(Equal([]recordedFetch{{url: documentURL + ".csv", defensive: false}}))
	})
})
