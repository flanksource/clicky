package entity

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"
)

var _ = Describe("operation scheduling metadata", func() {
	suggestion := ScheduleSuggestion{
		Label:       "Recommended",
		Cron:        "0 2 * * 0",
		Description: "Every Sunday at 02:00",
	}

	It("round-trips schedule suggestions through Cobra annotations", func() {
		cmd := &cobra.Command{Use: "vacuum"}
		AnnotateSchedule(cmd, OperationScheduleMeta{
			Suggestions: []ScheduleSuggestion{suggestion},
			Timeout:     12 * time.Hour,
		})

		meta := GetCommandOpenAPIMeta(cmd)
		Expect(meta).NotTo(BeNil())
		Expect(meta.Schedule).To(Equal(&OperationScheduleMeta{
			Suggestions: []ScheduleSuggestion{suggestion},
			Timeout:     12 * time.Hour,
		}))
	})

	It("carries opt-in metadata from typed actions", func() {
		action := Action("vacuum", func(string, map[string]string) (string, error) {
			return "done", nil
		}).WithSchedule(OperationScheduleMeta{Suggestions: []ScheduleSuggestion{suggestion}})

		Expect(action.actionInfo().Schedule).To(Equal(&OperationScheduleMeta{
			Suggestions: []ScheduleSuggestion{suggestion},
		}))
	})
})
