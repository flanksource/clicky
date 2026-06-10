package cache_test

import (
	"testing"

	"github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestCache(t *testing.T) {
	RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "Cache Browser Suite")
}
