package task

import (
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("schedule payload", func() {
	It("preserves operation arguments through JSON persistence", func() {
		original := Schedule{
			Name:    "weekly-maintenance",
			Kind:    "maintenance",
			Cron:    "0 2 * * 0",
			Enabled: true,
			Payload: json.RawMessage(`{"operationId":"vacuum","args":{"batchSize":500}}`),
		}

		encoded, err := json.Marshal(original)
		Expect(err).NotTo(HaveOccurred())
		var restored Schedule
		Expect(json.Unmarshal(encoded, &restored)).To(Succeed())
		Expect(restored.Payload).To(MatchJSON(original.Payload))
	})
})
