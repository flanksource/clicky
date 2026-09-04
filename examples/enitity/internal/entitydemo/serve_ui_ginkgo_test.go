package entitydemo

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"

	capchat "github.com/flanksource/captain/pkg/aichat"
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

		configuredModel := "configured-model"
		provider := api.OpenAI
		Expect(captainconfig.Save(captainconfig.Config{
			AI: captainconfig.AIDefaults{
				DefaultProvider: provider.Name,
				Providers: map[string]captainconfig.ProviderDefaults{
					provider.Name: {Mode: string(api.ModeAPI), Model: configuredModel},
				},
			},
		})).To(Succeed())

		profile, err := captainRuntimeProfile(context.Background(), capchat.RuntimeProfileSelection{})
		Expect(err).NotTo(HaveOccurred())
		Expect(profile.Resolved.Spec.Model.Name).To(Equal(configuredModel))
		Expect(profile.Resolved.Spec.Model.Mode).To(Equal(api.ModeAPI))
	})

	It("rejects a named runtime profile because the demo only serves its default", func() {
		service := capchat.NewService(capchat.ServiceOptions{Profile: capchat.RuntimeProfileProviderFunc(captainRuntimeProfile)})
		request := httptest.NewRequest(http.MethodGet, "/api/chat/models?runtimeProfile=review", nil)
		response := httptest.NewRecorder()

		service.Handler().ServeHTTP(response, request)

		Expect(response.Code).To(Equal(http.StatusBadRequest))
		Expect(response.Body.String()).To(ContainSubstring(`runtime profile "review" is not available`))
	})
})
