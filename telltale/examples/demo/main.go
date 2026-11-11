// Command demo is a clicky process observable by telltale and gops. Run it with
// TELLTALE_AGENT=1, then attach with `telltale trace run --pid <pid> ...` or
// `gops <pid>`. It runs a task, an entity operation, a subprocess, a commons
// HTTP request and a log line every few seconds until interrupted.
package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"time"

	"github.com/flanksource/clicky/entity"
	"github.com/flanksource/clicky/exec"
	"github.com/flanksource/clicky/shutdown"
	"github.com/flanksource/clicky/task"
	_ "github.com/flanksource/clicky/telltale/auto"
	commonscontext "github.com/flanksource/commons/context"
	commonshttp "github.com/flanksource/commons/http"
	"github.com/flanksource/commons/logger"
)

func main() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = fmt.Fprint(w, "ok") }))
	defer server.Close()
	logger.Infof("demo pid %d", os.Getpid())
	go func() {
		for i := 0; ; i++ {
			tick(server.URL, i)
			time.Sleep(3 * time.Second)
		}
	}()
	shutdown.WaitForSignal()
}

func tick(url string, i int) {
	handle := task.StartTask(fmt.Sprintf("demo tick %d", i), func(_ commonscontext.Context, t *task.Task) (bool, error) {
		t.SetProgress(1, 2)
		exec.NewExec("sleep", "1").Run()
		t.SetProgress(2, 2)
		return true, nil
	})
	if _, err := handle.GetResult(); err != nil {
		logger.Errorf("demo task: %v", err)
	}
	_, _ = entity.ObserveOperation(entity.ContextWithOperationSurface(context.Background(), "cli"), func() entity.OperationEvent {
		return entity.OperationEvent{Entity: "widget", Verb: "get", TargetID: fmt.Sprint(i)}
	}, func() (any, error) { return i, nil })
	if _, err := commonshttp.NewClient().R(context.Background()).Get(url + "/widgets"); err != nil {
		logger.Errorf("demo request: %v", err)
	}
	logger.Infof("demo tick %d done", i)
}
