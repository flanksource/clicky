package rpc

import (
	"context"
	"net/http/httptest"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"
)

// A command naming its positional in Use ("pretty <file>") publishes that name
// as a required parameter; callers may still send the generic "args" array.
var _ = Describe("named positional parameters", func() {
	const dataFile = "orders.json"

	var (
		executor  *CommandExecutor
		operation *RPCOperation
	)

	BeforeEach(func() {
		cmd := &cobra.Command{
			Use:  "pretty [flags] <file> [file...]",
			Args: cobra.MinimumNArgs(1),
			RunE: func(*cobra.Command, []string) error { return nil },
		}
		cmd.Flags().String("schema", "", "schema file")
		var err error
		operation, err = NewConverter(nil).ConvertCommand(cmd)
		Expect(err).NotTo(HaveOccurred())
		executor = NewCommandExecutor(&RPCService{Operations: []RPCOperation{*operation}}, nil)
	})

	It("publishes the named positional as a required positional parameter", func() {
		Expect(operation.Parameters).To(ContainElement(RPCParameter{
			Name: "file", Type: "string", Description: "Positional argument from command", Required: true, In: "query", Positional: true,
		}))
	})

	DescribeTable("HTTP requests deliver the positional as args, never as a flag",
		func(target, body string) {
			request := httptest.NewRequest("POST", target, strings.NewReader(body))
			request.Header.Set("Content-Type", "application/json")

			extracted, err := executor.ExtractRequestFromHTTP(request, operation)

			Expect(err).NotTo(HaveOccurred())
			Expect(extracted.Args).To(Equal([]string{dataFile}))
			Expect(extracted.Flags).NotTo(HaveKey("file"))
			Expect(executor.ValidateParameters(extracted, operation)).To(Succeed())
		},
		Entry("generic args array in the body", "/api/v1/pretty", `{"args":["`+dataFile+`"],"schema":"s.yaml"}`),
		Entry("named positional in the body", "/api/v1/pretty", `{"file":"`+dataFile+`","schema":"s.yaml"}`),
		Entry("named positional in the query", "/api/v1/pretty?file="+dataFile, `{"schema":"s.yaml"}`),
		Entry("generic args in the query", "/api/v1/pretty?args="+dataFile, `{"schema":"s.yaml"}`),
	)

	It("still reports a missing positional", func() {
		request := httptest.NewRequest("POST", "/api/v1/pretty", strings.NewReader(`{"schema":"s.yaml"}`))
		request.Header.Set("Content-Type", "application/json")

		_, err := executor.ExtractRequestFromHTTP(request, operation)

		Expect(err).To(MatchError("required parameter file is missing"))
	})

	It("operation values deliver the named positional as args", func() {
		request, err := executionRequestFromValues(context.Background(), operation, map[string]any{"file": dataFile, "schema": "s.yaml"})

		Expect(err).NotTo(HaveOccurred())
		Expect(request.Args).To(Equal([]string{dataFile}))
		Expect(request.Flags).To(Equal(map[string]string{"schema": "s.yaml"}))
	})
})
