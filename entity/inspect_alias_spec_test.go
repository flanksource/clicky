package entity

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"
)

var _ = Describe("explicit inspection actions", func() {
	It("resolves inspect to its action rather than the get alias", func() {
		root := &cobra.Command{Use: "test"}
		generateEntityCLI(root, EntityInfo{Name: "connection", Operations: []EntityOperation{{Verb: "get", Method: "GET"}}, Actions: []ActionInfo{{Name: "inspect", Method: "POST"}}})
		command, _, err := root.Find([]string{"connection", "inspect"})
		Expect(err).NotTo(HaveOccurred())
		Expect(command.Name()).To(Equal("inspect"))
	})
})
