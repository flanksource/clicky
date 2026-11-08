package flags

import (
	"encoding/json"
	"reflect"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"
)

var _ = Describe("structured action flags", func() {
	It("omits unset JSON defaults from help while retaining explicit defaults", func() {
		type target struct{ URI string }
		type options struct {
			Target   target            `flag:"target"`
			Assignee *target           `flag:"assignee"`
			Links    []target          `flag:"links"`
			Content  json.RawMessage   `flag:"content"`
			Default  map[string]string `flag:"default" default:"{\"env\":\"dev\"}"`
		}
		fields, err := ParseStructFields(reflect.TypeFor[options]())
		Expect(err).NotTo(HaveOccurred())
		command := &cobra.Command{}
		for _, field := range fields {
			BindFlag(command, field)
		}
		for _, name := range []string{"target", "assignee", "links", "content"} {
			Expect(command.Flags().Lookup(name).DefValue).To(BeEmpty(), name)
		}
		Expect(command.Flags().Lookup("default").DefValue).To(MatchJSON(`{"env":"dev"}`))
		Expect(command.Flags().FlagUsages()).NotTo(ContainSubstring("default null"))
	})

	It("renders invalid JSON values as diagnostics without panicking", func() {
		Expect((&jsonFlagValue{}).String()).To(BeEmpty())
		value := &jsonFlagValue{value: reflect.ValueOf(json.RawMessage(`invalid`))}
		Expect(value.String()).To(ContainSubstring("invalid JSON"))
	})

	It("preserves standard duration defaults and request values alongside int64 versions", func() {
		type options struct {
			Timeout time.Duration `flag:"timeout" default:"60s"`
		}
		fields, err := ParseStructFields(reflect.TypeFor[options]())
		Expect(err).NotTo(HaveOccurred())
		command := &cobra.Command{}
		value := BindFlag(command, fields[0])
		var cli options
		Expect(AssignFieldValue(reflect.ValueOf(&cli).Elem(), value, nil, false)).To(Succeed())
		Expect(cli.Timeout).To(Equal(time.Minute))
		var request options
		Expect(PopulateFromRequest(reflect.ValueOf(&request).Elem(), fields, map[string]string{"timeout": "90s"}, nil)).To(Succeed())
		Expect(request.Timeout).To(Equal(90 * time.Second))
	})
	type target struct {
		URI      string          `json:"uri"`
		Selector json.RawMessage `json:"selector"`
	}
	type options struct {
		Version  int64           `flag:"expectedVersion"`
		Target   target          `flag:"target"`
		Assignee *target         `flag:"assignee"`
		Links    []target        `flag:"links"`
		Content  json.RawMessage `flag:"content"`
	}
	It("preserves structured values and full-width versions through CLI and request binding", func() {
		inputs := map[string]string{"expectedVersion": "9223372036854775807", "target": `{"uri":"example://one","selector":{"line":3}}`, "assignee": "null", "links": `[{"uri":"example://evidence","selector":null}]`, "content": `{"capture":{"x":3}}`}
		fields, err := ParseStructFields(reflect.TypeFor[options]())
		Expect(err).NotTo(HaveOccurred())
		command := &cobra.Command{Use: "example"}
		bound := make([]*FlagValue, 0, len(fields))
		for _, field := range fields {
			bound = append(bound, BindFlag(command, field))
		}
		for key, value := range inputs {
			Expect(command.Flags().Set(key, value)).To(Succeed())
		}
		var cli options
		for _, value := range bound {
			Expect(AssignFieldValue(reflect.ValueOf(&cli).Elem(), value, nil, false)).To(Succeed())
		}
		var request options
		Expect(PopulateFromRequest(reflect.ValueOf(&request).Elem(), fields, inputs, nil)).To(Succeed())
		Expect(request).To(Equal(cli))
		Expect(request.Version).To(Equal(int64(9223372036854775807)))
		Expect(request.Target.URI).To(Equal("example://one"))
		Expect(request.Links).To(HaveLen(1))
		Expect(request.Assignee).To(BeNil())
		Expect(request.Content).To(MatchJSON(`{"capture":{"x":3}}`))
		Expect(command.Flags().Set("target", `{"unknown":"field"}`)).To(HaveOccurred())
		Expect(command.Flags().Set("target", `{} {}`)).To(HaveOccurred())
		Expect(PopulateFromRequest(reflect.ValueOf(&request).Elem(), fields, map[string]string{"expectedVersion": "9223372036854775808"}, nil)).To(HaveOccurred())
	})
})
