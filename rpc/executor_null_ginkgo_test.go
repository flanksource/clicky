package rpc

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"

	"github.com/flanksource/clicky/flags"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"
)

var _ = Describe("nullable JSON request flags", func() {
	type actor struct {
		ID string `json:"id"`
	}
	type options struct {
		Assignee *actor `flag:"assignee"`
	}

	DescribeTable("preserves assignment values through HTTP extraction and both flag execution paths",
		func(body string, want *actor, wantPresent bool) {
			fields, err := flags.ParseStructFields(reflect.TypeFor[options]())
			Expect(err).NotTo(HaveOccurred())
			command := &cobra.Command{Use: "assign", RunE: func(*cobra.Command, []string) error { return nil }}
			bound := flags.BindFlag(command, fields[0])
			operation, err := NewConverter(nil).ConvertCommand(command)
			Expect(err).NotTo(HaveOccurred())
			executor := NewCommandExecutor(&RPCService{Operations: []RPCOperation{*operation}}, &ExecutorConfig{Enabled: true})
			httpRequest := httptest.NewRequest(http.MethodPost, operation.Path, strings.NewReader(body))
			httpRequest.Header.Set("Content-Type", "application/json")

			request, err := executor.ExtractRequestFromHTTP(httpRequest, operation)
			Expect(err).NotTo(HaveOccurred())
			_, present := request.Flags["assignee"]
			Expect(present).To(Equal(wantPresent))

			var decoded options
			Expect(flags.PopulateFromRequest(reflect.ValueOf(&decoded).Elem(), fields, request.Flags, nil)).To(Succeed())
			Expect(decoded).To(Equal(options{Assignee: want}))

			_, response, err := executor.ExecuteCommand(operation, request)
			Expect(err).NotTo(HaveOccurred())
			Expect(response.Success).To(BeTrue())
			var cli options
			Expect(flags.AssignFieldValue(reflect.ValueOf(&cli).Elem(), bound, nil, false)).To(Succeed())
			Expect(cli).To(Equal(decoded))
		},
		Entry("explicit null clears the assignee", `{"assignee":null}`, nil, true),
		Entry("nested null clears the assignee", `{"flags":{"assignee":null}}`, nil, true),
		Entry("an actor is assigned", `{"assignee":{"id":"reviewer"}}`, &actor{ID: "reviewer"}, true),
		Entry("an omitted assignee stays absent", `{}`, nil, false),
	)
})
