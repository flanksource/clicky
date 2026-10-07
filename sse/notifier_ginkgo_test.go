package sse

import (
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Notifier", func() {
	const quiet = 50 * time.Millisecond

	It("wakes every subscriber from a single Notify", func() {
		const subscribers = 5
		var notifier Notifier
		wakes := make([]<-chan struct{}, subscribers)
		for i := range wakes {
			wake, cancel := notifier.Subscribe()
			DeferCleanup(cancel)
			wakes[i] = wake
		}

		notifier.Notify()

		for i, wake := range wakes {
			Expect(wake).To(Receive(), "subscriber %d", i)
		}
	})

	It("coalesces notifications a subscriber has not consumed yet into one wake", func() {
		var notifier Notifier
		wake, cancel := notifier.Subscribe()
		DeferCleanup(cancel)

		notifier.Notify()
		notifier.Notify()
		notifier.Notify()

		Expect(wake).To(Receive())
		Consistently(wake, quiet).ShouldNot(Receive())
	})

	It("stops waking a subscriber once it unsubscribed, without disturbing the others", func() {
		var notifier Notifier
		gone, cancel := notifier.Subscribe()
		kept, cancelKept := notifier.Subscribe()
		DeferCleanup(cancelKept)

		cancel()
		cancel()
		notifier.Notify()

		Consistently(gone, quiet).ShouldNot(Receive())
		Expect(kept).To(Receive())
	})

	It("is safe for concurrent subscribe, notify and unsubscribe", func() {
		const workers = 16
		var notifier Notifier
		var wg sync.WaitGroup
		for range workers {
			wg.Add(2)
			go func() {
				defer wg.Done()
				for range 100 {
					wake, cancel := notifier.Subscribe()
					select {
					case <-wake:
					default:
					}
					cancel()
				}
			}()
			go func() {
				defer wg.Done()
				for range 100 {
					notifier.Notify()
				}
			}()
		}
		wg.Wait()
	})
})
