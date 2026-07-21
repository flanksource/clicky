package aichat_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestAichatAdapter(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Clicky Captain Tool Adapter Suite")
}
