package clicky_test

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/flanksource/clicky"
	rpchttp "github.com/flanksource/clicky/rpc/http"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/pflag"
)

var _ = Describe("CLI timings", func() {
	It("binds --timings, defaulting a bare flag to the stderr header", func() {
		flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
		var options clicky.TimingsOptions
		options.BindFlags(flags)
		Expect(flags.Parse([]string{"--timings"})).To(Succeed())
		Expect(options.Timings).To(Equal(clicky.TimingsStderr))
		Expect(flags.Parse([]string{"--timings=report.json"})).To(Succeed())
		Expect(options.Timings).To(Equal("report.json"))
	})

	It("leaves the context untouched when timings are off", func() {
		var options clicky.TimingsOptions
		ctx := options.StartTimings(context.Background())
		_, accumulating := rpchttp.TimingsFromContext(ctx)
		Expect(accumulating).To(BeFalse())
		Expect(options.StopTimings()).To(Succeed())
	})

	It("writes the report once, however often it is stopped", func() {
		path := filepath.Join(GinkgoT().TempDir(), "timings.json")
		options := clicky.TimingsOptions{Timings: path}
		options.StartTimings(context.Background())
		Expect(options.StopTimings()).To(Succeed())
		Expect(os.Remove(path)).To(Succeed())
		Expect(options.StopTimings()).To(Succeed())
		_, err := os.Stat(path)
		Expect(os.IsNotExist(err)).To(BeTrue(), "a second stop must not rewrite the report")
	})

	It("reports the accumulated metrics, wall time and child-process usage", func() {
		path := filepath.Join(GinkgoT().TempDir(), "timings.json")
		options := clicky.TimingsOptions{Timings: path}
		ctx := options.StartTimings(context.Background())
		rpchttp.AddTiming(ctx, rpchttp.TimingMetric{Name: "git", Duration: 5 * time.Millisecond, Counters: []rpchttp.TimingCounter{
			{Name: "commands", Value: 3}, {Name: "max_rss_bytes", Value: 1 << 20, Aggregate: rpchttp.AggregateMax},
		}})
		Expect(exec.Command("git", "version").Run()).To(Succeed())
		Expect(options.StopTimings()).To(Succeed())

		body, err := os.ReadFile(path)
		Expect(err).NotTo(HaveOccurred())
		var report clicky.TimingsReport
		Expect(json.Unmarshal(body, &report)).To(Succeed())
		Expect(report.Metrics).To(Equal([]clicky.TimingsMetric{{Name: "git", DurationMs: 5, Counters: map[string]int64{"commands": 3, "max_rss_bytes": 1 << 20}}}))
		Expect(report.WallMs).To(BeNumerically(">", 0))
		Expect(report.Process.Self.MaxRSSBytes).To(BeNumerically(">", 0))
		Expect(report.Process.Children.MaxRSSBytes).To(BeNumerically(">", 0), "the git child ran and was waited for")
	})
})
