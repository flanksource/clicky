package exec

import (
	"context"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("exec observers", func() {
	It("reports an exit for each failure before launch", func() {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		cases := []*Process{
			NewExec("true").WithContext(ctx),
			NewExec("true").WithShell("missing-shell-for-observer-test"),
			NewExec("true").WithCwd("/missing-directory-for-observer-test"),
		}
		for _, p := range cases {
			var events []ExecEvent
			release := ObserveExec(func(event ExecEvent) { events = append(events, event) })
			p.Run()
			release()
			Expect(events).To(HaveLen(1))
			Expect(events[0].Phase).To(Equal(ExecExit))
			Expect(events[0].PID).To(BeZero())
			Expect(events[0].Err).To(MatchError(p.Err))
			Expect(events[0].Err).To(HaveOccurred())
		}
	})

	It("waits for in-flight delivery before release returns", func() {
		entered, unblock, released, done := make(chan struct{}), make(chan struct{}), make(chan struct{}), make(chan struct{})
		release := ObserveExec(func(ExecEvent) { close(entered); <-unblock })
		go func() { NewExec("true").Run(); close(done) }()
		Eventually(entered).Should(BeClosed())
		go func() { release(); close(released) }()
		Consistently(released, 100*time.Millisecond).ShouldNot(BeClosed())
		close(unblock)
		Eventually(released).Should(BeClosed())
		Eventually(done).Should(BeClosed())
	})

	It("retains valid UTF-8 at the tail boundary for both streams", func() {
		var events []ExecEvent
		release := ObserveExec(func(event ExecEvent) { events = append(events, event) })
		defer release()
		input := "€" + strings.Repeat("x", TailBytes-1)
		p := NewExec("sh", "-c", `value=$(cat); printf %s "$value"; printf %s "$value" >&2`).WithStdin(strings.NewReader(input))
		p.Run()
		Expect(events).To(HaveLen(2))
		Expect(utf8.ValidString(events[1].Stdout)).To(BeTrue())
		Expect(events[1].Stdout).To(Equal(strings.Repeat("x", TailBytes-1)))
		Expect(events[1].Stderr).To(Equal(events[1].Stdout))
	})

	It("reads a wrapped capture without splitting its first rune", func() {
		var buffer captureBuffer
		buffer.setLimit(6)
		_, err := buffer.Write([]byte("abcd€xyz"))
		Expect(err).NotTo(HaveOccurred())
		Expect(observerTail(&buffer)).To(Equal("€xyz"))
		_, err = buffer.Write([]byte("q"))
		Expect(err).NotTo(HaveOccurred())
		Expect(observerTail(&buffer)).To(Equal("xyzq"))
	})
	It("reports start and exit with command details and environment keys", func() {
		var events []ExecEvent
		release := ObserveExec(func(event ExecEvent) { events = append(events, event) })
		defer release()
		p := NewExec("sh", "-c", "echo hi; exit 3").WithExactEnv(map[string]string{"SECRET_KEY": "secret-value"}).Run()
		Expect(events).To(HaveLen(2))
		Expect(events[0].Phase).To(Equal(ExecPhase("start")))
		Expect(events[1].Phase).To(Equal(ExecPhase("exit")))
		for _, event := range events {
			Expect(event.Name).To(Equal(p.Name()))
			Expect(event.Command).To(Equal("sh"))
			Expect(event.Args).To(Equal([]string{"-c", "echo hi; exit 3"}))
			Expect(event.EnvKeys).To(Equal([]string{"SECRET_KEY"}))
			Expect(event.PID).To(BeNumerically(">", 0))
			Expect(event.StartedAt).ToNot(BeZero())
		}
		Expect(events[1].ExitCode).To(Equal(3))
		Expect(events[1].Err).To(HaveOccurred())
		Expect(events[1].Stdout).To(Equal("hi\n"))
		Expect(events[1].Duration).To(BeNumerically(">", 0))
	})

	It("stops delivery on release and recovers observer panics", func() {
		var events []ExecEvent
		release := ObserveExec(func(event ExecEvent) { events = append(events, event) })
		panicRelease := ObserveExec(func(ExecEvent) { panic("observer panic") })
		NewExec("true").Run()
		panicRelease()
		release()
		release()
		NewExec("true").Run()
		Expect(events).To(HaveLen(2))
	})

	It("reports one exit after Start and Wait and for supervised processes", func() {
		events := make(chan ExecEvent, 4)
		release := ObserveExec(func(event ExecEvent) { events <- event })
		defer release()
		p := NewExec("true")
		Expect(p.Start()).To(Succeed())
		Expect(p.Wait()).To(Succeed())
		s := NewExec("true").Supervise(SuperviseOptions{RestartPolicy: RestartNo})
		s.Start()
		s.Wait()
		Expect(events).To(HaveLen(4))
		for range 2 {
			Expect((<-events).Phase).To(Equal(ExecPhase("start")))
			Expect((<-events).Phase).To(Equal(ExecPhase("exit")))
		}
	})

	It("caps captured output to the newest bytes", func() {
		var events []ExecEvent
		release := ObserveExec(func(event ExecEvent) { events = append(events, event) })
		defer release()
		input := strings.Repeat("x", TailBytes) + "end"
		NewExec("cat").WithStdin(strings.NewReader(input)).Run()
		Expect(events).To(HaveLen(2))
		Expect(events[1].Stdout).To(Equal(input[len(input)-TailBytes:]))
	})
})

func TestPublishExecUnobservedAllocations(t *testing.T) {
	if allocs := testing.AllocsPerRun(100, func() { publishExec(nil, "start") }); allocs != 0 {
		t.Fatalf("unobserved publish allocated %v times", allocs)
	}
}
