package task

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestInteractiveRenderUsesCustomFinalContent(t *testing.T) {
	tm := newTestManager(1)
	t.Cleanup(func() { close(tm.shutdown) })
	tm.isInteractive.Store(true)

	buf := &syncBuffer{}
	tm.renderer = lipgloss.NewRenderer(buf)
	tm.setLiveRenderer(&fakeLiveRenderer{live: "LIVE-ONLY", final: "FINAL-SUMMARY"})
	addCompletedTask(tm, "fixture")

	rows := tm.interactiveRender(0, false)
	tm.interactiveRender(rows, true)

	output := buf.String()
	if strings.Count(output, "LIVE-ONLY") != 1 {
		t.Fatalf("final render must not repeat live content, got %q", output)
	}
	if !strings.Contains(output, "FINAL-SUMMARY") {
		t.Fatalf("final render must use RenderFinal content, got %q", output)
	}
	if !strings.Contains(output, "\x1b[2K\rFINAL-SUMMARY") {
		t.Fatalf("final render must restart at the first terminal column, got %q", output)
	}
}
