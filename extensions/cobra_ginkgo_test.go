package extensions

import (
	"github.com/flanksource/clicky/rpc"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"
)

var _ = Describe("ServeCommandWithConfig", func() {
	It("registers serve directly on the root command", func() {
		root := &cobra.Command{Use: "example"}
		CobraExtensions(root).ServeCommandWithConfig(&rpc.OpenAPIConfig{Title: "Example API"})

		command, _, err := root.Find([]string{"serve"})
		Expect(err).NotTo(HaveOccurred())
		Expect(command.Name()).To(Equal("serve"))
		Expect(command.Parent()).To(BeIdenticalTo(root))
	})
})
