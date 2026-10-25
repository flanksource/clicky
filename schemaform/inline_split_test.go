package schemaform

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("splitAdvanced", func() {
	var (
		discriminator = splitRow{discriminator: true, hasValue: true}
		required      = splitRow{required: true}
		requiredSet   = splitRow{required: true, hasValue: true}
		valued        = splitRow{hasValue: true}
		empty         = splitRow{}
	)

	type split struct{ primary, advanced []int }

	DescribeTable("shows required rows, then what fits, valued rows first",
		func(rows []splitRow, budget int, want split) {
			primary, advanced := splitAdvanced(rows, budget)
			Expect(split{primary, advanced}).To(Equal(want))
		},
		Entry("everything fits: nothing is advanced",
			[]splitRow{empty, required, valued}, 5,
			split{primary: []int{1, 2, 0}}),
		Entry("valued rows take the room before empty ones",
			[]splitRow{empty, valued, empty, valued}, 2,
			split{primary: []int{1, 3}, advanced: []int{0, 2}}),
		Entry("required rows stay even past the budget",
			[]splitRow{required, empty, requiredSet, valued}, 1,
			split{primary: []int{2, 0}, advanced: []int{1, 3}}),
		Entry("the discriminator leads",
			[]splitRow{requiredSet, empty, discriminator}, 2,
			split{primary: []int{2, 0}, advanced: []int{1}}),
		Entry("an empty optional positional is shown, ahead of required rows and values",
			[]splitRow{valued, requiredSet, {positional: true}}, 0,
			split{primary: []int{2, 1}, advanced: []int{0}}),
		Entry("no room: only pinned rows are shown",
			[]splitRow{empty, valued}, 0,
			split{advanced: []int{0, 1}}),
	)
})
