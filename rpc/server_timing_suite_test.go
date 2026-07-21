package rpc

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestServerTiming(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "RPC Server Timing Suite")
}
