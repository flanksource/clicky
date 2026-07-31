package rpchttp

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestServerTiming(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "HTTP Server Timing Suite")
}
