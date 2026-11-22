package valkey_test

import (
	"bytes"
	"context"
	"crypto/tls"
	"net"
	"os"
	"slices"
	"sync/atomic"
	"time"

	"github.com/alicebob/miniredis/v2"
	miniserver "github.com/alicebob/miniredis/v2/server"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	valkeygo "github.com/valkey-io/valkey-go"

	"github.com/flanksource/clicky/cache"
	"github.com/flanksource/clicky/valkey"
)

var _ = Describe("Valkey store budgets", func() {
	var (
		server  *miniredis.Miniredis
		options valkeygo.ClientOption
	)

	BeforeEach(func() {
		var err error
		server, err = miniredis.Run()
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(server.Close)
		options = valkeygo.ClientOption{
			InitAddress: []string{server.Addr()}, DisableCache: true, ForceSingleClient: true,
			PipelineMultiplex: -1, DisableRetry: true,
		}
	})

	DescribeTable("returns promptly when the caller cancels an in-flight command",
		func(command string, alwaysPipelining bool, run func(cache.Store, context.Context) error) {
			options.AlwaysPipelining = alwaysPipelining
			client, err := valkeygo.NewClient(options)
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(client.Close)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			reply := make(chan struct{})
			defer close(reply)
			var commands atomic.Int32
			server.Server().SetPreHook(func(_ *miniserver.Peer, cmd string, _ ...string) bool {
				if cmd == command {
					commands.Add(1)
					cancel()
					<-reply
				}
				return false
			})
			store := valkey.NewStoreWithTimeouts(client, 3*time.Second, 5*time.Second)
			done := make(chan error, 1)
			go func() { done <- run(store, ctx) }()
			Eventually(done, time.Second).Should(Receive(MatchError(context.Canceled)))
			Expect(commands.Load()).To(Equal(int32(1)))
		},
		Entry("SET with default pipelining", "SET", false, setLargeValue),
		Entry("SET with AlwaysPipelining", "SET", true, setLargeValue),
		Entry("MGET with default pipelining", "MGET", false, readValue),
		Entry("MGET with AlwaysPipelining", "MGET", true, readValue),
	)

	It("finishes a large SET whose reply takes longer than the base budget", func() {
		client, err := valkeygo.NewClient(options)
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(client.Close)
		server.Server().SetPreHook(func(_ *miniserver.Peer, cmd string, _ ...string) bool {
			if cmd == "SET" {
				time.Sleep(350 * time.Millisecond)
			}
			return false
		})
		Expect(setLargeValue(valkey.NewStore(client), context.Background())).To(Succeed())
	})

	DescribeTable("retries a transport deadline returned before the attempt context expires",
		func(deadline error) {
			var failNextRead atomic.Bool
			options.DialCtxFn = func(ctx context.Context, addr string, dialer *net.Dialer, _ *tls.Config) (net.Conn, error) {
				conn, err := dialer.DialContext(ctx, "tcp", addr)
				if err != nil {
					return nil, err
				}
				return &deadlineConn{Conn: conn, failNextRead: &failNextRead, deadline: deadline}, nil
			}
			client, err := valkeygo.NewClient(options)
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(client.Close)
			Expect(server.Set("key", "value")).To(Succeed())
			failNextRead.Store(true)
			store := valkey.NewStoreWithTimeouts(client, time.Second, 5*time.Second)
			Expect(collectMGet(store.MGet(context.Background(), slices.Values([]string{"key"})))).To(Equal([]mgetEntry{
				{Key: "key", Value: "value", Found: true},
			}))
			Expect(failNextRead.Load()).To(BeFalse())
		},
		Entry("context deadline", context.DeadlineExceeded),
		Entry("socket deadline", os.ErrDeadlineExceeded),
	)
})

func setLargeValue(store cache.Store, ctx context.Context) error {
	return store.Set(ctx, "key", bytes.Repeat([]byte("v"), 1<<20), 0)
}

func readValue(store cache.Store, ctx context.Context) error {
	for _, err := range store.MGet(ctx, slices.Values([]string{"key"})) {
		return err
	}
	return nil
}

type deadlineConn struct {
	net.Conn
	failNextRead *atomic.Bool
	deadline     error
}

func (c *deadlineConn) Read(p []byte) (int, error) {
	if c.failNextRead.Swap(false) {
		return 0, c.deadline
	}
	return c.Conn.Read(p)
}
