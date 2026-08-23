package exec

import (
	"os"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Process environment", func() {
	It("replaces the host environment with an exact environment", func() {
		const hostOnly = "CLICKY_EXEC_HOST_ONLY"
		DeferCleanup(os.Unsetenv, hostOnly)
		Expect(os.Setenv(hostOnly, "must-not-leak")).To(Succeed())

		result := NewExec("/usr/bin/env").WithExactEnv(map[string]string{
			"CLICKY_EXEC_EXACT": "present",
		}).Run().Result()

		Expect(result.Error).ToNot(HaveOccurred())
		Expect(environmentMap(result.Stdout)).To(Equal(map[string]string{
			"CLICKY_EXEC_EXACT": "present",
		}))
	})

	It("merges WithEnv values into the exact environment", func() {
		result := NewExec("/usr/bin/env").
			WithExactEnv(map[string]string{"EXISTING": "original"}).
			WithEnv(map[string]string{"EXISTING": "overridden", "ADDED": "value"}).
			Run().Result()

		Expect(result.Error).ToNot(HaveOccurred())
		Expect(environmentMap(result.Stdout)).To(Equal(map[string]string{
			"ADDED":    "value",
			"EXISTING": "overridden",
		}))
	})
})

func environmentMap(output string) map[string]string {
	env := map[string]string{}
	for line := range strings.SplitSeq(strings.TrimSpace(output), "\n") {
		key, value, ok := strings.Cut(line, "=")
		if ok {
			env[key] = value
		}
	}
	return env
}
