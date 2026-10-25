package schemaform_test

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/flanksource/clicky/api/filter"
	"github.com/flanksource/clicky/api/filter/celfilter"
	"github.com/flanksource/clicky/schemaform"
)

func TestSchemaform(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Schemaform Suite")
}

var _ = BeforeEach(celfilter.Use)

// answers runs a schema in accessible mode with one scripted answer per line
// (an empty string accepts the field's current value). Select answers are the
// 1-based option number; an optional select lists "(none)" as option 1.
// Required fields and fields with a value are asked first; "Configure N
// advanced options?" then gates the empty optional ones.
type answers struct {
	opts   schemaform.Options
	output bytes.Buffer
}

func (a *answers) run(schema json.RawMessage, lines ...string) (map[string]any, error) {
	a.opts.Accessible = true
	a.opts.Input = strings.NewReader(strings.Join(lines, "\n") + "\n")
	a.opts.Output = &a.output
	return schemaform.Run(context.Background(), schema, a.opts)
}

var _ = Describe("Run (accessible mode)", func() {
	var a *answers
	BeforeEach(func() { a = &answers{} })

	Describe("allOf if/then/else branches", func() {
		It("asks the fields of the Add branch", func() {
			Expect(a.run(fixture("conditionals.json"),
				"1",    // Member class group: Add (01)
				"Gold", // New member class group (required by the branch)
				"y",    // configure the advanced (optional) fields
				"3",    // Loan override: Yes (01)
				"2500", // Loan amount
				"4.25", // Loan rate
			)).To(Equal(map[string]any{
				"MemberClassGroupOption": "01",
				"NewMemberClassGroup":    "Gold",
				"LoanOverride":           "01",
				"LoanAmount":             float64(2500),
				"LoanRate":               4.25,
			}))
		})

		It("asks the fields of the Update branch and skips the loan fields", func() {
			Expect(a.run(fixture("conditionals.json"),
				"2",      // Member class group: Update (02)
				"Silver", // Current member class group
				"n",      // leave Change reason and Loan override (optional) empty
			)).To(Equal(map[string]any{
				"MemberClassGroupOption":  "02",
				"CurrentMemberClassGroup": "Silver",
			}))
		})

		It("keeps the value of an x-hidden field, as clicky-ui does", func() {
			a.opts.Default = map[string]any{"CurrentMemberClassGroup": "kept"}
			Expect(a.run(fixture("conditionals.json"), "1", "Gold", "n")).To(Equal(map[string]any{
				"MemberClassGroupOption":  "01",
				"NewMemberClassGroup":     "Gold",
				"CurrentMemberClassGroup": "kept",
			}))
		})
	})

	Describe("x-on-change listeners", func() {
		It("shows, requires and sets fields, including a CEL when.expr", func() {
			Expect(a.run(fixture("listeners.json"),
				"1",      // Member class group: Add (Loan rate, disabled, prints its 3.5)
				"Gold",   // New member class group (shown + required)
				"y",      // configure the advanced fields
				"3",      // Loan override: Yes — sets Loan amount to 1000
				"250000", // Loan amount (> 100000 reveals Approval code)
				"A-7",    // Approval code
			)).To(Equal(map[string]any{
				"MemberClassGroupOption": "01",
				"NewMemberClassGroup":    "Gold",
				"LoanOverride":           "01",
				"LoanAmount":             float64(250000),
				"LoanRate":               3.5,
				"ApprovalCode":           "A-7",
			}))
		})

		It("fails a when.expr with ErrNoAdapter when no filter engine is configured", func() {
			filter.SetAdapter(nil)
			_, err := a.run(fixture("listeners.json"), "1", "Gold", "y", "3", "250000", "A-7")
			Expect(err).To(MatchError(filter.ErrNoAdapter))
		})

		It("runs the else branch: sets a value and shows a disabled field as read-only", func() {
			Expect(a.run(fixture("listeners.json"),
				"2",      // Member class group: Update — sets Change reason
				"Silver", // Current member class group
				"y",      // configure the advanced fields
				"",       // Change reason keeps the "Amendment" the listener set
				"2",      // Loan override: No — disables Loan rate
			)).To(Equal(map[string]any{
				"MemberClassGroupOption":  "02",
				"CurrentMemberClassGroup": "Silver",
				"ChangeReason":            "Amendment",
				"LoanOverride":            "00",
				"LoanRate":                3.5,
			}))
			Expect(a.output.String()).To(ContainSubstring("Loan rate"))
		})

		It("refreshes an enum a patch rewrites", func() {
			schema := json.RawMessage(`{
				"type": "object",
				"properties": {
					"region": {"type": "string", "enum": ["eu", "us"], "x-on-change": [
						{"when": {"const": "eu"}, "patch": {"zone": {"enum": ["eu-west", "eu-central"]}}}
					]},
					"zone": {"type": "string", "enum": ["us-east"]}
				}
			}`)
			Expect(a.run(schema, "y", "2", "3")).To(Equal(map[string]any{"region": "eu", "zone": "eu-central"}))
		})

		It("rejects a listener that targets an undeclared key before asking anything", func() {
			_, err := a.run(json.RawMessage(`{"type":"object","properties":{
				"mode": {"type": "string", "x-on-change": [{"hide": ["missing"]}]}
			}}`))
			Expect(err).To(MatchError(ContainSubstring(`unknown target "missing"`)))
		})
	})

	It("asks the x-discriminator first and only the chosen kind's fields, dropping the other kind's preset", func() {
		a.opts.Default = map[string]any{"url": "https://stale.example.com"}
		schema := json.RawMessage(`{
			"type": "object",
			"x-discriminator": "kind",
			"required": ["kind"],
			"properties": {
				"name": {"type": "string", "title": "Name"},
				"kind": {"type": "string", "enum": ["webhook", "email"], "x-enum-labels": {"webhook": "Webhook", "email": "Email"}}
			},
			"allOf": [
				{"if": {"properties": {"kind": {"const": "webhook"}}, "required": ["kind"]},
				 "then": {"properties": {"url": {"type": "string", "title": "URL"}}, "required": ["url"]}},
				{"if": {"properties": {"kind": {"const": "email"}}, "required": ["kind"]},
				 "then": {"properties": {"to": {"type": "array", "title": "Recipients", "items": {"type": "string"}}}, "required": ["to"]}}
			]
		}`)
		// Recipients, required by the chosen branch, comes before the optional Name.
		Expect(a.run(schema, "2", "ops@example.com, dev@example.com", "y", "Alerts")).To(Equal(map[string]any{
			"kind": "email",
			"name": "Alerts",
			"to":   []any{"ops@example.com", "dev@example.com"},
		}))
	})

	It("collects tags, maps, multi choices, booleans, dates and nested objects", func() {
		schema := json.RawMessage(`{
			"type": "object",
			"properties": {
				"tags": {"type": "array", "items": {"type": "string"}},
				"labels": {"type": "object", "additionalProperties": {"type": "string"}},
				"zones": {"type": "array", "uniqueItems": true, "items": {"type": "string", "enum": ["a", "b", "c"]}},
				"dryRun": {"type": "boolean"},
				"starts": {"type": "string", "format": "date"},
				"owner": {"type": "object", "title": "Owner", "properties": {
					"name": {"type": "string"},
					"email": {"type": "string"}
				}}
			}
		}`)
		Expect(a.run(schema,
			"y", // every field is optional: configure them
			"web, api",
			"team=core, tier=1",
			"1", "3", "0", // zones: toggle a and c, then finish
			"y",
			"2026-13-01", "2026-10-23", // an invalid date is asked again
			"Dana",
			"dana@example.com",
		)).To(Equal(map[string]any{
			"tags":   []any{"web", "api"},
			"labels": map[string]any{"team": "core", "tier": "1"},
			"zones":  []any{"a", "c"},
			"dryRun": true,
			"starts": "2026-10-23",
			"owner":  map[string]any{"name": "Dana", "email": "dana@example.com"},
		}))
		Expect(a.output.String()).To(ContainSubstring(`"2026-13-01" is not a valid date`))
	})

	It("re-asks a number until it satisfies the schema", func() {
		schema := json.RawMessage(`{"type":"object","required":["replicas"],"properties":{
			"replicas": {"type": "integer", "minimum": 1, "title": "Replicas"}
		}}`)
		Expect(a.run(schema, "abc", "0", "2.5", "3")).To(Equal(map[string]any{"replicas": float64(3)}))
		Expect(a.output.String()).To(And(
			ContainSubstring(`"abc" is not an integer`),
			ContainSubstring("must be ≥ 1"),
			ContainSubstring(`"2.5" is not an integer`),
		))
	})

	It("fails with ErrNoInput when the input ends before a required field is answered", func() {
		_, err := a.run(json.RawMessage(`{"type":"object","required":["name"],"properties":{"name":{"type":"string"}}}`))
		Expect(err).To(MatchError(schemaform.ErrNoInput))
	})

	It("collects an array of objects one item at a time within minItems", func() {
		schema := json.RawMessage(`{
			"type": "object",
			"properties": {
				"team": {"type": "string"},
				"members": {
					"type": "array", "title": "Members", "minItems": 1,
					"x-item": {"noun": "member", "title": ["login"]},
					"items": {"type": "object", "required": ["login"], "properties": {
						"login": {"type": "string", "title": "Login"},
						"admin": {"type": "boolean", "title": "Admin"}
					}}
				}
			}
		}`)
		Expect(a.run(schema,
			"y", "Platform", // team is optional
			"alice", "y", "y", // first member (minItems makes it mandatory), configure Admin, yes
			"y",            // add a member?
			"bob", "y", "", // admin defaults to no
			"n", // add a member?
		)).To(Equal(map[string]any{
			"team": "Platform",
			"members": []any{
				map[string]any{"login": "alice", "admin": true},
				map[string]any{"login": "bob", "admin": false},
			},
		}))
		Expect(a.output.String()).To(And(ContainSubstring("added alice"), ContainSubstring("added bob")))
	})

	It("asks the fields of an x-clicky-schema slot, loaded for the answers before them", func() {
		a.opts.LoadSchema = func(_ context.Context, req schemaform.SchemaRequest) (json.RawMessage, error) {
			Expect(req).To(Equal(schemaform.SchemaRequest{Source: "vars", Values: map[string]any{"prompt": "review"}}))
			return json.RawMessage(`{"type":"object","required":["diff"],"properties":{"diff":{"type":"string","title":"Diff"}}}`), nil
		}
		schema := json.RawMessage(`{"type":"object","required":["prompt"],"properties":{
			"prompt": {"type": "string", "enum": ["review", "summarize"]},
			"vars": {"type": "string", "x-clicky-schema": {"source": "vars", "scope": ["prompt"]}}
		}}`)
		Expect(a.run(schema, "1", "HEAD~1")).To(Equal(map[string]any{
			"prompt": "review",
			"vars":   map[string]any{"diff": "HEAD~1"},
		}))
	})

	It("fails loudly when the schema has an x-clicky-schema slot but no LoadSchema", func() {
		_, err := a.run(json.RawMessage(`{"type":"object","properties":{
			"vars": {"type": "string", "x-clicky-schema": {"source": "vars", "scope": ["prompt"]}}
		}}`))
		Expect(err).To(MatchError(ContainSubstring(`x-clicky-schema "vars" needs a LoadSchema`)))
	})

	Describe("x-clicky-lookup", func() {
		var requests []schemaform.LookupRequest
		BeforeEach(func() {
			requests = nil
			options := map[string][]string{
				"env":   {"dev", "prod"},
				"owner": {"alice", "bob", "carol"},
			}
			a.opts.Lookup = func(_ context.Context, req schemaform.LookupRequest) (schemaform.LookupResult, error) {
				requests = append(requests, req)
				if req.Spec.Filter == "service" {
					return schemaform.LookupResult{Options: []schemaform.Option{{Value: "checkout", Label: "checkout"}}, Truncated: true, Total: 500}, nil
				}
				var out []schemaform.Option
				for _, v := range options[req.Spec.Filter] {
					out = append(out, schemaform.Option{Value: v, Label: v, Raw: v})
				}
				return schemaform.LookupResult{Options: out}, nil
			}
		})

		It("offers lookup options, multi-selects and free text for a truncated set", func() {
			schema := json.RawMessage(`{"type":"object","properties":{
				"env": {"type": "string", "title": "Environment", "x-clicky-lookup": {"filter": "env"}},
				"owners": {"type": "array", "items": {"type": "string"}, "x-clicky-lookup": {"filter": "owner", "multi": true}},
				"service": {"type": "string", "x-clicky-lookup": {"filter": "service"}}
			}}`)
			Expect(a.run(schema,
				"y",           // every field is optional: configure them
				"3",           // env: (none), dev, prod
				"1", "3", "0", // owners: alice, carol
				"checkout-api", // service: truncated → free text
			)).To(Equal(map[string]any{
				"env":     "prod",
				"owners":  []any{"alice", "carol"},
				"service": "checkout-api",
			}))
			last := requests[len(requests)-1]
			Expect(last.Values).To(HaveKeyWithValue("env", "prod"))
		})

		It("fails loudly when the schema has a lookup but no Lookup is configured", func() {
			a.opts.Lookup = nil
			_, err := a.run(json.RawMessage(`{"type":"object","properties":{"env":{"type":"string","x-clicky-lookup":{"filter":"env"}}}}`))
			Expect(err).To(MatchError(ContainSubstring(`x-clicky-lookup "env" needs a Lookup`)))
		})
	})
})
