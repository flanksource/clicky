package clicky

import (
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/flanksource/clicky/api"
	"github.com/flanksource/clicky/internal/gumchoose"
	"github.com/flanksource/clicky/task"
	"golang.org/x/sys/unix"
	"golang.org/x/term"
)

const (
	promptMinSelectPageSize   = 5
	promptClearScreenSequence = "\x1b[2J\x1b[H"
	promptCursorQuerySequence = "\x1b[6n"
	promptCursorQueryTimeout  = 150 * time.Millisecond
)

// PromptSelectOptions configures an ANSI list prompt.
type PromptSelectOptions[T any] struct {
	Title        string
	InitialIndex int
	PageSize     int
	Render       func(T) api.Textable
}

// PromptMultiSelectOptions configures an ANSI multi-select prompt.
type PromptMultiSelectOptions[T any] struct {
	Title    string
	PageSize int
	Limit    int
	Ordered  bool
	Render   func(T) api.Textable
}

// PromptTextOptions configures an ANSI text prompt.
type PromptTextOptions struct {
	Title       string
	Default     string
	Placeholder string
	Secret      bool
	Validate    func(string) error
}

type promptPretty interface {
	Pretty() api.Text
}

type promptTerminal struct {
	input  *os.File
	output *os.File
	close  func() error
}

func openPromptTerminal() (*promptTerminal, error) {
	if term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stderr.Fd())) {
		return &promptTerminal{
			input:  os.Stdin,
			output: os.Stderr,
		}, nil
	}

	if tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0); err == nil {
		return &promptTerminal{
			input:  tty,
			output: tty,
			close:  tty.Close,
		}, nil
	}

	return nil, fmt.Errorf("interactive terminal required")
}

func (p *promptTerminal) Close() error {
	if p == nil || p.close == nil {
		return nil
	}
	return p.close()
}

func (p *promptTerminal) lineCount() int {
	if p != nil {
		if lines := promptTerminalLines(p.output); lines > 0 {
			return lines
		}
		if p.input != p.output {
			if lines := promptTerminalLines(p.input); lines > 0 {
				return lines
			}
		}
	}

	return api.GetTerminalLines()
}

func (p *promptTerminal) columnCount() int {
	if p != nil {
		if columns := promptTerminalColumns(p.output); columns > 0 {
			return columns
		}
		if p.input != p.output {
			if columns := promptTerminalColumns(p.input); columns > 0 {
				return columns
			}
		}
	}

	return api.GetTerminalWidth()
}

func (p *promptTerminal) clearScreen() {
	if p == nil || p.output == nil {
		return
	}
	_, _ = io.WriteString(p.output, promptClearScreenSequence)
}

func (p *promptTerminal) cursorRow() (int, bool) {
	if p == nil || p.input == nil || p.output == nil {
		return 0, false
	}

	fd := int(p.input.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return 0, false
	}
	defer func() {
		_ = term.Restore(fd, oldState)
	}()

	if _, err := io.WriteString(p.output, promptCursorQuerySequence); err != nil {
		return 0, false
	}

	response, ok := readPromptCursorResponse(fd, promptCursorQueryTimeout)
	if !ok {
		return 0, false
	}

	row, _, ok := parsePromptCursorResponse(response)
	return row, ok
}

type promptSession struct {
	terminal        *promptTerminal
	releaseTerminal func()
	tookOverRender  bool
}

func newPromptSession() (*promptSession, error) {
	terminal, err := openPromptTerminal()
	if err != nil {
		return nil, err
	}

	releaseTerminal, tookOverRender := task.AcquirePromptTerminal()

	return &promptSession{
		terminal:        terminal,
		releaseTerminal: releaseTerminal,
		tookOverRender:  tookOverRender,
	}, nil
}

func (s *promptSession) Close() {
	if s == nil {
		return
	}

	if s.releaseTerminal != nil {
		s.releaseTerminal()
		s.releaseTerminal = nil
	}
	if s.terminal != nil {
		_ = s.terminal.Close()
		s.terminal = nil
	}
}

func (s *promptSession) prepareSelectPrompt(promptHeight int) {
	if s == nil || s.terminal == nil || promptHeight <= 0 {
		return
	}

	if s.tookOverRender {
		s.terminal.clearScreen()
		return
	}

	if row, ok := s.terminal.cursorRow(); ok && shouldClearPromptScreen(promptHeight, s.terminal.lineCount(), row) {
		s.terminal.clearScreen()
	}
}

// PromptChoose renders a minimal arrow-key choice prompt.
func PromptChoose[T any](items []T) (T, bool) {
	return Prompt(items, PromptSelectOptions[T]{})
}

// PromptSelect renders a configurable arrow-key choice prompt.
func PromptSelect[T any](items []T, opts PromptSelectOptions[T]) (T, bool) {
	return Prompt(items, opts)
}

// PromptMultiSelect renders a configurable ANSI multi-select prompt.
func PromptMultiSelect[T any](items []T, opts PromptMultiSelectOptions[T]) ([]T, bool) {
	if len(items) == 0 {
		return nil, false
	}

	session, err := newPromptSession()
	if err != nil {
		return nil, false
	}
	defer session.Close()

	title := promptTitle(opts.Title, "Choose one or more options")
	labels, pageSize, promptHeight := preparePromptChoices(session, title, opts.PageSize, items, opts.Render)
	session.prepareSelectPrompt(promptHeight)

	limit := opts.Limit
	if limit <= 0 || limit > len(items) {
		limit = len(items)
	}

	result, err := gumchoose.Run(session.terminal.input, session.terminal.output, labels, gumchoose.Options{
		Header:       title,
		Height:       pageSize,
		Cursor:       "> ",
		ShowHelp:     false,
		Limit:        limit,
		Ordered:      opts.Ordered,
		InitialIndex: 0,
	})
	if err != nil {
		if errors.Is(err, tea.ErrInterrupted) {
			return nil, false
		}
		return nil, false
	}
	if !result.Submitted {
		return nil, false
	}

	selected := make([]T, 0, len(result.Indexes))
	for _, index := range result.Indexes {
		if index >= 0 && index < len(items) {
			selected = append(selected, items[index])
		}
	}

	return selected, true
}

// Prompt is the shared ANSI list prompt runner used by PromptChoose and PromptSelect.
func Prompt[T any](items []T, opts PromptSelectOptions[T]) (T, bool) {
	var zero T
	if len(items) == 0 {
		return zero, false
	}

	session, err := newPromptSession()
	if err != nil {
		return zero, false
	}
	defer session.Close()

	title := promptTitle(opts.Title, "Choose an option")
	labels, pageSize, promptHeight := preparePromptChoices(session, title, opts.PageSize, items, opts.Render)
	selectedIndex := clampPromptIndex(opts.InitialIndex, len(items))
	session.prepareSelectPrompt(promptHeight)

	result, err := gumchoose.Run(session.terminal.input, session.terminal.output, labels, gumchoose.Options{
		Header:       title,
		Height:       pageSize,
		Cursor:       "> ",
		ShowHelp:     false,
		Limit:        1,
		InitialIndex: selectedIndex,
	})
	if err != nil {
		if errors.Is(err, tea.ErrInterrupted) {
			return zero, false
		}
		return zero, false
	}
	if !result.Submitted || len(result.Indexes) == 0 {
		return zero, false
	}

	return items[result.Indexes[0]], true
}

// PromptText renders an ANSI text entry prompt.
func PromptText(opts PromptTextOptions) (string, bool) {
	session, err := newPromptSession()
	if err != nil {
		return "", false
	}
	defer session.Close()

	value := opts.Default

	field := huh.NewInput().
		Title(promptTitle(opts.Title, "Enter a value")).
		Prompt("> ").
		Placeholder(opts.Placeholder).
		Value(&value)

	if opts.Secret {
		field.EchoMode(huh.EchoModePassword)
	}
	if opts.Validate != nil {
		field.Validate(opts.Validate)
	}

	form := newPromptForm(session, field)
	if err := form.Run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return "", false
		}
		return "", false
	}

	return value, true
}

func newPromptForm(session *promptSession, field huh.Field) *huh.Form {
	return huh.NewForm(
		huh.NewGroup(field),
	).
		WithInput(session.terminal.input).
		WithOutput(session.terminal.output).
		WithShowHelp(false)
}

func promptTitle(title, fallback string) string {
	if strings.TrimSpace(title) == "" {
		return fallback
	}
	return title
}

func preparePromptChoices[T any](session *promptSession, title string, requestedPageSize int, items []T, render func(T) api.Textable) ([]string, int, int) {
	titleHeight := promptRenderedLineCount(title, session.terminal.columnCount())
	pageSize := clampPromptPageSize(requestedPageSize, len(items), session.terminal.lineCount(), titleHeight)
	promptHeight := promptSelectHeight(pageSize, len(items), titleHeight)

	labels := make([]string, 0, len(items))
	for _, item := range items {
		labels = append(labels, defaultPromptLabel(item, render))
	}

	return labels, pageSize, promptHeight
}

func defaultPromptLabel[T any](item T, render func(T) api.Textable) string {
	if render != nil {
		if rendered := render(item); rendered != nil {
			if label := sanitizePromptLabel(rendered.String()); label != "" {
				return label
			}
		}
	}

	if label, ok := promptPrettyLabel(any(item)); ok && label != "" {
		return label
	}

	return sanitizePromptLabel(fmt.Sprintf("%v", item))
}

func promptPrettyLabel(item any) (string, bool) {
	if isNilPromptValue(item) {
		return "", false
	}

	pretty, ok := item.(promptPretty)
	if !ok {
		return "", false
	}

	return sanitizePromptLabel(pretty.Pretty().String()), true
}

func isNilPromptValue(value any) bool {
	if value == nil {
		return true
	}

	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return v.IsNil()
	default:
		return false
	}
}

func sanitizePromptLabel(label string) string {
	label = strings.Join(strings.Fields(label), " ")
	return label
}

func clampPromptIndex(index, itemCount int) int {
	if itemCount == 0 {
		return 0
	}
	if index < 0 {
		return 0
	}
	if index >= itemCount {
		return itemCount - 1
	}
	return index
}

func promptTerminalLines(file *os.File) int {
	if file == nil {
		return 0
	}

	_, lines, err := term.GetSize(int(file.Fd()))
	if err != nil || lines <= 0 {
		return 0
	}

	return lines
}

func promptTerminalColumns(file *os.File) int {
	if file == nil {
		return 0
	}

	columns, _, err := term.GetSize(int(file.Fd()))
	if err != nil || columns <= 0 {
		return 0
	}

	return columns
}

func promptRenderedLineCount(text string, width int) int {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0
	}

	lines := 0
	for _, line := range strings.Split(text, "\n") {
		if line == "" {
			lines++
			continue
		}
		if width <= 0 {
			lines++
			continue
		}
		lineLength := len([]rune(line))
		lines += (lineLength-1)/width + 1
	}

	return lines
}

func promptSelectHeight(pageSize, itemCount, titleHeight int) int {
	height := pageSize + titleHeight
	if itemCount > pageSize {
		height++
	}
	return height
}

func promptAvailableLines(terminalLines, cursorRow int) int {
	if terminalLines <= 0 {
		return 0
	}
	if cursorRow <= 0 {
		return terminalLines
	}

	availableLines := terminalLines - cursorRow
	if availableLines < 0 {
		return 0
	}

	return availableLines
}

func shouldClearPromptScreen(pageSize, terminalLines, cursorRow int) bool {
	if pageSize <= 0 || terminalLines <= 0 {
		return false
	}

	return pageSize > promptAvailableLines(terminalLines, cursorRow)
}

func readPromptCursorResponse(fd int, timeout time.Duration) ([]byte, bool) {
	deadline := time.Now().Add(timeout)
	buf := make([]byte, 0, 32)
	tmp := []byte{0}

	for time.Now().Before(deadline) {
		remaining := time.Until(deadline)
		timeoutMillis := 1
		if remaining > 0 {
			timeoutMillis = int(remaining / time.Millisecond)
			if timeoutMillis < 1 {
				timeoutMillis = 1
			}
		}

		pollfds := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}}
		ready, err := unix.Poll(pollfds, timeoutMillis)
		if err != nil {
			if err == unix.EINTR {
				continue
			}
			return nil, false
		}
		if ready == 0 || pollfds[0].Revents&unix.POLLIN == 0 {
			continue
		}

		n, err := unix.Read(fd, tmp)
		if err != nil {
			if err == unix.EINTR {
				continue
			}
			return nil, false
		}
		if n == 0 {
			continue
		}

		buf = append(buf, tmp[0])
		if tmp[0] == 'R' {
			return buf, true
		}
	}

	return nil, false
}

func parsePromptCursorResponse(response []byte) (row, col int, ok bool) {
	start := strings.LastIndex(string(response), "\x1b[")
	end := strings.LastIndexByte(string(response), 'R')
	if start == -1 || end == -1 || end <= start {
		return 0, 0, false
	}

	if _, err := fmt.Sscanf(string(response[start:end+1]), "\x1b[%d;%dR", &row, &col); err != nil {
		return 0, 0, false
	}
	if row < 1 || col < 1 {
		return 0, 0, false
	}

	return row, col, true
}

func clampPromptPageSize(pageSize, itemCount, terminalLines, chromeHeight int) int {
	if itemCount == 0 {
		return 0
	}

	maxPageSize := itemCount
	if terminalLines > 0 && itemCount+chromeHeight > terminalLines {
		maxPageSize = terminalLines - chromeHeight - 1
		if maxPageSize < 1 {
			maxPageSize = 1
		}
	}

	minPageSize := promptMinSelectPageSize
	if minPageSize > itemCount {
		minPageSize = itemCount
	}
	if maxPageSize > 0 && maxPageSize < minPageSize {
		minPageSize = maxPageSize
	}

	if pageSize <= 0 {
		pageSize = maxPageSize
	} else if pageSize > maxPageSize {
		pageSize = maxPageSize
	}
	if pageSize > itemCount {
		pageSize = itemCount
	}
	if pageSize < minPageSize {
		pageSize = minPageSize
	}
	return pageSize
}
