package task

import (
	"fmt"
	"reflect"

	"github.com/flanksource/commons/logger"
	"github.com/samber/lo"

	"github.com/flanksource/clicky/api"
	"github.com/flanksource/clicky/formatters"
)

// prettyWithLogOffset renders the task's status line with buffered-log
// children, considering only log entries at index >= from. It returns the
// rendered text plus the total number of buffered entries observed, so
// PlainRender can advance its per-task cursor without re-reading (and racing)
// the buffer. Pretty() delegates with from=0 for the full history. Caller must
// hold t.mu.
func (t *Task) prettyWithLogOffset(from int) (api.Text, int) {
	if pretty, ok := t.result.(api.PrettyShort); ok {
		rv := reflect.ValueOf(pretty)
		if rv.Kind() != reflect.Ptr || (!rv.IsNil() && rv.Pointer() != reflect.ValueOf(t).Pointer()) {
			return api.Text{}.Add(pretty.PrettyShort()), from
		}
	}

	if pretty, ok := t.result.(formatters.PrettyMixin); ok {
		rv := reflect.ValueOf(pretty)
		if rv.Kind() != reflect.Ptr || (!rv.IsNil() && rv.Pointer() != reflect.ValueOf(t).Pointer()) {
			return pretty.Pretty(), from
		}
	}

	var text api.Text

	duration := t.getDuration()
	displayName := t.name
	if t.modelName != "" {
		displayName = t.modelName + " " + displayName
	}
	if t.prompt != "" {
		truncatedPrompt := t.prompt
		displayName += fmt.Sprintf(" %q", truncatedPrompt)
	}
	if t.description != "" {
		displayName += ": " + t.description
	}

	text.Content = displayName
	text.Style = "max-w-[tw-20ch] truncate-suffix"
	text = text.Space().Append(fmt.Sprintf("%-10s", duration), "")

	// Note: We can't call t.Status() here since it would try to acquire the same mutex
	// So we directly access t.status and handle the health check inline
	if health, ok := t.result.(HealthMixin); ok {
		switch health.Health() {
		case HealthOK:
			t.status = StatusSuccess
		case HealthWarning:
			t.status = StatusWarning
		case HealthError:
			t.status = StatusFailed
		case HealthPending:
			t.status = StatusPending
		}
	}
	text = t.status.Apply(text)

	level := t.ctx.Logger.GetLevel()
	// Add logs as children if present from bufferedLogger
	bufferedLogs := t.getBufferedLogger().GetLogs()
	total := len(bufferedLogs)
	if from > total {
		from = total // per-level ring eviction may have shrunk the buffer
	}
	bufferedLogs = bufferedLogs[from:]
	maxLogs := 5
	if t.ctx.Logger.IsLevelEnabled(logger.Trace2) {
		maxLogs = 1000
	} else if t.ctx.Logger.IsLevelEnabled(logger.Trace1) {
		maxLogs = 100
	} else if t.ctx.Logger.IsLevelEnabled(logger.Trace) {
		maxLogs = 50
	} else if t.ctx.Logger.IsLevelEnabled(logger.Debug) {
		maxLogs = 20
	}
	if len(bufferedLogs) > maxLogs {
		excess := len(bufferedLogs) - maxLogs
		bufferedLogs = bufferedLogs[len(bufferedLogs)-maxLogs:]
		bufferedLogs = append(bufferedLogs, logger.BufferedLogEntry{Message: fmt.Sprintf("\t... %d more log lines ...", excess)})
	}
	for _, log := range bufferedLogs {
		if level < log.Level {
			continue
		}
		// Hide info logs when task completed successfully and log level is Info (0) or higher
		if t.status == StatusSuccess && level <= logger.Info && log.Level == logger.Info {
			continue
		}
		var logStyle string

		switch log.Level {
		case logger.Error:
			logStyle = "text-red-600"
		case logger.Warn:
			logStyle = "text-yellow-600"
		default:
			logStyle = "text-gray-400"
		}

		text.Children = append(text.Children, api.Text{
			Content: fmt.Sprintf("\n%s", lo.Ellipsis(log.Message, 500)),
			Style:   logStyle,
		})
	}

	return text, total
}
