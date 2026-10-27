package rpc

import (
	"context"
	"net/http/httptest"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"

	"github.com/flanksource/clicky"
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

type optionalIDSyncFlags struct {
	IDs []string `flag:"id" help:"Only these resources"`
}

func (optionalIDSyncFlags) ClickyActionFlags() {}

// An optional-id entity action (`prompt run [id]`) is a flat collection route
// that takes its id as ?id=, keeps it out of the body schema, and yields the
// name to an `id` flag the action declares itself (recon's `resource sync`).
var _ = Describe("optional-id action parameters", Ordered, func() {
	const promptID = "p-review"

	var (
		executor   *CommandExecutor
		run, sync  *RPCOperation
		received   string
		syncedIDs  string
		convertCmd = func(root *cobra.Command, path ...string) *RPCOperation {
			cmd, _, err := root.Find(path)
			Expect(err).NotTo(HaveOccurred())
			op, err := NewConverter(nil).ConvertCommand(cmd)
			Expect(err).NotTo(HaveOccurred())
			return op
		}
	)

	BeforeAll(func() {
		clicky.NewEntity[formPrompt, struct{}, any]("optional-id-prompt").
			List(func(struct{}) ([]formPrompt, error) { return nil, nil }).
			WithAction(clicky.ActionWithContext("run", func(_ context.Context, id string, _ map[string]string) (any, error) {
				received = id
				return id, nil
			}).WithOptionalID()).
			WithAction(clicky.ActionWithFlagsAndContext("sync", optionalIDSyncFlags{}, func(_ context.Context, _ string, flags map[string]string) (any, error) {
				syncedIDs = flags["id"]
				return nil, nil
			}).WithOptionalID()).
			Register()
		root := &cobra.Command{Use: "app"}
		clicky.GenerateCLI(root)
		run = convertCmd(root, "optional-id-prompt", "run")
		sync = convertCmd(root, "optional-id-prompt", "sync")
		executor = NewCommandExecutor(&RPCService{Operations: []RPCOperation{*run, *sync}}, &ExecutorConfig{Enabled: true})
	})

	BeforeEach(func() { received, syncedIDs = "unset", "unset" })

	execute := func(op *RPCOperation, target, body string) error {
		request := httptest.NewRequest("POST", target, strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		extracted, err := executor.ExtractRequestFromHTTP(request, op)
		if err != nil {
			return err
		}
		_, _, err = executor.ExecuteCommand(op, extracted)
		return err
	}

	It("publishes the id as an optional, query-only positional", func() {
		Expect(run.Parameters).To(ContainElement(RPCParameter{
			Name: "id", Type: "string", Description: "Positional argument from command", In: "query", Positional: true,
		}))
		Expect(run.Schema.Properties).NotTo(HaveKey("id"))
	})

	DescribeTable("delivers the id to the action",
		func(target, body, id string) {
			Expect(execute(run, target, body)).To(Succeed())
			Expect(received).To(Equal(id))
		},
		Entry("id in the query", "/api/v1/optional-id-prompt/run?id="+promptID, `{}`, promptID),
		Entry("id in the body", "/api/v1/optional-id-prompt/run", `{"id":"`+promptID+`"}`, promptID),
		Entry("no id", "/api/v1/optional-id-prompt/run", `{}`, ""),
	)

	It("rejects a body carrying the positional under two keys", func() {
		Expect(execute(run, "/api/v1/optional-id-prompt/run", `{"id":"a","args":["b"]}`)).
			To(MatchError(ContainSubstring("sets positional arguments twice")))
	})

	It("delivers schedule values' id to the action", func() {
		request, err := executionRequestFromValues(context.Background(), run, map[string]any{"id": promptID})
		Expect(err).NotTo(HaveOccurred())
		_, _, err = executor.ExecuteCommand(run, request)
		Expect(err).NotTo(HaveOccurred())
		Expect(received).To(Equal(promptID))
	})

	It("leaves an action's own id flag in charge of the name", func() {
		var ids []string
		for _, param := range sync.Parameters {
			if param.Name == "id" {
				ids = append(ids, param.Name)
				Expect(param.Positional).To(BeFalse())
			}
		}
		Expect(ids).To(HaveLen(1), "one id parameter: the flag")

		Expect(execute(sync, "/api/v1/optional-id-prompt/sync", `{"id":["r1"]}`)).To(Succeed())
		Expect(syncedIDs).To(Equal("r1"))
	})
})
