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
})
