package aichat

import (
	"fmt"

	captainai "github.com/flanksource/captain/pkg/ai"
)

// agentStreamResult is what draining one agent turn captured for the finish part.
type agentStreamResult struct {
	sessionID string
	usage     *captainai.Usage
	cost      float64
}

// agentEmitter maps captain ai.Events onto v6 SSE parts. It tracks the currently
// open text / reasoning block and a per-turn tool counter: captain Events carry
// no tool-call id and agent backends do not stream tool results, so each
// tool_use mints a synthetic id and closes its card immediately.
type agentEmitter struct {
	sse        *sseWriter
	textID     string
	textOpen   bool
	reasonID   string
	reasonOpen bool
	block      int
	toolSeq    int
	openTools  map[string]bool // tool-call ids announced but not yet resulted
}

func (e *agentEmitter) onText(delta string) error {
	if err := e.closeReasoning(); err != nil {
		return err
	}
	if !e.textOpen {
		e.textID = fmt.Sprintf("a-text-%d", e.block)
		e.block++
		e.textOpen = true
		if err := e.sse.textStart(e.textID); err != nil {
			return err
		}
	}
	return e.sse.textDelta(e.textID, delta)
}

func (e *agentEmitter) onReasoning(delta string) error {
	if err := e.closeText(); err != nil {
		return err
	}
	if !e.reasonOpen {
		e.reasonID = fmt.Sprintf("a-reason-%d", e.block)
		e.block++
		e.reasonOpen = true
		if err := e.sse.reasoningStart(e.reasonID); err != nil {
			return err
		}
	}
	return e.sse.reasoningDelta(e.reasonID, delta)
}

// onToolUse announces a tool call, keyed by the backend's tool-call id so a
// later result correlates to it. A missing id falls back to a per-turn counter.
func (e *agentEmitter) onToolUse(id, name string, input map[string]any) error {
	if err := e.closeOpen(); err != nil {
		return err
	}
	if id == "" {
		e.toolSeq++
		id = fmt.Sprintf("a-tool-%d", e.toolSeq)
	}
	if err := e.sse.toolInputAvailable(id, name, input); err != nil {
		return err
	}
	e.openTools[id] = true
	return nil
}

// onToolResult closes the card for a prior call with its real output. A result
// without a correlation id cannot attach to a card and is dropped.
func (e *agentEmitter) onToolResult(id, text string, success bool) error {
	if err := e.closeOpen(); err != nil {
		return err
	}
	if id == "" {
		return nil
	}
	output := map[string]any{"output": text}
	if !success {
		output["isError"] = true
	}
	if err := e.sse.toolOutputAvailable(id, output); err != nil {
		return err
	}
	delete(e.openTools, id)
	return nil
}

// closeDangling emits a synthetic output for any tool call that never received a
// result (a backend that omits results, or a turn that ended mid-tool), so the
// UI does not leave cards pending.
func (e *agentEmitter) closeDangling() error {
	for id := range e.openTools {
		if err := e.sse.toolOutputAvailable(id, map[string]any{"status": "executed"}); err != nil {
			return err
		}
	}
	e.openTools = map[string]bool{}
	return nil
}

func (e *agentEmitter) closeText() error {
	if !e.textOpen {
		return nil
	}
	e.textOpen = false
	return e.sse.textEnd(e.textID)
}

func (e *agentEmitter) closeReasoning() error {
	if !e.reasonOpen {
		return nil
	}
	e.reasonOpen = false
	return e.sse.reasoningEnd(e.reasonID)
}

func (e *agentEmitter) closeOpen() error {
	if err := e.closeText(); err != nil {
		return err
	}
	return e.closeReasoning()
}

// drainAgentEvents maps the captain event stream onto SSE parts and returns the
// captured session id, usage and cost. An EventError closes any open block and
// is returned as an error so the caller surfaces it as an SSE error part.
func drainAgentEvents(sse *sseWriter, ch <-chan captainai.Event) (agentStreamResult, error) {
	em := &agentEmitter{sse: sse, openTools: map[string]bool{}}
	var res agentStreamResult
	for ev := range ch {
		switch ev.Kind {
		case captainai.EventText:
			if ev.Text == "" {
				continue
			}
			if err := em.onText(ev.Text); err != nil {
				return res, err
			}
		case captainai.EventThinking:
			if ev.Text == "" {
				continue
			}
			if err := em.onReasoning(ev.Text); err != nil {
				return res, err
			}
		case captainai.EventToolUse:
			if ev.Tool == "" {
				continue
			}
			if err := em.onToolUse(ev.ToolCallID, ev.Tool, ev.Input); err != nil {
				return res, err
			}
		case captainai.EventToolResult:
			if err := em.onToolResult(ev.ToolCallID, ev.Text, ev.Success); err != nil {
				return res, err
			}
		case captainai.EventSystem:
			if ev.SessionID != "" {
				res.sessionID = ev.SessionID
			}
		case captainai.EventResult:
			if ev.SessionID != "" {
				res.sessionID = ev.SessionID
			}
			res.usage = ev.Usage
			res.cost = ev.CostUSD
		case captainai.EventError:
			_ = em.closeOpen()
			_ = em.closeDangling()
			return res, fmt.Errorf("%s", ev.Error)
		}
	}
	if err := em.closeOpen(); err != nil {
		return res, err
	}
	if err := em.closeDangling(); err != nil {
		return res, err
	}
	return res, nil
}
