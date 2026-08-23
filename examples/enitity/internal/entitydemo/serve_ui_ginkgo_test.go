package entitydemo

import (
	"context"
	"path/filepath"

	"github.com/flanksource/captain/pkg/api/registry"
	"github.com/flanksource/captain/pkg/captainconfig"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Captain chat settings", func() {
	It("uses the model configured for Captain's active provider", func() {
		configPath := filepath.Join(GinkgoT().TempDir(), ".captain.yaml")
		captainconfig.SetPathForTesting(configPath)
		DeferCleanup(func() { captainconfig.SetPathForTesting("") })

		configuredModel := "configured-model"
		provider := registry.OpenAIProvider
		Expect(captainconfig.Save(captainconfig.Config{
			AI: captainconfig.AIDefaults{
				DefaultProvider: string(provider),
				Providers: map[string]captainconfig.ProviderDefaults{
					string(provider): {Agent: string(provider), Model: configuredModel},
				},
			},
		})).To(Succeed())

		profile, err := captainRuntimeProfile(context.Background())
		Expect(err).NotTo(HaveOccurred())
		Expect(profile.Resolved.Spec.Model.Name).To(Equal(configuredModel))
		Expect(profile.Resolved.Spec.Model.Backend).To(Equal(provider))
	})
})
