package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
)

// TestSSEServer_InitializeRoundTrip starts the real SSE server on a
// random port, opens the event stream, parses the announced endpoint,
// posts an `initialize` JSON-RPC request, and verifies the response is
// delivered back over SSE. This is the minimum end-to-end smoke for the
// new transport — anything finer-grained is covered by the existing
// stdio handler tests.
func TestSSEServer_InitializeRoundTrip(t *testing.T) {
	port := freePort(t)

	cfg := DefaultConfig()
	cfg.Transport = TransportConfig{Type: "sse", Address: "127.0.0.1", Port: port}
	cfg.Security.RequireConfirmation = false

	root := &cobra.Command{Use: "ssetest"}
	srv := NewMCPServer(cfg, root)
	if err := srv.Initialize(); err != nil {
		t.Fatalf("Initialize: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	serverErr := make(chan error, 1)
	go func() { serverErr <- srv.Start(ctx) }()

	addr := fmt.Sprintf("127.0.0.1:%d", port)
	base := "http://" + addr
	waitForListener(t, addr)

	// Open the SSE stream and read the endpoint announcement.
	streamReq, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/sse", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(streamReq)
	if err != nil {
		t.Fatalf("open SSE: %v", err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })

	reader := bufio.NewReader(resp.Body)
	endpoint := readSSEEvent(t, reader, "endpoint")
	if !strings.HasPrefix(endpoint, "/messages?session_id=") {
		t.Fatalf("unexpected endpoint payload: %q", endpoint)
	}

	// Post an initialize request and check the SSE side delivers a response.
	initReq := `{"jsonrpc":"2.0","id":1,"method":"initialize"}`
	postResp, err := http.Post(base+endpoint, "application/json", strings.NewReader(initReq))
	if err != nil {
		t.Fatalf("post initialize: %v", err)
	}
	postResp.Body.Close()
	if postResp.StatusCode != http.StatusAccepted {
		t.Fatalf("POST /messages status = %d, want 202", postResp.StatusCode)
	}

	payload := readSSEEvent(t, reader, "message")
	var rpc map[string]any
	if err := json.Unmarshal([]byte(payload), &rpc); err != nil {
		t.Fatalf("invalid JSON in SSE payload %q: %v", payload, err)
	}
	if rpc["jsonrpc"] != "2.0" {
		t.Fatalf("missing jsonrpc field: %v", rpc)
	}
	result, ok := rpc["result"].(map[string]any)
	if !ok {
		t.Fatalf("missing result: %v", rpc)
	}
	if result["protocolVersion"] == nil {
		t.Fatalf("initialize result missing protocolVersion: %v", result)
	}
}

func TestSameOriginOrNoOrigin(t *testing.T) {
	cases := []struct {
		name   string
		origin string
		host   string
		want   bool
	}{
		{name: "no origin", host: "127.0.0.1:8080", want: true},
		{name: "same origin", origin: "http://127.0.0.1:8080", host: "127.0.0.1:8080", want: true},
		{name: "different port", origin: "http://127.0.0.1:3000", host: "127.0.0.1:8080", want: false},
		{name: "different host", origin: "http://example.com", host: "127.0.0.1:8080", want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, "http://"+tc.host+"/sse", nil)
			if err != nil {
				t.Fatal(err)
			}
			req.Host = tc.host
			if tc.origin != "" {
				req.Header.Set("Origin", tc.origin)
			}
			if got := sameOriginOrNoOrigin(req); got != tc.want {
				t.Fatalf("sameOriginOrNoOrigin() = %v, want %v", got, tc.want)
			}
		})
	}
}

// readSSEEvent advances the reader until it sees `event: <wantEvent>`
// followed by a `data:` line, and returns the data payload. Fails the
// test if the stream ends or 5s pass without the event.
func readSSEEvent(t *testing.T, r *bufio.Reader, wantEvent string) string {
	t.Helper()
	type result struct {
		data string
		err  error
	}
	done := make(chan result, 1)
	go func() {
		var pendingEvent string
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				done <- result{err: err}
				return
			}
			line = strings.TrimRight(line, "\r\n")
			switch {
			case strings.HasPrefix(line, "event: "):
				pendingEvent = strings.TrimPrefix(line, "event: ")
			case strings.HasPrefix(line, "data: "):
				data := strings.TrimPrefix(line, "data: ")
				if pendingEvent == wantEvent {
					done <- result{data: data}
					return
				}
				pendingEvent = ""
			}
		}
	}()

	select {
	case got := <-done:
		if got.err != nil {
			t.Fatalf("SSE read: %v", got.err)
		}
		return got.data
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for SSE event %q", wantEvent)
		return ""
	}
}

// freePort grabs and immediately releases an OS-assigned TCP port. The
// brief race between release and re-bind is acceptable for tests.
func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	_ = l.Close()
	return port
}

// waitForListener polls until the server is accepting TCP connections,
// so the test isn't flaky when Start hasn't bound yet.
func waitForListener(t *testing.T, addr string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("server never bound: %s", addr)
}
