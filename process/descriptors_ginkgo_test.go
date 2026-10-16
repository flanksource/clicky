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
	"github.com/onsi/gomega/types"
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
p4343
fcwd
tDIR
n/
f3
tIPv4
PUDP
n10.0.0.2:5000->10.0.0.9:53
`

var _ = Describe("process descriptors", func() {
	It("splits lsof field output by process into files and sockets with their endpoints", func() {
		byPID := parseLsof([]byte(lsofFixture))

		Expect(byPID).To(Equal(map[int][]Descriptor{
			4242: {
				{FD: "cwd", Type: "DIR", Name: "/work/repo"},
				{FD: "1", Type: "REG", Name: "/work/repo/out.log"},
				{FD: "5", Type: "IPv4", Name: "127.0.0.1:8080", Protocol: "TCP", State: "LISTEN", LocalAddr: "127.0.0.1:8080", Port: 8080},
				{FD: "6", Type: "IPv6", Name: "[::1]:5432->[::1]:62805", Protocol: "TCP", State: "ESTABLISHED", LocalAddr: "[::1]:5432", RemoteAddr: "[::1]:62805", Port: 5432},
				{FD: "7", Type: "IPv4", Name: "*:5353", Protocol: "UDP", LocalAddr: "*:5353", Port: 5353},
				{FD: "8", Type: "unix", Name: "->0x7c8f344aac6ca236"},
				{FD: "9", Type: "PIPE", Name: "->0x1234"},
			},
			4343: {
				{FD: "cwd", Type: "DIR", Name: "/"},
				{FD: "3", Type: "IPv4", Name: "10.0.0.2:5000->10.0.0.9:53", Protocol: "UDP", LocalAddr: "10.0.0.2:5000", RemoteAddr: "10.0.0.9:53", Port: 5000},
			},
		}))
		descriptors := append(byPID[4242], byPID[4343]...)
		Expect(descriptors).To(HaveEach(Satisfy(func(d Descriptor) bool {
			return d.IsSocket() == (d.Type == "IPv4" || d.Type == "IPv6" || d.Type == "unix")
		})))
		var listening, connected []string
		for _, d := range descriptors {
			if d.IsListening() {
				listening = append(listening, d.LocalAddr)
			}
			if d.IsConnected() {
				connected = append(connected, d.RemoteAddr)
			}
		}
		Expect(listening).To(Equal([]string{"127.0.0.1:8080", "*:5353"}))
		Expect(connected).To(Equal([]string{"[::1]:62805", "10.0.0.9:53"}), "a unix socket's peer is no connection")
	})

	It("lists the open files of many processes with one lsof, leaving out a pid that is not running", func() {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		Expect(err).NotTo(HaveOccurred())
		defer listener.Close()
		child := exec.Command("sleep", "30")
		Expect(child.Start()).To(Succeed())
		defer child.Process.Kill() //nolint:errcheck
		exited := exec.Command("true")
		Expect(exited.Run()).To(Succeed())

		byPID, err := Descriptors(context.Background(), os.Getpid(), child.Process.Pid, exited.Process.Pid)
		Expect(err).NotTo(HaveOccurred())
		Expect(byPID).To(HaveLen(2))
		Expect(byPID[os.Getpid()]).To(ContainElement(HaveField("Port", listener.Addr().(*net.TCPAddr).Port)))
		Expect(byPID[child.Process.Pid]).To(ContainElement(HaveField("FD", "cwd")))
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

		byPID, err := Descriptors(context.Background(), os.Getpid())
		Expect(err).NotTo(HaveOccurred())
		descriptors := byPID[os.Getpid()]
		Expect(descriptors).To(ContainElement(HaveField("Name", Or(Equal(path), Equal(resolved)))))
		Expect(descriptors).To(ContainElement(And(
			HaveField("Port", listener.Addr().(*net.TCPAddr).Port),
			HaveField("State", "LISTEN"),
		)))
	})

	DescribeTable("reads a live process's environment, limited by options",
		func(options EnvironmentOptions, want types.GomegaMatcher) {
			cmd := exec.Command("sleep", "30")
			cmd.Env = []string{"ACME_PROBE=left=right", "ACME_OTHER=1", "PATH=" + os.Getenv("PATH")}
			Expect(cmd.Start()).To(Succeed())
			defer cmd.Process.Kill() //nolint:errcheck

			Eventually(func() (map[string]string, error) {
				return Environment(context.Background(), cmd.Process.Pid, options)
			}).Should(want)
		},
		// darwin appends the kernel's own apple_* entries to every environment.
		Entry("every variable when no keys or prefixes are given", EnvironmentOptions{},
			And(HaveKeyWithValue("ACME_PROBE", "left=right"), HaveKeyWithValue("ACME_OTHER", "1"), HaveKey("PATH"))),
		Entry("only the named keys and prefixes", EnvironmentOptions{Keys: []string{"PATH"}, Prefixes: []string{"ACME_P"}},
			Equal(map[string]string{"ACME_PROBE": "left=right", "PATH": os.Getenv("PATH")})),
	)
})
