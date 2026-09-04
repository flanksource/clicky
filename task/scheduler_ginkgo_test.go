package task

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	flanksourceContext "github.com/flanksource/commons/context"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// kindSeq keeps each spec's runner kind unique: RegisterRunner deliberately
// panics on a duplicate, and the registry is process-global.
var kindSeq atomic.Int64

func uniqueKind(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, kindSeq.Add(1))
}

// controlledRunner is a runner whose task blocks until it is released, so a
// spec can hold a run open and fire the schedule again underneath it.
type controlledRunner struct {
	mu      sync.Mutex
	release chan struct{}
	starts  atomic.Int64
}

func newControlledRunner() *controlledRunner {
	return &controlledRunner{release: make(chan struct{})}
}

func (r *controlledRunner) run(_ flanksourceContext.Context, _ Schedule, group *Group) error {
	r.mu.Lock()
	release := r.release
	r.mu.Unlock()

	r.starts.Add(1)
	typed := TypedGroup[any]{Group: group}
	typed.Add("work", func(_ flanksourceContext.Context, t *Task) (any, error) {
		<-release
		t.Success()
		return nil, nil
	})
	return nil
}

// releaseAll unblocks every task started so far and re-arms for the next one.
func (r *controlledRunner) releaseAll() {
	r.mu.Lock()
	close(r.release)
	r.release = make(chan struct{})
	r.mu.Unlock()
}

// fixedClock is a manually advanced clock so firing decisions are exact rather
// than a race against wall time.
type fixedClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *fixedClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fixedClock) Advance(d time.Duration) {
	c.mu.Lock()
	c.now = c.now.Add(d)
	c.mu.Unlock()
}

var _ = Describe("Scheduler", func() {
	var (
		clock    *fixedClock
		store    *testStore
		runner   *controlledRunner
		kind     string
		sched    *Scheduler
		ctx      flanksourceContext.Context
		original *Manager
		cancel   context.CancelFunc
	)

	BeforeEach(func() {
		original = global
		global = newTestManager(2)

		clock = &fixedClock{now: time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)}
		store = newTestStore()
		runner = newControlledRunner()
		kind = uniqueKind("spec")
		RegisterRunner(kind, runner.run)

		var stdctx context.Context
		stdctx, cancel = context.WithCancel(context.Background())
		SetStore(stdctx, store)
		ctx = flanksourceContext.NewContext(stdctx)

		sched = NewScheduler(SchedulerOptions{Store: store, Now: clock.Now})
	})

	AfterEach(func() {
		runner.releaseAll()
		SetStore(context.Background(), nil)
		cancel()
		global.stopRender()
		global = original
	})

	newSchedule := func(name string) Schedule {
		return Schedule{Name: name, Kind: kind, Cron: "@every 1h", Enabled: true}
	}

	It("rejects a schedule whose cron, timezone or policy is not usable", func() {
		Expect(sched.Add(ctx, Schedule{Kind: kind, Cron: "@every 1h"})).
			To(MatchError(ContainSubstring("name is required")))

		bad := newSchedule("bad-cron")
		bad.Cron = "not a cron"
		Expect(sched.Add(ctx, bad)).To(MatchError(ContainSubstring("invalid cron")))

		badZone := newSchedule("bad-zone")
		badZone.Timezone = "Mars/Olympus"
		Expect(sched.Add(ctx, badZone)).To(MatchError(ContainSubstring("unknown timezone")))

		badOverlap := newSchedule("bad-overlap")
		badOverlap.Overlap = "sometimes"
		Expect(sched.Add(ctx, badOverlap)).To(MatchError(ContainSubstring("unknown overlap policy")))

		unknown := newSchedule("no-runner")
		unknown.Kind = "never-registered"
		Expect(sched.Add(ctx, unknown)).To(MatchError(ContainSubstring("no runner registered")))
	})

	It("fires when the scheduled time arrives, and not before", func() {
		Expect(sched.Add(ctx, newSchedule("hourly"))).To(Succeed())

		sched.RunDue(ctx)
		Expect(runner.starts.Load()).To(BeZero(), "fired before its time")

		clock.Advance(time.Hour)
		sched.RunDue(ctx)

		Eventually(runner.starts.Load).Should(BeEquivalentTo(1))
		Eventually(func() []Fire { return store.firesFor("hourly") }).Should(HaveLen(1))
		Expect(store.firesFor("hourly")[0].Outcome).To(Equal(FireStarted))
		Expect(store.firesFor("hourly")[0].RunID).ToNot(BeEmpty())
	})

	It("does not fire a disabled schedule", func() {
		disabled := newSchedule("paused")
		disabled.Enabled = false
		Expect(sched.Add(ctx, disabled)).To(Succeed())

		clock.Advance(2 * time.Hour)
		sched.RunDue(ctx)

		Consistently(runner.starts.Load).Should(BeZero())
	})

	It("stamps the schedule name and kind on every run it starts", func() {
		Expect(sched.Add(ctx, newSchedule("labelled"))).To(Succeed())
		clock.Advance(time.Hour)
		sched.RunDue(ctx)

		Eventually(func() []RunMeta { return RunsRaw(RunFilter{Kind: kind}) }).Should(HaveLen(1))
		run := RunsRaw(RunFilter{Kind: kind})[0]
		Expect(run.Name).To(Equal("labelled"))
		Expect(run.Labels).To(HaveKeyWithValue("schedule", "labelled"))

		// The label is what a per-schedule run history filters on.
		Expect(RunsRaw(RunFilter{Labels: map[string]string{"schedule": "labelled"}})).To(HaveLen(1))
	})

	Context("when a fire lands while the previous run is still going", func() {
		It("skips it under the default policy, and records why", func() {
			Expect(sched.Add(ctx, newSchedule("skipper"))).To(Succeed())

			clock.Advance(time.Hour)
			sched.RunDue(ctx)
			Eventually(runner.starts.Load).Should(BeEquivalentTo(1))

			clock.Advance(time.Hour)
			sched.RunDue(ctx)

			Eventually(func() []Fire { return store.firesFor("skipper") }).Should(HaveLen(2))
			skipped := store.firesFor("skipper")[1]
			Expect(skipped.Outcome).To(Equal(FireSkipped))
			Expect(skipped.Reason).To(ContainSubstring("still in progress"))
			Consistently(runner.starts.Load).Should(BeEquivalentTo(1))
		})

		It("runs it after the previous one under the queue policy", func() {
			queued := newSchedule("queuer")
			queued.Overlap = OverlapQueue
			Expect(sched.Add(ctx, queued)).To(Succeed())

			clock.Advance(time.Hour)
			sched.RunDue(ctx)
			Eventually(runner.starts.Load).Should(BeEquivalentTo(1))

			clock.Advance(time.Hour)
			sched.RunDue(ctx)
			Eventually(func() []Fire { return store.firesFor("queuer") }).Should(HaveLen(2))
			Expect(store.firesFor("queuer")[1].Reason).To(ContainSubstring("queued"))

			// Letting the first run finish is what releases the queued fire.
			runner.releaseAll()
			Eventually(func() int64 {
				sched.RunDue(ctx)
				return runner.starts.Load()
			}).Should(BeEquivalentTo(2))
		})

		It("records each deferred fire as itself and keeps the oldest queued", func() {
			start := clock.Now()
			backlog := newSchedule("backlog")
			backlog.Overlap = OverlapQueue
			Expect(sched.Add(ctx, backlog)).To(Succeed())

			clock.Advance(time.Hour) // 10:00 — runs
			sched.RunDue(ctx)
			Eventually(runner.starts.Load).Should(BeEquivalentTo(1))

			clock.Advance(time.Hour) // 11:00 — queued behind it
			sched.RunDue(ctx)
			clock.Advance(time.Hour) // 12:00 — deferred while 11:00 is still owed
			sched.RunDue(ctx)

			Eventually(func() []Fire { return store.firesFor("backlog") }).Should(HaveLen(3))
			fires := store.firesFor("backlog")
			Expect(fires[1].ScheduledFor).To(BeTemporally("==", start.Add(2*time.Hour)))
			Expect(fires[2].ScheduledFor).To(BeTemporally("==", start.Add(3*time.Hour)),
				"the second deferral is its own fire, not a replay of the queued one")

			// The queued instant is the one still owed, so it is the one that runs.
			runner.releaseAll()
			Eventually(func() []Fire {
				sched.RunDue(ctx)
				return store.firesFor("backlog")
			}).Should(HaveLen(4))
			Expect(store.firesFor("backlog")[3].ScheduledFor).
				To(BeTemporally("==", start.Add(2*time.Hour)))
		})

		It("cancels the previous run under the cancel-previous policy", func() {
			replacing := newSchedule("replacer")
			replacing.Overlap = OverlapCancelPrevious
			Expect(sched.Add(ctx, replacing)).To(Succeed())

			clock.Advance(time.Hour)
			sched.RunDue(ctx)
			Eventually(runner.starts.Load).Should(BeEquivalentTo(1))
			first := RunsRaw(RunFilter{Kind: kind})[0].ID

			clock.Advance(time.Hour)
			sched.RunDue(ctx)

			// TEMP: debug
			time.Sleep(400 * time.Millisecond)
			if runner.starts.Load() < 2 {
				buf := make([]byte, 1<<20)
				n := runtime.Stack(buf, true)
				_ = os.WriteFile("../.tmp/stacks.txt", buf[:n], 0o644)
			}

			Eventually(runner.starts.Load).Should(BeEquivalentTo(2))
			Eventually(func() string {
				for _, run := range RunsRaw(RunFilter{Kind: kind}) {
					if run.ID == first {
						return run.Status
					}
				}
				return ""
			}).ShouldNot(Equal(string(StatusRunning)))
		})
	})

	Context("when scheduled times passed while the process was down", func() {
		It("ignores them by default", func() {
			missed := clock.Now().Add(-3 * time.Hour)
			schedule := newSchedule("no-catchup")
			schedule.NextRun = &missed

			Expect(sched.Add(ctx, schedule)).To(Succeed())
			sched.RunDue(ctx)

			Consistently(runner.starts.Load).Should(BeZero())
		})

		It("replays exactly one fire under catch-up once, however many were missed", func() {
			missed := clock.Now().Add(-72 * time.Hour)
			schedule := newSchedule("catchup")
			schedule.CatchUp = CatchUpOnce
			schedule.NextRun = &missed

			Expect(sched.Add(ctx, schedule)).To(Succeed())
			sched.RunDue(ctx)

			Eventually(runner.starts.Load).Should(BeEquivalentTo(1))
			Eventually(func() []Fire { return store.firesFor("catchup") }).Should(HaveLen(1))
			Expect(store.firesFor("catchup")[0].ScheduledFor).To(BeTemporally("==", missed))

			// Three days of missed hourly fires must not become 72 runs.
			runner.releaseAll()
			Eventually(func() int64 {
				sched.RunDue(ctx)
				return runner.starts.Load()
			}).Should(BeEquivalentTo(1))
			Consistently(runner.starts.Load).Should(BeEquivalentTo(1))
		})
	})

	It("loads persisted schedules and reports the ones that no longer validate", func() {
		Expect(store.SaveSchedule(ctx, newSchedule("good"))).To(Succeed())

		broken := newSchedule("broken")
		broken.Cron = "still not a cron"
		Expect(store.SaveSchedule(ctx, broken)).To(Succeed())

		err := sched.Load(ctx)
		Expect(err).To(MatchError(ContainSubstring("broken")))

		names := []string{}
		for _, schedule := range sched.Schedules() {
			names = append(names, schedule.Name)
		}
		Expect(names).To(ConsistOf("good"), "a broken schedule must not stop a good one loading")
	})

	It("stops firing a schedule that was removed", func() {
		Expect(sched.Add(ctx, newSchedule("transient"))).To(Succeed())
		Expect(sched.Remove(ctx, "transient")).To(Succeed())

		clock.Advance(2 * time.Hour)
		sched.RunDue(ctx)

		Consistently(runner.starts.Load).Should(BeZero())
		Expect(sched.Schedules()).To(BeEmpty())
	})

	Context("persisting the definitions it holds", func() {
		It("saves an added schedule with the fire it has just computed", func() {
			Expect(sched.Add(ctx, newSchedule("durable"))).To(Succeed())

			stored := store.schedule("durable")
			Expect(stored.Name).To(Equal("durable"))
			Expect(stored.NextRun).ToNot(BeNil())
			Expect(*stored.NextRun).To(BeTemporally("==", clock.Now().Add(time.Hour)))
		})

		It("advances the stored fire times as it fires", func() {
			Expect(sched.Add(ctx, newSchedule("advancing"))).To(Succeed())

			clock.Advance(time.Hour)
			sched.RunDue(ctx)
			Eventually(runner.starts.Load).Should(BeEquivalentTo(1))

			stored := store.schedule("advancing")
			Expect(stored.LastRun).ToNot(BeNil(), "a run that started is what LastRun means")
			Expect(*stored.LastRun).To(BeTemporally("==", clock.Now()))
			Expect(*stored.NextRun).To(BeTemporally("==", clock.Now().Add(time.Hour)))
		})

		It("deletes a removed schedule so a restart does not resurrect it", func() {
			Expect(sched.Add(ctx, newSchedule("doomed"))).To(Succeed())
			Expect(store.schedule("doomed").Name).To(Equal("doomed"))

			Expect(sched.Remove(ctx, "doomed")).To(Succeed())
			Expect(store.schedule("doomed").Name).To(BeEmpty())
		})
	})

	Context("its start/stop lifecycle", func() {
		It("returns from Stop when it was never started", func() {
			done := make(chan struct{})
			go func() {
				defer close(done)
				sched.Stop()
			}()
			Eventually(done).Should(BeClosed())
		})

		It("runs one loop however often it is started, and stops once", func() {
			sched.Start(ctx)
			sched.Start(ctx)

			// A second loop would close the same done channel and panic; a
			// clean Stop is the assertion that only one ever ran.
			Expect(sched.Stop).ToNot(Panic())
			Expect(sched.Stop).ToNot(Panic())
		})
	})

	It("does not run a schedule removed between the due scan and the fire", func() {
		Expect(sched.Add(ctx, newSchedule("vanishing"))).To(Succeed())
		clock.Advance(time.Hour)

		// fire resolves the entry itself, so removing it first is exactly the
		// race RunDue would otherwise lose.
		Expect(sched.Remove(ctx, "vanishing")).To(Succeed())
		sched.fire(ctx, "vanishing", clock.Now())

		Consistently(runner.starts.Load).Should(BeZero())
		Expect(store.firesFor("vanishing")).To(BeEmpty())
	})
})
