// Package e2e builds the real `teo` binary and drives it as a subprocess,
// exercising the full path a user hits: argv in, files/stdin read from disk,
// bytes out, exit codes. It treats the binary as a black box but still uses the
// teo parser to assert the emitted text is well-formed.
package e2e

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var (
	teoBin  string
	tempDir string
)

func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "E2E Suite")
}

var _ = BeforeSuite(func() {
	var err error
	tempDir, err = os.MkdirTemp("", "teo-e2e")
	Expect(err).NotTo(HaveOccurred())

	teoBin = filepath.Join(tempDir, "teo")
	build := exec.Command("go", "build", "-o", teoBin, "./cmd/teo")
	build.Dir = ".." // module root, the parent of this package dir
	out, err := build.CombinedOutput()
	Expect(err).NotTo(HaveOccurred(), "building teo binary:\n%s", string(out))
})

var _ = AfterSuite(func() {
	if tempDir != "" {
		_ = os.RemoveAll(tempDir)
	}
})
