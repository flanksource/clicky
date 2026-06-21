package entity

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestEntity(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Entity Filter Suite")
}

// resetFilterRegistry clears the named-filter registry between specs so
// RegisterFilter's duplicate-name guard doesn't fire across tests.
func resetFilterRegistry() {
	filterRegistryMu.Lock()
	defer filterRegistryMu.Unlock()
	filterRegistry = map[string]NamedFilter{}
}
