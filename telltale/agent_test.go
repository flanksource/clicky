package telltale

import (
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	commonshttp "github.com/flanksource/commons/http"
	"github.com/flanksource/commons/har"
	gops "github.com/google/gops/agent"
	"github.com/google/gops/signal"
	sdk "github.com/flanksource/telltale/sdk"
	"github.com/flanksource/telltale/sdk/protocol"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("telltale agent", func() {
	It("advertises the clicky probes and gops-equivalent ops", func() {
		agent, stream, _, hello := loopback()
		defer agent.Close()
		defer stream.Close()
		var probes, ops []string
		for _, probe := range hello.Probes {
			probes = append(probes, probe.Name)
		}
		for _, op := range hello.Ops {
			ops = append(ops, op.Name)
		}
		Expect(probes).To(ContainElements("exec", "entity", "task", "http", "logs"))
		Expect(ops).To(ContainElements("memstats", "version", "stack", "gc", "setgc"))
	})

	It("answers a stack op with every goroutine", func() {
		agent, stream, conn, _ := loopback()
		defer agent.Close()
		defer stream.Close()
		Expect(conn.Send(protocol.TypeCall, protocol.Call{CallID: "stack", Op: "stack", Params: json.RawMessage(`{}`)})).To(Succeed())
		frame, _, err := conn.Read()
		Expect(err).NotTo(HaveOccurred())
		Expect(frame.Type).To(Equal(protocol.TypeResult))
		var result protocol.Result
		Expect(json.Unmarshal(frame.Body, &result)).To(Succeed())
		Expect(result.Error).To(BeEmpty())
		var dump string
		Expect(json.Unmarshal(result.Value, &dump)).To(Succeed())
		Expect(dump).To(ContainSubstring("goroutine "))
	})

	It("captures a commons client configured for TLS after instrumentation", func() {
		previous := http.DefaultTransport
		DeferCleanup(func() { http.DefaultTransport = previous })
		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "ok") }))
		defer server.Close()
		agent, stream, conn, _ := loopback()
		defer agent.Close()
		defer stream.Close()
		arm(conn, "http", "http", map[string]any{})
		sdk.InstrumentDefaultTransport()
		response, err := commonshttp.NewClient().InsecureSkipVerify(true).R(GinkgoT().Context()).Get(server.URL + "/widgets")
		Expect(err).NotTo(HaveOccurred())
		Expect(response.StatusCode).To(Equal(http.StatusOK))
		items := readRecords(conn)
		Expect(items).To(HaveLen(1))
		var entry har.Entry
		Expect(json.Unmarshal(items[0], &entry)).To(Succeed())
		Expect(entry.Request.URL).To(HaveSuffix("/widgets"))
		Expect(entry.Response.Status).To(Equal(http.StatusOK))
	})

	It("starts a gops agent that answers the gops wire protocol", func() {
		dir := filepath.Join(GinkgoT().TempDir(), "gops")
		Expect(StartGops(gops.Options{ConfigDir: dir})).To(Succeed())
		defer gops.Close()
		port, err := os.ReadFile(filepath.Join(dir, strconv.Itoa(os.Getpid())))
		Expect(err).NotTo(HaveOccurred())
		conn, err := net.DialTimeout("tcp", "127.0.0.1:"+strings.TrimSpace(string(port)), 5*time.Second)
		Expect(err).NotTo(HaveOccurred())
		defer conn.Close()
		Expect(conn.SetDeadline(time.Now().Add(10 * time.Second))).To(Succeed())
		_, err = conn.Write([]byte{signal.StackTrace})
		Expect(err).NotTo(HaveOccurred())
		reply, err := io.ReadAll(conn)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(reply)).To(ContainSubstring("goroutine "))
	})
})
