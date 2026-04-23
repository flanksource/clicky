package main

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	flanksourceContext "github.com/flanksource/commons/context"

	"github.com/flanksource/clicky"
	"github.com/flanksource/clicky/task"
)

func main() {
	mode := os.Getenv("PROMPT_MODE")
	if mode == "" {
		fmt.Fprintln(os.Stderr, "PROMPT_MODE not set")
		os.Exit(2)
	}

	task.SetNoColor(true)
	task.SetNoProgress(false)

	switch mode {
	case "choose_confirm":
		runChoose()
	case "choose_cancel":
		runChoose()
	case "multi_confirm":
		runMultiChoose()
	case "text_validate":
		runValidatedText()
	case "secret":
		runSecretText()
	case "task_takeover":
		runTaskTakeover()
	default:
		fmt.Fprintf(os.Stderr, "unknown PROMPT_MODE: %s\n", mode)
		os.Exit(2)
	}
}

func runChoose() {
	value, ok := clicky.PromptSelect([]string{"alpha", "beta", "gamma"}, clicky.PromptSelectOptions[string]{
		Title: "Pick one",
	})
	fmt.Fprintf(os.Stderr, "\nRESULT=%s OK=%v\n", value, ok)
}

func runValidatedText() {
	value, ok := clicky.PromptText(clicky.PromptTextOptions{
		Title: "Name",
		Validate: func(value string) error {
			if strings.TrimSpace(value) == "" {
				return errors.New("value required")
			}
			return nil
		},
	})
	fmt.Fprintf(os.Stderr, "\nRESULT=%s OK=%v\n", value, ok)
}

func runMultiChoose() {
	values, ok := clicky.PromptMultiSelect([]string{"alpha", "beta", "gamma"}, clicky.PromptMultiSelectOptions[string]{
		Title: "Pick checks",
		Limit: 2,
	})
	fmt.Fprintf(os.Stderr, "\nRESULT=%s OK=%v\n", strings.Join(values, ","), ok)
}

func runSecretText() {
	value, ok := clicky.PromptText(clicky.PromptTextOptions{
		Title:  "Secret",
		Secret: true,
	})
	fmt.Fprintf(os.Stderr, "\nRESULT_LEN=%d OK=%v\n", len(value), ok)
}

func runTaskTakeover() {
	release := make(chan struct{})
	clicky.StartTask[string]("rendering", func(ctx flanksourceContext.Context, t *clicky.Task) (string, error) {
		t.SetDescription("before prompt TAKEOVER-MARKER")
		select {
		case <-release:
			t.SetDescription("after prompt TAKEOVER-MARKER")
			return "done", nil
		case <-ctx.Done():
			return "", ctx.Err()
		}
	})

	time.Sleep(600 * time.Millisecond)

	value, ok := clicky.PromptSelect([]string{"continue-marker", "stop-marker"}, clicky.PromptSelectOptions[string]{
		Title: "Take over terminal",
	})

	close(release)
	_ = clicky.WaitForGlobalCompletion()
	fmt.Fprintf(os.Stderr, "\nRESULT=%s OK=%v\n", value, ok)
}
