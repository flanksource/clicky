package task_test

import (
	"context"
	"errors"
	"fmt"

	"github.com/flanksource/clicky/task"
	commonscontext "github.com/flanksource/commons/context"
)

func ExampleTaskFromContext() {
	run := task.StartTask("Index files", func(ctx commonscontext.Context, _ *task.Task) (int, error) {
		return indexExampleFiles(ctx, []string{"settings.yaml", "routes.yaml"})
	})
	count, err := run.GetResult()
	fmt.Println(count, err)
	// Output: 2 <nil>
}

func indexExampleFiles(ctx context.Context, files []string) (int, error) {
	current := task.TaskFromContext(ctx)
	if current == nil {
		return 0, errors.New("indexing files requires a task context")
	}
	for i, name := range files {
		current.SetDescription(name)
		current.SetProgress(i+1, len(files))
	}
	return len(files), nil
}
