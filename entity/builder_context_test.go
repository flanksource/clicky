package entity

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type contextualGetFlags struct {
	Database string `flag:"database"`
}

func (contextualGetFlags) ClickyActionFlags() {}

var _ = Describe("EntityBuilder context-aware get", func() {
	It("registers flags and the request-context callback together", func() {
		callback := func(context.Context, string, map[string]string) (samplePlainEntity, error) {
			return samplePlainEntity{}, nil
		}
		builder := NewEntity[samplePlainEntity, struct{}, samplePlainEntity]("contextual").
			GetWithFlagsAndContext(contextualGetFlags{}, callback)

		Expect(builder.entity.GetFlags).To(Equal(contextualGetFlags{}))
		Expect(builder.entity.GetWithFlagsAndContext).NotTo(BeNil())
		Expect(builder.entity.GetWithFlags).To(BeNil())
		Expect(builder.entity.GetWithContext).To(BeNil())
	})
})
