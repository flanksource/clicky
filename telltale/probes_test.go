package telltale

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	osexec "os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/flanksource/clicky/entity"
	"github.com/flanksource/clicky/exec"
	"github.com/flanksource/clicky/task"
	commonscontext "github.com/flanksource/commons/context"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// psStartKey is how telltale's process sampler keys pid: ps start time to the second.
func psStartKey(pid int) string {
	out, err := osexec.Command("ps", "-o", "lstart=", "-p", strconv.Itoa(pid)).Output()
	Expect(err).NotTo(HaveOccurred())
	started, err := time.ParseInLocation("Mon Jan _2 15:04:05 2006", strings.TrimSpace(string(out)), time.Local)
	Expect(err).NotTo(HaveOccurred())
	return fmt.Sprintf("%d@%d", pid, started.Unix())
}

var _ = Describe("exec probe", func() {
	It("validates the glob, tail limit and phases", func() {
		for _, raw := range []string{`{"name":"["}`, `{"tailBytes":-1}`, `{"tailBytes":65537}`, `{"phases":["spawn"]}`} {
			_, err := (execProbe{}).Arm(context.Background(), json.RawMessage(raw), &testSink{})
			Expect(err).To(HaveOccurred(), raw)
		}
	})

	It("keeps only the configured tail and masks secrets in arguments and output", func() {
		event := exec.ExecEvent{Phase: exec.ExecExit, Name: "mysql", Args: []string{"--password", "hunter2", "-p", "private"},
			Stdout: "abc password=output-secret", Stderr: "token=error-secret", ExitCode: 3}
		record := makeExecRecord(event, 4096)
		Expect(record.Args).To(Equal([]string{"--password", "****", "-p", "****"}))
		Expect(record.StdoutTail).NotTo(ContainSubstring("output-secret"))
		Expect(record.StderrTail).NotTo(ContainSubstring("error-secret"))
		Expect(*record.ExitCode).To(Equal(3))
		Expect(makeExecRecord(exec.ExecEvent{Phase: exec.ExecExit, Stdout: "abcdef"}, 3).StdoutTail).To(Equal("def"))
	})

	It("keeps a shell command identifiable and masks it, including before launch", func() {
		const command = "mysql --password hunter2"
		record := makeExecRecord(exec.ExecEvent{Phase: exec.ExecExit, Name: command, Command: command}, 0)
		Expect(record.Name).To(Equal("mysql --password ****"))
		Expect(record.Command).To(Equal("mysql --password ****"))
	})

	It("hands a started child off with the process key telltale's sampler computes", func() {
		sink := &testSink{}
		var psKey string
		release, err := (execProbe{}).Arm(context.Background(), json.RawMessage(`{"name":"sleep","phases":["start"]}`), sink)
		Expect(err).NotTo(HaveOccurred())
		observedPS := exec.ObserveExec(func(event exec.ExecEvent) {
			if event.Phase == exec.ExecStart && strings.HasSuffix(event.Name, "sleep") {
				psKey = psStartKey(event.PID)
			}
		})
		p := exec.NewExec("sleep", "0.3").Run()
		observedPS()
		Expect(release()).To(Succeed())
		records := sink.all()
		Expect(records).To(HaveLen(1))
		start := records[0].(ExecRecord)
		Expect(start.Phase).To(Equal(exec.ExecStart))
		Expect(start.PID).To(Equal(p.Pid()))
		Expect(start.ExitCode).To(BeNil())
		Expect(start.ProcessKey).To(Equal(psKey))
	})
})

var _ = Describe("entity probe", func() {
	It("records a failed operation with masked parameters and its surface", func() {
		sink := &testSink{}
		release, err := (entityProbe{}).Arm(context.Background(), json.RawMessage(`{"entity":"widget","errorsOnly":true}`), sink)
		Expect(err).NotTo(HaveOccurred())
		defer func() { Expect(release()).To(Succeed()) }()
		ctx := entity.ContextWithOperationSurface(context.Background(), "http")
		readOnly := false
		run := func(name string, fail bool) {
			_, _ = entity.ObserveOperation(ctx, func() entity.OperationEvent {
				return entity.OperationEvent{Entity: name, Verb: "update", TargetID: "w-1", ReadOnly: &readOnly,
					Parameters: map[string]string{"password": "hunter2", "color": "blue"}, Args: []string{"--token", "abc"}}
			}, func() (any, error) {
				if fail {
					return nil, errors.New("update rejected")
				}
				return "ok", nil
			})
		}
		run("widget", false)
		run("gadget", true)
		run("widget", true)
		records := sink.all()
		Expect(records).To(HaveLen(1))
		record := records[0].(EntityRecord)
		record.StartedAt, record.DurationMs = time.Time{}, 0
		Expect(record).To(Equal(EntityRecord{Entity: "widget", Verb: "update", Surface: "http", TargetID: "w-1", ReadOnly: &readOnly,
			Parameters: map[string]string{"password": "****", "color": "blue"}, Args: []string{"--token", "****"}, Error: "update rejected"}))
	})
})

var _ = Describe("task probe", func() {
	It("emits a baseline, transitions and throttled progress without logs or output", func() {
		group := task.StartGroup[bool]("probe baseline group")
		existing := group.StartStep("probe existing")
		sink := &testSink{}
		release, err := (taskProbe{}).Arm(context.Background(), json.RawMessage(`{"name":"probe *","progressIntervalMs":60000}`), sink)
		Expect(err).NotTo(HaveOccurred())
		handle := task.StartTask("probe work", func(_ commonscontext.Context, t *task.Task) (bool, error) {
			t.Infof("working")
			t.SetProgress(1, 3)
			t.SetProgress(2, 3)
			return true, nil
		})
		_, err = handle.GetResult()
		Expect(err).NotTo(HaveOccurred())
		existing.Finish(nil)
		Eventually(func() int { return len(sink.all()) }).Should(Equal(5))
		Expect(release()).To(Succeed())
		type seen struct {
			Phase  task.TaskPhase
			Name   string
			Status string
		}
		var got []seen
		for _, item := range sink.all() {
			record := item.(TaskRecord)
			Expect(record.Logs).To(BeEmpty())
			got = append(got, seen{record.Phase, record.Name, record.Status})
		}
		Expect(got).To(Equal([]seen{
			{taskBaseline, "probe existing", "running"},
			{task.TaskStatusChanged, "probe work", "running"},
			{task.TaskProgressed, "probe work", "running"},
			{task.TaskStatusChanged, "probe work", "success"},
			{task.TaskStatusChanged, "probe existing", "success"},
		}))
	})

	It("delivers a baseline larger than the record ring once the ring drains", func() {
		const baselineTasks = 300
		group := task.StartGroup[bool]("probe ring group")
		for i := range baselineTasks {
			step := group.StartStep(fmt.Sprintf("probe ring %d", i))
			defer step.Finish(nil)
		}
		sink := &boundedSink{capacity: 256}
		release, err := (taskProbe{}).Arm(context.Background(), json.RawMessage(`{"name":"probe ring *"}`), sink)
		Expect(err).NotTo(HaveOccurred())
		defer func() { Expect(release()).To(Succeed()) }()
		Eventually(sink.drain, "5s", "20ms").Should(BeNumerically(">=", baselineTasks))
	})

	It("masks exec command arguments in task details", func() {
		record, err := makeTaskRecord(task.TaskStatusChanged, task.TaskSnapshot{
			Details: exec.ExecTaskDetails{Command: "mysql", Args: []string{"--password", "hunter2"}},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(string(record.Details.(json.RawMessage))).To(And(ContainSubstring(`"args":["--password","****"]`), Not(ContainSubstring("hunter2"))))
	})

	It("rejects an invalid glob and a negative progress interval", func() {
		for _, raw := range []string{`{"name":"["}`, `{"group":"["}`, `{"progressIntervalMs":-1}`} {
			_, err := (taskProbe{}).Arm(context.Background(), json.RawMessage(raw), &testSink{})
			Expect(err).To(HaveOccurred(), raw)
		}
	})
})
