//go:build !windows

package process

import (
	"context"
	"net"
	"os"
	"os/exec"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const lsofFixture = `p4242
fcwd
tDIR
n/work/repo
f1
tREG
n/work/repo/out.log
f5
tIPv4
PTCP
n127.0.0.1:8080
TST=LISTEN
TQR=0
f6
tIPv6
PTCP
n[::1]:5432->[::1]:62805
TST=ESTABLISHED
f7
tIPv4
PUDP
n*:5353
f8
tunix
n->0x7c8f344aac6ca236
f9
tPIPE
n->0x1234
`

var _ = Describe("process descriptors", func() {
	It("splits lsof field output into files and sockets with their endpoints", func() {
		descriptors := parseLsof([]byte(lsofFixture))

		Expect(descriptors).To(Equal([]Descriptor{
			{FD: "cwd", Type: "DIR", Name: "/work/repo"},
			{FD: "1", Type: "REG", Name: "/work/repo/out.log"},
			{FD: "5", Type: "IPv4", Name: "127.0.0.1:8080", Protocol: "TCP", State: "LISTEN", LocalAddr: "127.0.0.1:8080", Port: 8080},
			{FD: "6", Type: "IPv6", Name: "[::1]:5432->[::1]:62805", Protocol: "TCP", State: "ESTABLISHED", LocalAddr: "[::1]:5432", RemoteAddr: "[::1]:62805", Port: 5432},
			{FD: "7", Type: "IPv4", Name: "*:5353", Protocol: "UDP", LocalAddr: "*:5353", Port: 5353},
			{FD: "8", Type: "unix", Name: "->0x7c8f344aac6ca236"},
			{FD: "9", Type: "PIPE", Name: "->0x1234"},
		}))
		Expect(descriptors).To(HaveEach(Satisfy(func(d Descriptor) bool {
			return d.IsSocket() == (d.Type == "IPv4" || d.Type == "IPv6" || d.Type == "unix")
		})))
		var listening []string
		for _, d := range descriptors {
			if d.IsListening() {
				listening = append(listening, d.LocalAddr)
			}
		}
		Expect(listening).To(Equal([]string{"127.0.0.1:8080", "*:5353"}))
	})

	It("lists a live process's open files and listening ports", func() {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		Expect(err).NotTo(HaveOccurred())
		defer listener.Close()
		path := filepath.Join(GinkgoT().TempDir(), "held-open.txt")
		file, err := os.Create(path)
		Expect(err).NotTo(HaveOccurred())
		defer file.Close()
		resolved, err := filepath.EvalSymlinks(path)
		Expect(err).NotTo(HaveOccurred())

		descriptors, err := Descriptors(context.Background(), os.Getpid())
		Expect(err).NotTo(HaveOccurred())
		Expect(descriptors).To(ContainElement(HaveField("Name", Or(Equal(path), Equal(resolved)))))
		Expect(descriptors).To(ContainElement(And(
			HaveField("Port", listener.Addr().(*net.TCPAddr).Port),
			HaveField("State", "LISTEN"),
		)))
	})

	It("fails for a pid that is not running", func() {
		cmd := exec.Command("true")
		Expect(cmd.Run()).To(Succeed())
		_, err := Descriptors(context.Background(), cmd.Process.Pid)
		Expect(err).To(MatchError(ContainSubstring("no open files")))
	})

	It("reads a live process's whole environment", func() {
		cmd := exec.Command("sleep", "30")
		cmd.Env = []string{"ACME_PROBE=left=right", "PATH=" + os.Getenv("PATH")}
		Expect(cmd.Start()).To(Succeed())
		defer cmd.Process.Kill() //nolint:errcheck

		Eventually(func() (map[string]string, error) {
			return Environment(context.Background(), cmd.Process.Pid)
		}).Should(HaveKeyWithValue("ACME_PROBE", "left=right"))
	})
})
