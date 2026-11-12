package telltale

import (
	"encoding/json"
	"net"
	"sync"
	"testing"
	"time"

	sdk "github.com/flanksource/telltale/sdk"
	"github.com/flanksource/telltale/sdk/protocol"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestTelltale(t *testing.T) { RegisterFailHandler(Fail); RunSpecs(t, "clicky telltale") }

type testSink struct {
	mu      sync.Mutex
	records []any
}

func (s *testSink) TryEmit(v any) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records = append(s.records, v)
	return true
}
func (*testSink) Blob(string, string, []byte, any) error { return nil }
func (*testSink) Report(any)                             {}
func (*testSink) Warn(string)                            {}

func (s *testSink) all() []any {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]any(nil), s.records...)
}

// loopback opens an authenticated in-memory protocol stream to a fresh agent.
func loopback() (agent *sdk.Agent, stream net.Conn, conn *protocol.Conn, hello protocol.Hello) {
	agent = sdk.Loopback()
	stream = agent.Connect()
	Expect(stream.SetDeadline(time.Now().Add(10 * time.Second))).To(Succeed())
	conn, hello, err := protocol.Authenticate(stream, "")
	Expect(err).NotTo(HaveOccurred())
	return agent, stream, conn, hello
}

func arm(conn *protocol.Conn, session, probe string, params any) {
	raw, err := json.Marshal(params)
	Expect(err).NotTo(HaveOccurred())
	Expect(conn.Send(protocol.TypeArm, protocol.Arm{Session: session, Probe: probe, Params: raw})).To(Succeed())
	frame, _, err := conn.Read()
	Expect(err).NotTo(HaveOccurred())
	Expect(frame.Type).To(Equal(protocol.TypeArmed))
	var armed protocol.Armed
	Expect(json.Unmarshal(frame.Body, &armed)).To(Succeed())
	Expect(armed.Error).To(BeEmpty())
}

// readRecords reads frames until one records batch arrives and returns its items.
func readRecords(conn *protocol.Conn) []json.RawMessage {
	for {
		frame, _, err := conn.Read()
		Expect(err).NotTo(HaveOccurred())
		if frame.Type != protocol.TypeRecords {
			continue
		}
		var batch protocol.Records
		Expect(json.Unmarshal(frame.Body, &batch)).To(Succeed())
		return batch.Items
	}
}

// boundedSink drops records once capacity are pending, like the SDK's ring;
// drain empties it and reports how many records it has accepted in total.
type boundedSink struct {
	testSink
	capacity int
	pending  int
	accepted int
}

func (s *boundedSink) TryEmit(any) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pending >= s.capacity {
		return false
	}
	s.pending++
	s.accepted++
	return true
}

func (s *boundedSink) drain() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pending = 0
	return s.accepted
}
