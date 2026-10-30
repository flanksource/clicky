package rpc

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"

	"github.com/flanksource/clicky/entity"
	"github.com/flanksource/clicky/flags"
)

// A query key repeated on a GET must reach the operation the way a repeated
// flag reaches it on the command line: ?kind=a&kind=b is --kind a --kind b.
var _ = Describe("repeated query parameters", func() {
	var (
		executor  *CommandExecutor
		operation *RPCOperation
		kinds     []string
	)

	BeforeEach(func() {
		kinds = nil
		cmd := &cobra.Command{
			Use:  "targets",
			RunE: func(*cobra.Command, []string) error { return nil },
		}
		cmd.Flags().StringSliceVar(&kinds, "kind", nil, "kinds to include")
		cmd.Flags().String("name", "", "target name")
		var err error
		operation, err = NewConverter(nil).ConvertCommand(cmd)
		Expect(err).NotTo(HaveOccurred())
		executor = NewCommandExecutor(&RPCService{Operations: []RPCOperation{*operation}},
			&ExecutorConfig{Enabled: true, SkipPreRun: true, PathPrefix: "/api/v1"})
	})

	DescribeTable("a slice flag receives every value, as the CLI would",
		func(target string, want []string) {
			request, err := executor.ExtractRequestFromHTTP(httptest.NewRequest(http.MethodGet, target, nil), operation)
			Expect(err).NotTo(HaveOccurred())

			_, _, err = executor.ExecuteCommand(operation, request)

			Expect(err).NotTo(HaveOccurred())
			Expect(kinds).To(Equal(want))
		},
		Entry("repeated key", "/api/v1/targets?kind=table&kind=!column", []string{"table", "!column"}),
		Entry("comma-separated single value", "/api/v1/targets?kind=table,!column", []string{"table", "!column"}),
		Entry("repeated key mixing comma values", "/api/v1/targets?kind=table,view&kind=!column", []string{"table", "view", "!column"}),
	)

	It("binds every repeated value onto a MultiFilter field", func() {
		type targetFlags struct {
			Kind entity.MultiFilter `flag:"kind"`
		}
		request, err := executor.ExtractRequestFromHTTP(httptest.NewRequest(http.MethodGet, "/api/v1/targets?kind=table&kind=!column", nil), operation)
		Expect(err).NotTo(HaveOccurred())
		fields, err := flags.ParseStructFields(reflect.TypeOf(targetFlags{}))
		Expect(err).NotTo(HaveOccurred())

		var decoded targetFlags
		Expect(flags.PopulateFromRequest(reflect.ValueOf(&decoded).Elem(), fields, request.Flags, nil)).To(Succeed())

		Expect(decoded.Kind).To(Equal(entity.MultiFilter{"table", "!column"}))
	})

	It("refuses a scalar flag given more than once with a 400 naming it", func() {
		_, err := executor.ExtractRequestFromHTTP(httptest.NewRequest(http.MethodGet, "/api/v1/targets?name=a&name=b", nil), operation)

		Expect(err).To(MatchError(ContainSubstring("name")))
		var status *entity.StatusError
		Expect(errors.As(err, &status)).To(BeTrue())
		Expect(status.StatusCode()).To(Equal(http.StatusBadRequest))
	})

	It("passes every repeated positional value as args", func() {
		request, err := executor.ExtractRequestFromHTTP(httptest.NewRequest(http.MethodGet, "/api/v1/targets?args=a&args=b,c", nil), operation)

		Expect(err).NotTo(HaveOccurred())
		Expect(request.Args).To(Equal([]string{"a", "b", "c"}))
	})
})
