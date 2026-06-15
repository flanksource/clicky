//go:build unix

package exec

import (
	"net"
	"os/exec"
	"strconv"
	"syscall"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("parseListenPorts", func() {
	It("extracts ports from IPv4, IPv6 and wildcard addresses", func() {
		out := "p123\nf3\nn*:3000\nf5\nn127.0.0.1:8080\nf7\nn[::1]:5000\n"
		Expect(parseListenPorts(out)).To(Equal([]int{3000, 5000, 8080}))
	})

	It("deduplicates the same port reported on multiple fds", func() {
		out := "p123\nf8\nn*:49152\nf9\nn*:49152\n"
		Expect(parseListenPorts(out)).To(Equal([]int{49152}))
	})

	It("returns every distinct listening port of one process in ascending order", func() {
		out := "p123\nf3\nn*:8080\nf4\nn127.0.0.1:3000\nf5\nn[::1]:9090\n"
		Expect(parseListenPorts(out)).To(Equal([]int{3000, 8080, 9090}))
	})

	It("ignores established connections and malformed addresses", func() {
		out := "n127.0.0.1:8080->127.0.0.1:55000\nn*:notaport\nnno-colon-here\nn*:\n"
		Expect(parseListenPorts(out)).To(BeNil())
	})

	It("returns nil for empty output", func() {
		Expect(parseListenPorts("")).To(BeNil())
	})
})

var _ = Describe("ListeningPorts", func() {
	It("returns nil for a non-positive pgid", func() {
		got, err := ListeningPorts(0)
		Expect(err).NotTo(HaveOccurred())
		Expect(got).To(BeNil())
	})

	It("detects multiple ports opened in the current process group", func() {
		if _, err := exec.LookPath("lsof"); err != nil {
			Skip("lsof not installed")
		}
		ln1, err := net.Listen("tcp", "127.0.0.1:0")
		Expect(err).NotTo(HaveOccurred())
		defer ln1.Close()
		ln2, err := net.Listen("tcp", "127.0.0.1:0")
		Expect(err).NotTo(HaveOccurred())
		defer ln2.Close()
		p1 := ln1.Addr().(*net.TCPAddr).Port
		p2 := ln2.Addr().(*net.TCPAddr).Port

		pgid, err := syscall.Getpgid(syscall.Getpid())
		Expect(err).NotTo(HaveOccurred())

		Eventually(func() []int {
			ports, perr := ListeningPorts(pgid)
			Expect(perr).NotTo(HaveOccurred())
			return ports
		}, "3s", "100ms").Should(And(ContainElement(p1), ContainElement(p2)),
			"expected to detect both bound ports "+strconv.Itoa(p1)+" and "+strconv.Itoa(p2))
	})
})
