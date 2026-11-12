package entity

import (
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("LookupNode wire shape", func() {
	It("carries icon, description, tone, badges and group, and omits them when unset", func() {
		node := LookupNode{
			Kind:        "text",
			Plain:       "orders-db",
			Icon:        "devicon:postgresql",
			Description: "db.internal:5432",
			Tone:        "success",
			Badges:      []LookupBadge{{Label: "proxy", Icon: "ion:shield", Tone: "info"}, {Label: "svc"}},
			Group:       "Postgres",
		}
		raw, err := json.Marshal(LookupResponse{Filters: map[string]LookupFilter{
			"connection": {Options: map[string]LookupNode{"orders": node, "plain": {Kind: "text", Plain: "plain"}}},
		}})
		Expect(err).NotTo(HaveOccurred())
		Expect(raw).To(MatchJSON(`{"filters":{"connection":{"options":{
			"orders": {"kind":"text","plain":"orders-db","icon":"devicon:postgresql","description":"db.internal:5432","tone":"success",
				"badges":[{"label":"proxy","icon":"ion:shield","tone":"info"},{"label":"svc"}],"group":"Postgres"},
			"plain": {"kind":"text","plain":"plain"}
		}}}}`))

		var decoded LookupResponse
		Expect(json.Unmarshal(raw, &decoded)).To(Succeed())
		Expect(decoded.Filters["connection"].Options["orders"]).To(Equal(node))
	})
})
