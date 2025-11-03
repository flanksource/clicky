package ai

import (
	"sync"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestSession(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Session Suite")
}

var _ = Describe("Session", func() {
	var session *Session

	BeforeEach(func() {
		session = NewSession("test-session", "test-project")
	})

	Describe("NewSession", func() {
		It("should create a new session with ID and ProjectName", func() {
			Expect(session.ID).To(Equal("test-session"))
			Expect(session.ProjectName).To(Equal("test-project"))
			Expect(session.Costs).To(HaveLen(0))
		})
	})

	Describe("AddCost", func() {
		It("should add a cost to the session", func() {
			cost := Cost{
				Model:        "claude-sonnet-4",
				InputTokens:  100,
				OutputTokens: 50,
				TotalTokens:  150,
				InputCost:    0.003,
				OutputCost:   0.0075,
			}

			session.AddCost(cost)

			Expect(session.Costs).To(HaveLen(1))
			Expect(session.Costs[0]).To(Equal(cost))
		})

		It("should handle multiple cost additions", func() {
			cost1 := Cost{
				Model:        "claude-sonnet-4",
				InputTokens:  100,
				OutputTokens: 50,
				TotalTokens:  150,
				InputCost:    0.003,
				OutputCost:   0.0075,
			}

			cost2 := Cost{
				Model:        "claude-sonnet-4",
				InputTokens:  200,
				OutputTokens: 100,
				TotalTokens:  300,
				InputCost:    0.006,
				OutputCost:   0.015,
			}

			session.AddCost(cost1)
			session.AddCost(cost2)

			Expect(session.Costs).To(HaveLen(2))
			Expect(session.Costs[0]).To(Equal(cost1))
			Expect(session.Costs[1]).To(Equal(cost2))
		})

		It("should be thread-safe", func() {
			var wg sync.WaitGroup
			numGoroutines := 100

			for i := 0; i < numGoroutines; i++ {
				wg.Add(1)
				go func(idx int) {
					defer wg.Done()
					cost := Cost{
						Model:        "claude-sonnet-4",
						InputTokens:  10,
						OutputTokens: 5,
						TotalTokens:  15,
						InputCost:    0.0003,
						OutputCost:   0.00075,
					}
					session.AddCost(cost)
				}(i)
			}

			wg.Wait()

			Expect(session.Costs).To(HaveLen(numGoroutines))
		})
	})

	Describe("GetTotalCost", func() {
		It("should return zero cost for empty session", func() {
			total := session.GetTotalCost()

			Expect(total.InputTokens).To(Equal(0))
			Expect(total.OutputTokens).To(Equal(0))
			Expect(total.TotalTokens).To(Equal(0))
			Expect(total.InputCost).To(Equal(0.0))
			Expect(total.OutputCost).To(Equal(0.0))
		})

		It("should aggregate costs from multiple entries", func() {
			cost1 := Cost{
				Model:        "claude-sonnet-4",
				InputTokens:  100,
				OutputTokens: 50,
				TotalTokens:  150,
				InputCost:    0.003,
				OutputCost:   0.0075,
			}

			cost2 := Cost{
				Model:        "claude-sonnet-4",
				InputTokens:  200,
				OutputTokens: 100,
				TotalTokens:  300,
				InputCost:    0.006,
				OutputCost:   0.015,
			}

			session.AddCost(cost1)
			session.AddCost(cost2)

			total := session.GetTotalCost()

			Expect(total.InputTokens).To(Equal(300))
			Expect(total.OutputTokens).To(Equal(150))
			Expect(total.TotalTokens).To(Equal(450))
			Expect(total.InputCost).To(BeNumerically("~", 0.009, 0.00001))
			Expect(total.OutputCost).To(BeNumerically("~", 0.0225, 0.00001))
		})
	})

	Describe("GetCostsByModel", func() {
		It("should return empty map for empty session", func() {
			costsByModel := session.GetCostsByModel()

			Expect(costsByModel).To(BeEmpty())
		})

		It("should group costs by model", func() {
			cost1 := Cost{
				Model:        "claude-sonnet-4",
				InputTokens:  100,
				OutputTokens: 50,
				TotalTokens:  150,
				InputCost:    0.003,
				OutputCost:   0.0075,
			}

			cost2 := Cost{
				Model:        "claude-opus-4",
				InputTokens:  200,
				OutputTokens: 100,
				TotalTokens:  300,
				InputCost:    0.015,
				OutputCost:   0.075,
			}

			cost3 := Cost{
				Model:        "claude-sonnet-4",
				InputTokens:  150,
				OutputTokens: 75,
				TotalTokens:  225,
				InputCost:    0.0045,
				OutputCost:   0.01125,
			}

			session.AddCost(cost1)
			session.AddCost(cost2)
			session.AddCost(cost3)

			costsByModel := session.GetCostsByModel()

			Expect(costsByModel).To(HaveLen(2))
			Expect(costsByModel).To(HaveKey("claude-sonnet-4"))
			Expect(costsByModel).To(HaveKey("claude-opus-4"))

			// Verify sonnet costs are aggregated
			sonnetCost := costsByModel["claude-sonnet-4"]
			Expect(sonnetCost.InputTokens).To(Equal(250))
			Expect(sonnetCost.OutputTokens).To(Equal(125))
			Expect(sonnetCost.TotalTokens).To(Equal(375))
			Expect(sonnetCost.InputCost).To(BeNumerically("~", 0.0075, 0.00001))
			Expect(sonnetCost.OutputCost).To(BeNumerically("~", 0.01875, 0.00001))

			// Verify opus costs
			opusCost := costsByModel["claude-opus-4"]
			Expect(opusCost.InputTokens).To(Equal(200))
			Expect(opusCost.OutputTokens).To(Equal(100))
			Expect(opusCost.TotalTokens).To(Equal(300))
			Expect(opusCost.InputCost).To(BeNumerically("~", 0.015, 0.00001))
			Expect(opusCost.OutputCost).To(BeNumerically("~", 0.075, 0.00001))
		})

		It("should handle costs with empty model as 'unknown'", func() {
			cost := Cost{
				Model:        "",
				InputTokens:  100,
				OutputTokens: 50,
				TotalTokens:  150,
				InputCost:    0.003,
				OutputCost:   0.0075,
			}

			session.AddCost(cost)

			costsByModel := session.GetCostsByModel()

			Expect(costsByModel).To(HaveLen(1))
			Expect(costsByModel).To(HaveKey("unknown"))
		})
	})

})
