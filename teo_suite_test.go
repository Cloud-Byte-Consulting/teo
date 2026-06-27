package teo_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestTeo(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "TEO Suite")
}
