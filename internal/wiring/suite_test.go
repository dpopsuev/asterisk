// Package wiring tests the full mock flow. To run Ginkgo specs use the Ginkgo binary (from repo root):
//
//	go run github.com/onsi/ginkgo/v2/ginkgo ./internal/wiring/...
package wiring

import (
	"testing"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

func TestWiring(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "Wiring Suite")
}
