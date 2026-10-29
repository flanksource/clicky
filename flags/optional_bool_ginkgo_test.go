package flags

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"
	"reflect"
)

var _ = Describe("optional boolean flags", func() {
	type options struct {
		Value *bool `flag:"value"`
	}
	It("distinguishes omitted, false and true across CLI and request inputs", func() {
		fields, err := ParseStructFields(reflect.TypeOf(options{}))
		Expect(err).NotTo(HaveOccurred())
		var invalid options
		Expect(PopulateFromRequest(reflect.ValueOf(&invalid).Elem(), fields, map[string]string{"value": "invalid"}, nil)).To(HaveOccurred())
		for _, input := range []map[string]string{nil, {"value": "false"}, {"value": "true"}} {
			var o options
			Expect(PopulateFromRequest(reflect.ValueOf(&o).Elem(), fields, input, nil)).To(Succeed())
			value, present := input["value"]
			Expect(o.Value != nil).To(Equal(present))
			if present {
				Expect(*o.Value).To(Equal(value == "true"))
			}
		}
		for _, args := range [][]string{nil, {"--value=false"}, {"--value"}} {
			cmd := &cobra.Command{}
			flag := BindFlag(cmd, fields[0])
			Expect(cmd.ParseFlags(args)).To(Succeed())
			var o options
			Expect(AssignFieldValue(reflect.ValueOf(&o).Elem(), flag, nil, false)).To(Succeed())
			Expect(o.Value != nil).To(Equal(len(args) > 0))
			if o.Value != nil {
				Expect(*o.Value).To(Equal(args[0] == "--value"))
			}
		}
	})
})
