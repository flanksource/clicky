package rpc

import (
	clicky "github.com/flanksource/clicky/entity"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"
)

var _ = Describe("scheduled operation conversion", func() {
	It("copies opt-in schedule metadata to x-clicky", func() {
		cmd := &cobra.Command{
			Use:  "vacuum",
			RunE: func(*cobra.Command, []string) error { return nil },
		}
		clicky.AnnotateSchedule(cmd, clicky.OperationScheduleMeta{
			Suggestions: []clicky.ScheduleSuggestion{{
				Label: "Recommended",
				Cron:  "0 2 * * 0",
			}},
		})

		operation, err := NewConverter(DefaultConfig()).ConvertCommand(cmd)
		Expect(err).NotTo(HaveOccurred())
		Expect(operation.Clicky).NotTo(BeNil())
		Expect(operation.Clicky.Schedule).To(Equal(&clicky.OperationScheduleMeta{
			Suggestions: []clicky.ScheduleSuggestion{{
				Label: "Recommended",
				Cron:  "0 2 * * 0",
			}},
		}))
	})
})
