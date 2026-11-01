package entitydemo

import (
	"context"
	"path/filepath"

	"github.com/flanksource/captain/pkg/api"
	"github.com/flanksource/captain/pkg/captainconfig"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Captain chat settings", func() {
	It("uses the model configured for Captain's active provider", func() {
		configPath := filepath.Join(GinkgoT().TempDir(), ".captain.yaml")
		captainconfig.SetPathForTesting(configPath)
		DeferCleanup(func() { captainconfig.SetPathForTesting("") })

		configuredModel := "gpt-5"
		provider := api.OpenAI
		Expect(captainconfig.Save(captainconfig.Config{
			AI: captainconfig.AIDefaults{
				DefaultProvider: provider.Name,
				Providers: map[string]captainconfig.ProviderDefaults{
					provider.Name: {Mode: string(api.ModeAPI), Model: configuredModel},
				},
			},
		})).To(Succeed())

		profile, err := captainRuntimeProfile(context.Background())
		Expect(err).NotTo(HaveOccurred())
		resolved, err := api.ResolveSpecLayers(api.ResolveSpecOptions{
			Layers: profile.Composed.Trace, Saved: profile.Saved, RequireModel: true,
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(resolved.Spec.Model.Name).To(Equal(configuredModel))
		Expect(resolved.Spec.Model.Mode).To(Equal(api.ModeAPI))
	})
})
