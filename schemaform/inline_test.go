package schemaform

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const notificationSchema = `{
  "type": "object",
  "x-discriminator": "kind",
  "required": ["kind", "name"],
  "properties": {
    "kind": {"type": "string", "title": "Channel", "enum": ["webhook", "email"], "x-enum-labels": {"webhook": "Webhook", "email": "Email"}},
    "name": {"type": "string", "title": "Name", "minLength": 3},
    "severity": {
      "type": "string", "title": "Severity", "enum": ["warning", "critical"], "default": "warning",
      "x-on-change": [{"when": {"const": "critical"}, "show": ["pager"], "require": ["pager"], "else": {"hide": ["pager"], "reset": ["pager"]}}]
    },
    "pager": {"type": "string", "title": "Pager"},
    "retries": {"type": "integer", "title": "Retries", "default": 2},
    "labels": {"type": "object", "title": "Labels", "additionalProperties": {"type": "string"}}
  },
  "allOf": [
    {"if": {"properties": {"kind": {"const": "email"}}, "required": ["kind"]},
     "then": {"properties": {"to": {"type": "array", "title": "Recipients", "items": {"type": "string"}}}, "required": ["to"]}}
  ]
}`

// manyOptional is one required field and optional ones that cannot all fit.
func manyOptional(optional int) string {
	props := []string{`"name": {"type": "string", "title": "Name"}`}
	for i := 1; i <= optional; i++ {
		props = append(props, fmt.Sprintf(`"opt%02d": {"type": "string", "title": "Option %02d"}`, i, i))
	}
	return `{"type": "object", "required": ["name"], "properties": {` + strings.Join(props, ",") + `}}`
}

// harness drives an inlineForm the way bubbletea would, without a terminal.
type harness struct {
	m  *inlineForm
	st *State
}

func newHarness(schema string, height int) *harness {
	GinkgoHelper()
	return newHarnessLoading(schema, height, nil)
}

// newHarnessLoading is newHarness with a LoadSchema for x-clicky-schema slots.
func newHarnessLoading(schema string, height int, load SchemaLoader) *harness {
	GinkgoHelper()
	s, err := Parse(json.RawMessage(schema))
	Expect(err).NotTo(HaveOccurred())
	st := newState(s, ApplyDefaults(s, map[string]any{}), FilterEvaluator())
	_, _, _, err = st.object(nil)
	Expect(err).NotTo(HaveOccurred())
	b := &builder{st: st, lookup: &lookupBinding{ctx: context.Background(), cache: map[string]LookupResult{}}, inline: true, loadSchema: load}
	steps, err := b.build(s, nil, "")
	Expect(err).NotTo(HaveOccurred())
	_, slots := expand(steps)
	_, err = syncSlots(context.Background(), slots)
	Expect(err).NotTo(HaveOccurred())
	h := &harness{m: newInlineForm(context.Background(), steps, inlineFormOptions{}), st: st}
	h.send(h.m.Init())
	h.update(tea.WindowSizeMsg{Width: 100, Height: height})
	return h
}

func (h *harness) update(msg tea.Msg) {
	_, cmd := h.m.Update(msg)
	h.send(cmd)
}

// send resolves cmd and feeds its messages back. A command that does not
// return promptly (a cursor blink's tick) is dropped.
func (h *harness) send(cmd tea.Cmd) {
	if cmd == nil {
		return
	}
	out := make(chan tea.Msg, 1)
	go func() { out <- cmd() }()
	select {
	case msg := <-out:
		switch msg := msg.(type) {
		case nil, tea.QuitMsg:
		case tea.BatchMsg:
			for _, c := range msg {
				h.send(c)
			}
		default:
			h.update(msg)
		}
	case <-time.After(50 * time.Millisecond):
	}
}

func (h *harness) keys(keys ...tea.KeyType) {
	for _, k := range keys {
		h.update(tea.KeyMsg{Type: k})
	}
}

func (h *harness) typed(text string) {
	h.update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(text)})
}

func (h *harness) focused() string {
	switch h.m.cursor.kind {
	case rowAdvanced:
		return "(advanced)"
	case rowSubmit:
		return "(submit)"
	}
	return h.m.cursor.f.key
}

// promptSchema is `prompt run`'s shape: --vars takes a JSON object whose
// schema depends on the prompt chosen.
const promptSchema = `{
  "type": "object",
  "required": ["prompt"],
  "properties": {
    "prompt": {"type": "string", "title": "Prompt", "enum": ["review", "summarize"]},
    "vars": {"type": "string", "title": "Variables", "x-clicky-schema": {"source": "vars", "scope": ["prompt"]}},
    "model": {"type": "string", "title": "Model", "default": "opus"}
  }
}`

var promptInputs = map[string]string{
	"review":    `{"type": "object", "required": ["diff"], "properties": {"diff": {"type": "string", "title": "Diff"}, "strict": {"type": "boolean", "title": "Strict", "default": true}}}`,
	"summarize": `{"type": "object", "properties": {"length": {"type": "integer", "title": "Length", "default": 3}}}`,
}

// loadPromptInputs answers from promptInputs, recording each request.
func loadPromptInputs(requests *[]SchemaRequest) SchemaLoader {
	return func(_ context.Context, req SchemaRequest) (json.RawMessage, error) {
		*requests = append(*requests, req)
		return json.RawMessage(promptInputs[req.Values["prompt"].(string)]), nil
	}
}

var _ = Describe("inlineForm x-clicky-schema slots", func() {
	It("loads a prompt's variables once it is chosen and puts them right after it", func() {
		var requests []SchemaRequest
		h := newHarnessLoading(promptSchema, 40, loadPromptInputs(&requests))
		Expect(requests).To(BeEmpty(), "nothing loads until a prompt is chosen")

		h.keys(tea.KeyEnter, tea.KeyEnter) // review
		Expect(h.focused()).To(Equal("diff"), "the prompt's first variable is the next row")
		h.typed("HEAD~1")
		h.keys(tea.KeyEnter, tea.KeyEnter) // Strict keeps its default
		Expect(h.focused()).To(Equal("model"))
		h.keys(tea.KeyEnter)

		Expect(h.m.done).To(BeTrue())
		Expect(h.st.prune()).To(Equal(map[string]any{
			"prompt": "review", "model": "opus",
			"vars": map[string]any{"diff": "HEAD~1", "strict": true},
		}))
		Expect(requests).To(Equal([]SchemaRequest{{Source: "vars", Values: map[string]any{"prompt": "review"}}}))
	})

	It("rebuilds the variables when the prompt changes, dropping the old ones", func() {
		var requests []SchemaRequest
		h := newHarnessLoading(promptSchema, 40, loadPromptInputs(&requests))
		h.keys(tea.KeyEnter, tea.KeyEnter) // review
		h.typed("HEAD~1")
		h.keys(tea.KeyShiftTab)
		Expect(h.focused()).To(Equal("prompt"))
		h.keys(tea.KeyEnter, tea.KeyDown, tea.KeyEnter) // summarize
		Expect(h.focused()).To(Equal("length"))
		Expect(h.st.get([]string{"vars"})).To(Equal(map[string]any{"length": float64(3)}))
		Expect(h.m.View()).NotTo(ContainSubstring("Diff"))
	})

	It("ends the form with the load's error", func() {
		h := newHarnessLoading(promptSchema, 40, func(context.Context, SchemaRequest) (json.RawMessage, error) {
			return nil, fmt.Errorf("prompt store unreachable")
		})
		h.keys(tea.KeyEnter, tea.KeyEnter)
		Expect(h.m.failure).To(MatchError(ContainSubstring("prompt store unreachable")))
	})
})

var _ = Describe("inlineForm", func() {
	It("fills a form one row at a time, promoting fields a branch or listener requires", func() {
		h := newHarness(notificationSchema, 40)
		Expect(h.focused()).To(Equal("kind"), "the discriminator leads")
		h.keys(tea.KeyEnter, tea.KeyDown, tea.KeyEnter) // open, Email
		Expect(h.focused()).To(Equal("name"))
		h.typed("ops-alerts")
		h.keys(tea.KeyEnter)
		Expect(h.focused()).To(Equal("severity"))
		h.keys(tea.KeyEnter, tea.KeyDown, tea.KeyEnter) // critical reveals and requires Pager
		Expect(h.focused()).To(Equal("pager"), "Pager is promoted right after Severity")
		h.typed("primary")
		h.keys(tea.KeyEnter, tea.KeyEnter) // Retries keeps 2
		Expect(h.focused()).To(Equal("labels"))
		h.keys(tea.KeyEnter)
		h.typed("team=sre")
		h.keys(tea.KeyEnter)
		Expect(h.focused()).To(Equal("to"), "Recipients, required by the Email branch")
		h.keys(tea.KeyEnter)
		h.typed("a@example.com")
		h.keys(tea.KeyEnter)

		Expect(h.m.done).To(BeTrue(), "enter on the last field submits")
		Expect(h.st.prune()).To(Equal(map[string]any{
			"kind": "email", "name": "ops-alerts", "severity": "critical", "pager": "primary",
			"retries": float64(2), "labels": map[string]any{"team": "sre"}, "to": []any{"a@example.com"},
		}))
		Expect(h.m.View()).To(And(ContainSubstring("Email"), ContainSubstring("ops-alerts"), ContainSubstring("team=sre")))
	})

	It("opens a focused select only on enter, and moves past a closed one with ↓", func() {
		h := newHarness(notificationSchema, 40)
		Expect(h.m.View()).NotTo(ContainSubstring("Webhook"), "the focused Channel select is closed")
		h.keys(tea.KeyEnter)
		Expect(h.m.View()).To(And(ContainSubstring("Webhook"), ContainSubstring("Email")))
		h.keys(tea.KeyEsc)
		Expect(h.m.editing).To(BeFalse(), "esc closes the editor")
		h.keys(tea.KeyDown)
		Expect(h.focused()).To(Equal("name"))
	})

	It("shows a blurred row's value and opens only the focused row's editor", func() {
		h := newHarness(notificationSchema, 40)
		h.keys(tea.KeyEnter, tea.KeyEnter) // Webhook
		view := h.m.View()
		Expect(view).To(ContainSubstring("Webhook"), "the blurred Channel row shows the label")
		Expect(view).To(ContainSubstring("› Name"))
		Expect(view).NotTo(ContainSubstring("Email"), "the closed Channel select lists no options")
	})

	It("folds what does not fit under Advanced, and expands it on enter", func() {
		h := newHarness(manyOptional(30), 24)
		view := h.m.View()
		Expect(view).To(ContainSubstring("▸ Advanced"))
		Expect(view).To(ContainSubstring("25 options"), "24 lines leave room for Name and 5 options")
		Expect(view).NotTo(ContainSubstring("Option 06"))

		for range 6 {
			h.keys(tea.KeyTab)
		}
		Expect(h.focused()).To(Equal("(advanced)"))
		h.keys(tea.KeyTab)
		Expect(h.focused()).To(Equal("(submit)"), "a collapsed Advanced row is passed over")
		h.keys(tea.KeyShiftTab, tea.KeyEnter)
		Expect(h.m.expanded).To(BeTrue())
		h.keys(tea.KeyTab)
		Expect(h.focused()).To(Equal("opt06"))
		Expect(h.m.View()).To(ContainSubstring("▾ Advanced"))
	})

	It("submits with ctrl+s only once every required field is valid", func() {
		h := newHarness(notificationSchema, 40)
		h.keys(tea.KeyEnter, tea.KeyEnter) // Webhook; Name is now focused and empty
		h.keys(tea.KeyTab)
		h.keys(tea.KeyCtrlS)
		Expect(h.m.done).To(BeFalse())
		Expect(h.focused()).To(Equal("name"), "the first invalid field gets focus")
		Expect(h.m.View()).To(ContainSubstring("is required"))
		h.typed("ops")
		h.keys(tea.KeyTab)
		h.typed("https://hooks.example.com")
		h.keys(tea.KeyCtrlS)
		Expect(h.m.done).To(BeTrue())
	})

	It("reverts a changed field on esc and goes back on shift+tab", func() {
		h := newHarness(notificationSchema, 40)
		h.keys(tea.KeyEnter, tea.KeyEnter)
		h.typed("draft")
		h.keys(tea.KeyEsc)
		Expect(h.st.get([]string{"name"})).To(BeNil())
		Expect(h.m.quitting).To(BeFalse(), "the esc that reverted does not count towards quitting")
		h.keys(tea.KeyShiftTab)
		Expect(h.focused()).To(Equal("kind"))
	})

	It("cancels on esc esc, but not when another key comes between", func() {
		h := newHarness(notificationSchema, 40)
		h.keys(tea.KeyEsc)
		Expect(h.m.View()).To(ContainSubstring("press esc again to quit"))
		h.keys(tea.KeyDown, tea.KeyEsc)
		Expect(h.m.aborted).To(BeFalse())
		h.keys(tea.KeyEsc)
		Expect(h.m.aborted).To(BeTrue())
	})

	It("describes the highlighted option while open, and the chosen one on its row", func() {
		h := newHarness(`{"type": "object", "required": ["prompt"], "properties": {
			"prompt": {"type": "string", "title": "Prompt", "enum": ["review", "summarize"],
				"x-enum-descriptions": {"review": "Reviews a diff for defects", "summarize": "Summarizes a diff"}},
			"model": {"type": "string", "title": "Model"}
		}}`, 40)
		h.keys(tea.KeyEnter)
		Expect(h.m.View()).To(ContainSubstring("Reviews a diff for defects"))
		h.keys(tea.KeyDown)
		Expect(h.m.View()).To(And(ContainSubstring("Summarizes a diff"), Not(ContainSubstring("Reviews a diff"))))
		h.keys(tea.KeyEnter)
		Expect(h.focused()).To(Equal("model"))
		Expect(h.m.View()).To(ContainSubstring("summarize  Summarizes a diff"))
	})

	It("cancels on ctrl+c", func() {
		h := newHarness(notificationSchema, 40)
		h.keys(tea.KeyCtrlC)
		Expect(h.m.aborted).To(BeTrue())
	})
})
