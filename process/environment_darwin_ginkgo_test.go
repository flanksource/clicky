//go:build darwin

package process

import (
	"bytes"
	"encoding/binary"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Darwin process environments", func() {
	It("decodes the NUL-separated argv and environment payload", func() {
		var raw bytes.Buffer
		Expect(binary.Write(&raw, binary.NativeEndian, int32(2))).To(Succeed())
		raw.WriteString("/usr/bin/tool\x00\x00tool\x00--flag\x00A=1\x00B=left=right\x00")

		environment, err := parseDarwinEnvironment(raw.Bytes())
		Expect(err).NotTo(HaveOccurred())
		Expect(environment).To(Equal([]string{"A=1", "B=left=right"}))
	})

	It("ends the environment at its terminator, leaving out the apple strings after it, and takes an empty argument as one", func() {
		var raw bytes.Buffer
		Expect(binary.Write(&raw, binary.NativeEndian, int32(3))).To(Succeed())
		raw.WriteString("/usr/bin/tool\x00\x00\x00\x00tool\x00\x00--flag\x00ACME_REGION=eu\x00\x00executable_file=0x1a01\x00ptr_munge=\x00main_stack=\x00\x00\x00")

		environment, err := parseDarwinEnvironment(raw.Bytes())
		Expect(err).NotTo(HaveOccurred())
		Expect(environment).To(Equal([]string{"ACME_REGION=eu"}))
	})

	It("reads no environment for a process that has none", func() {
		var raw bytes.Buffer
		Expect(binary.Write(&raw, binary.NativeEndian, int32(1))).To(Succeed())
		raw.WriteString("/usr/bin/tool\x00\x00tool\x00\x00executable_file=0x1a01\x00\x00")

		environment, err := parseDarwinEnvironment(raw.Bytes())
		Expect(err).NotTo(HaveOccurred())
		Expect(environment).To(BeEmpty())
	})
})
