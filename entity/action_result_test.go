package entity

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type actionPayload struct {
	ConnectionID string `json:"connectionId"`
	Provider     string `json:"provider"`
}

// A failing typed handler must contribute no data. Boxing its zero value into
// the `any` return makes it non-nil, and the RPC executor then serializes that
// zero struct as the response body instead of the error.
var _ = Describe("typed handler results", func() {
	failure := fmt.Errorf("refresh xero token: tenant name %q matches 2 entities", "Acme")

	It("drops the zero value when a context action fails", func() {
		info := ActionWithContext("refresh", func(context.Context, string, map[string]string) (actionPayload, error) {
			return actionPayload{}, failure
		}).actionInfo()

		data, err := info.ContextDataFunc(context.Background(), map[string]string{"id": "conn-1"}, nil)

		Expect(err).To(MatchError(failure))
		Expect(data).To(BeNil())
	})

	It("returns the value when a context action succeeds", func() {
		want := actionPayload{ConnectionID: "conn-1", Provider: "xero"}
		info := ActionWithContext("refresh", func(context.Context, string, map[string]string) (actionPayload, error) {
			return want, nil
		}).actionInfo()

		data, err := info.ContextDataFunc(context.Background(), map[string]string{"id": "conn-1"}, nil)

		Expect(err).ToNot(HaveOccurred())
		Expect(data).To(Equal(want))
	})

	It("drops a typed nil pointer when a context action fails", func() {
		info := ActionWithContext("refresh", func(context.Context, string, map[string]string) (*actionPayload, error) {
			return nil, failure
		}).actionInfo()

		data, err := info.ContextDataFunc(context.Background(), map[string]string{"id": "conn-1"}, nil)

		Expect(err).To(MatchError(failure))
		Expect(data).To(BeNil())
	})

	// A handler that fails after producing a partial result (a run that failed
	// with a trace of where it failed) returns both; the result is the
	// diagnostic, so it must reach the caller alongside the error.
	It("keeps a non-zero partial result when a context action fails", func() {
		partial := &actionPayload{ConnectionID: "conn-1", Provider: "xero"}
		info := ActionWithContext("refresh", func(context.Context, string, map[string]string) (any, error) {
			return partial, failure
		}).actionInfo()

		data, err := info.ContextDataFunc(context.Background(), map[string]string{"id": "conn-1"}, nil)

		Expect(err).To(MatchError(failure))
		Expect(data).To(Equal(partial))
	})

	It("drops the zero value when a flag action fails", func() {
		info := Action("refresh", func(string, map[string]string) (actionPayload, error) {
			return actionPayload{}, failure
		}).actionInfo()

		data, err := info.DataFunc(map[string]string{"id": "conn-1"}, nil)

		Expect(err).To(MatchError(failure))
		Expect(data).To(BeNil())
	})
})
