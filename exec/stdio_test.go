//go:build unix

package exec

import (
	"bufio"
	"io"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("WithStdioPipe", func() {
	It("round-trips a line through a supervised process and fires OnStarted", func() {
		ready := make(chan *Process, 1)
		s := NewExec("cat").WithStdioPipe().Supervise(SuperviseOptions{
			RestartPolicy: RestartNo,
			OnStarted:     func(p *Process) { ready <- p },
		})
		s.Start()
		DeferCleanup(s.Stop)

		// OnStarted hands back the running child with its stdio bound.
		var proc *Process
		Eventually(ready, 5*time.Second).Should(Receive(&proc))
		Expect(proc.Stdin()).ToNot(BeNil())
		Expect(proc.StdoutReader()).ToNot(BeNil())

		_, err := io.WriteString(proc.Stdin(), "ping\n")
		Expect(err).ToNot(HaveOccurred())

		// cat echoes stdin to the piped stdout and the task/output snapshot keeps
		// the same bytes without consuming the protocol reader.
		lineCh := make(chan string, 1)
		go func() {
			scanner := bufio.NewScanner(proc.StdoutReader())
			if scanner.Scan() {
				lineCh <- scanner.Text()
			}
		}()
		Eventually(lineCh, 5*time.Second).Should(Receive(Equal("ping")))
		Eventually(proc.GetStdout, 5*time.Second).Should(Equal("ping\n"))
	})

	It("makes Stdin/StdoutReader available only after the child starts", func() {
		// On the template (never run), the parent-side ends are nil.
		p := NewExec("cat").WithStdioPipe()
		Expect(p.Stdin()).To(BeNil())
		Expect(p.StdoutReader()).To(BeNil())
	})
})
