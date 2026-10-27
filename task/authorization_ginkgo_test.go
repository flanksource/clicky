package task_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"github.com/flanksource/clicky/route"
	"github.com/flanksource/clicky/task"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type ownerSource struct{}

func (ownerSource) Runs(context.Context, task.RunFilter) ([]task.RunMeta, error)  { return nil, nil }
func (ownerSource) Snapshot(context.Context, string) ([]task.TaskSnapshot, error) { return nil, nil }
func (ownerSource) Control(context.Context, string, task.ControlAction) error     { return nil }
func (ownerSource) AuthorizeRun(_ context.Context, run task.RunMeta) bool {
	return run.Owner == "alice"
}

var _ = Describe("task run authorization", func() {
	It("hides another owner's live run from listings, snapshots and controls", func() {
		owned := task.StartManagedRun("owned task", task.WithOwner("alice"))
		other := task.StartManagedRun("private task", task.WithOwner("bob"))
		DeferCleanup(func() { owned.Finish(task.StatusSuccess, nil); other.Finish(task.StatusSuccess, nil) })
		mux := http.NewServeMux()
		task.RegisterHandlersWithSource(route.NewRouter(mux), "/api/v1", ownerSource{})
		listing := httptest.NewRecorder()
		mux.ServeHTTP(listing, httptest.NewRequest(http.MethodGet, "/api/v1/tasks", nil))
		Expect(listing.Body.String()).To(ContainSubstring(owned.ID()))
		Expect(listing.Body.String()).NotTo(ContainSubstring(other.ID()))
		detail := httptest.NewRecorder()
		mux.ServeHTTP(detail, httptest.NewRequest(http.MethodGet, "/api/v1/tasks/"+other.ID(), nil))
		Expect(detail.Code).To(Equal(http.StatusNotFound))
		control := httptest.NewRecorder()
		mux.ServeHTTP(control, httptest.NewRequest(http.MethodPost, "/api/v1/tasks/"+other.ID()+"/control", strings.NewReader(`{"action":"stop"}`)))
		Expect(control.Code).To(Equal(http.StatusNotFound))
		Expect(other.Task().Status()).To(Equal(task.StatusRunning))
	})

	It("does not stream snapshots or output from another owner's run", func() {
		owned := task.StartManagedRun("owned stream", task.WithOwner("alice"))
		other := task.StartManagedRun("private stream", task.WithOwner("bob"))
		other.SetOutputProvider(func() task.OutputSnapshot { return task.OutputSnapshot{Stdout: "private output"} })
		DeferCleanup(func() { owned.Finish(task.StatusSuccess, nil); other.Finish(task.StatusSuccess, nil) })
		server := httptest.NewServer(task.SSEHandlerWithSource(ownerSource{}))
		DeferCleanup(server.Close)
		ctx, cancel := context.WithTimeout(context.Background(), 700*time.Millisecond)
		defer cancel()
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)
		Expect(err).NotTo(HaveOccurred())
		response, err := http.DefaultClient.Do(request)
		Expect(err).NotTo(HaveOccurred())
		defer response.Body.Close()
		data, _ := io.ReadAll(response.Body)
		Expect(string(data)).To(ContainSubstring(owned.ID()))
		Expect(string(data)).NotTo(ContainSubstring(other.ID()))
		Expect(string(data)).NotTo(ContainSubstring("private output"))
	})
})
