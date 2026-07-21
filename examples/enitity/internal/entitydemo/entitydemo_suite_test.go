package entitydemo

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestEntityDemo(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Entity Demo Suite")
}
