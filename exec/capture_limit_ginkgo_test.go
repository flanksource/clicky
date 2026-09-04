package exec

import (
	"bufio"
	"bytes"
	"io"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Process capture limits", func() {
	It("preserves unbounded capture by default", func() {
		process := NewExec("/bin/sh", "-c", "printf 0123456789; printf abcdefghij >&2").Run()

		Expect(process.GetStdout()).To(Equal("0123456789"))
		Expect(process.GetStderr()).To(Equal("abcdefghij"))
	})

	It("retains only the newest bytes from each captured stream", func() {
		process := NewExec("/bin/sh", "-c", "printf 0123456789; printf abcdefghij >&2").
			WithCaptureLimit(5).
			Run()

		Expect(process.GetStdout()).To(Equal("56789"))
		Expect(process.GetStderr()).To(Equal("fghij"))
		Expect(cap(process.captureOutput.stdout.data)).To(BeNumerically("<=", 5))
		Expect(cap(process.captureOutput.stderr.data)).To(BeNumerically("<=", 5))
	})

	It("retains the newest bytes across incremental writes", func() {
		capture := NewExecLogger()
		capture.setCaptureLimit(5)
		writer := capture.GetStderrWriter()

		_, err := writer.Write([]byte("012"))
		Expect(err).ToNot(HaveOccurred())
		_, err = writer.Write([]byte("345"))
		Expect(err).ToNot(HaveOccurred())
		_, err = writer.Write([]byte("6789"))

		Expect(err).ToNot(HaveOccurred())
		Expect(capture.GetStderr()).To(Equal("56789"))
		Expect(cap(capture.stderr.data)).To(BeNumerically("<=", 5))
	})

	It("tees complete streams while bounding only the captured snapshots", func() {
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		process := NewExec("/bin/sh", "-c", "printf 0123456789; printf abcdefghij >&2").
			Stream(&stdout, &stderr).
			WithCaptureLimit(5).
			Run()

		Expect(stdout.String()).To(Equal("0123456789"))
		Expect(stderr.String()).To(Equal("abcdefghij"))
		Expect(process.GetStdout()).To(Equal("56789"))
		Expect(process.GetStderr()).To(Equal("fghij"))
	})

	It("preserves the limit on wrapper clones", func() {
		run := NewExec("printf").WithCaptureLimit(4).AsWrapper()

		result, err := run("abcdefgh")

		Expect(err).ToNot(HaveOccurred())
		Expect(result.Stdout).To(Equal("efgh"))
	})

	It("rejects non-positive limits", func() {
		Expect(func() {
			NewExec("true").WithCaptureLimit(0)
		}).To(Panic())
		Expect(func() {
			NewExecLogger().setCaptureLimit(0)
		}).To(Panic())
	})

	It("delivers complete piped stdout while bounding its captured snapshot", func() {
		ready := make(chan *Process, 1)
		s := NewExec("cat").WithStdioPipe().WithCaptureLimit(5).Supervise(SuperviseOptions{
			RestartPolicy: RestartNo,
			OnStarted:     func(p *Process) { ready <- p },
		})
		s.Start()
		DeferCleanup(s.Stop)

		var proc *Process
		Eventually(ready, 5*time.Second).Should(Receive(&proc))
		_, err := io.WriteString(proc.Stdin(), "0123456789\n")
		Expect(err).ToNot(HaveOccurred())

		lineCh := make(chan string, 1)
		go func() {
			scanner := bufio.NewScanner(proc.StdoutReader())
			if scanner.Scan() {
				lineCh <- scanner.Text()
			}
		}()
		Eventually(lineCh, 5*time.Second).Should(Receive(Equal("0123456789")))
		Eventually(proc.GetStdout, 5*time.Second).Should(Equal("6789\n"))
	})
})
