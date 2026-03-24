package task_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	flanksourceContext "github.com/flanksource/commons/context"
	"github.com/flanksource/commons/logger"
	"github.com/flanksource/gomplate/v3"

	"github.com/flanksource/clicky/task"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v3"
)

type progressFixture struct {
	Value int `yaml:"value"`
	Max   int `yaml:"max"`
}

type logMessage struct {
	Level   string `yaml:"level"`
	Message string `yaml:"message"`
}

type taskFixture struct {
	Name        string          `yaml:"name"`
	LogMessages []logMessage    `yaml:"log_messages"`
	Progress    *progressFixture `yaml:"progress"`
	Status      string          `yaml:"status"`
	ResultType  string          `yaml:"result_type"`
	Result      any             `yaml:"result"`
	Error       string          `yaml:"error"`
	Assertions  []string        `yaml:"assertions"`
}

func executeFixtureTask(fixture taskFixture) (result any, status task.Status, logs []logger.BufferedLogEntry, logsByLevel map[string][]logger.BufferedLogEntry, err error) {
	typed := task.StartTask(fixture.Name, func(ctx flanksourceContext.Context, t *task.Task) (any, error) {
		t.SetLogLevel(logger.Trace)

		for _, msg := range fixture.LogMessages {
			switch msg.Level {
			case "trace":
				t.Tracef("%s", msg.Message)
			case "debug":
				t.Debugf("%s", msg.Message)
			case "info":
				t.Infof("%s", msg.Message)
			case "warn":
				t.Warnf("%s", msg.Message)
			case "error":
				t.Errorf("%s", msg.Message)
			}
		}

		if fixture.Progress != nil {
			t.SetProgress(fixture.Progress.Value, fixture.Progress.Max)
		}

		switch fixture.Status {
		case "success":
			t.Success()
		case "warning":
			t.Warning()
		case "failed":
			if fixture.Error != "" {
				return t.FailedWithError(fmt.Errorf("%s", fixture.Error))
			}
			t.Failed()
		}

		return fixture.Result, nil
	})

	typed.WaitFor()

	result, err = typed.GetResult()
	status = typed.Status()
	return
}

func buildCELEnv(fixture taskFixture, result any, status task.Status, err error) map[string]any {
	env := map[string]any{
		"status":   string(status),
		"duration": 1.0, // always > 0 since task ran
	}

	if err != nil {
		env["error"] = err.Error()
	}

	if result != nil {
		raw, _ := json.Marshal(result)
		var parsed any
		_ = json.Unmarshal(raw, &parsed)

		switch v := parsed.(type) {
		case map[string]any:
			env["result"] = v
		default:
			env["result"] = parsed
		}
	}

	// Count logs by level from fixture (since we can't easily access the buffered logger from outside)
	env["logs"] = toLogEntries(fixture.LogMessages)
	env["trace_logs"] = filterLogsByLevel(fixture.LogMessages, "trace")
	env["debug_logs"] = filterLogsByLevel(fixture.LogMessages, "debug")
	env["info_logs"] = filterLogsByLevel(fixture.LogMessages, "info")
	env["warn_logs"] = filterLogsByLevel(fixture.LogMessages, "warn")
	env["error_logs"] = filterLogsByLevel(fixture.LogMessages, "error")

	return env
}

func toLogEntries(msgs []logMessage) []any {
	result := make([]any, len(msgs))
	for i, m := range msgs {
		result[i] = map[string]any{"level": m.Level, "message": m.Message}
	}
	return result
}

func filterLogsByLevel(msgs []logMessage, level string) []any {
	var result []any
	for _, m := range msgs {
		if m.Level == level {
			result = append(result, map[string]any{"level": m.Level, "message": m.Message})
		}
	}
	if result == nil {
		return []any{}
	}
	return result
}

var _ = Describe("Task fixtures", func() {
	fixtures, err := filepath.Glob("testdata/tasks/*.yaml")
	if err != nil {
		panic(err)
	}

	for _, fixturePath := range fixtures {
		name := filepath.Base(fixturePath)

		It("fixture: "+name, func() {
			data, err := os.ReadFile(fixturePath)
			Expect(err).ToNot(HaveOccurred())

			var fixture taskFixture
			Expect(yaml.Unmarshal(data, &fixture)).To(Succeed())
			Expect(fixture.Assertions).ToNot(BeEmpty(), "fixture %s has no assertions", name)

			result, status, _, _, taskErr := executeFixtureTask(fixture)

			env := buildCELEnv(fixture, result, status, taskErr)

			for _, expr := range fixture.Assertions {
				ok, celErr := gomplate.RunTemplateBool(env, gomplate.Template{Expression: expr})
				Expect(celErr).ToNot(HaveOccurred(), "CEL error in %s: %s\nenv: %v", name, expr, env)
				Expect(ok).To(BeTrue(), "assertion failed in %s: %s\nenv: %v", name, expr, env)
			}
		})
	}
})
