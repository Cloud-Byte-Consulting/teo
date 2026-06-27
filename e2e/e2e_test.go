package e2e

import (
	"bytes"
	"os/exec"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"truenas-scale-1.tail5a208d.ts.net/Cloud-Byte-Consulting/teo"
)

// run invokes the built binary with optional stdin and returns code/stdout/stderr.
func run(stdin string, args ...string) (int, string, string) {
	cmd := exec.Command(teoBin, args...)
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	var out, errBuf bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errBuf

	switch e := cmd.Run().(type) {
	case nil:
		return 0, out.String(), errBuf.String()
	case *exec.ExitError:
		return e.ExitCode(), out.String(), errBuf.String()
	default:
		Fail("running binary: " + e.Error())
		return -1, "", ""
	}
}

func dataPath(name string) string { return filepath.Join("..", "testdata", name) }

var _ = Describe("teo binary", func() {
	It("converts a JSON file to valid TEO", func() {
		code, out, errOut := run("", "convert", dataPath("issues.json"))
		Expect(code).To(Equal(0), errOut)
		Expect(teo.Validate(out)).To(Succeed())
		// fields are sorted (decoders drop source key order)
		Expect(out).To(ContainSubstring("issues[3]{author,number,state,title}:"))
		// the comma in the title must be quoted so the row keeps 4 cells
		Expect(out).To(ContainSubstring(`bob,43,open,"Add dark mode, finally"`))
	})

	It("agrees between JSON and YAML inputs", func() {
		_, jsonOut, _ := run("", "convert", dataPath("issues.json"))
		code, yamlOut, errOut := run("", "convert", dataPath("issues.yaml"))
		Expect(code).To(Equal(0), errOut)
		Expect(yamlOut).To(Equal(jsonOut))
	})

	It("reads from a stdin pipeline", func() {
		code, out, errOut := run(`{"name":"acme","count":2}`, "convert")
		Expect(code).To(Equal(0), errOut)
		Expect(out).To(ContainSubstring("name: acme"))
		Expect(out).To(ContainSubstring("count: 2"))
	})

	It("round-trips convert piped into validate", func() {
		_, converted, _ := run("", "convert", dataPath("issues.json"))
		code, out, errOut := run(converted, "validate")
		Expect(code).To(Equal(0), errOut)
		Expect(out).To(ContainSubstring("ok"))
	})

	It("exits non-zero on invalid input", func() {
		code, _, _ := run(`{bad json`, "convert", "--from", "json")
		Expect(code).To(Equal(1))
	})

	It("prints a non-empty version", func() {
		code, out, _ := run("", "version")
		Expect(code).To(Equal(0))
		Expect(strings.TrimSpace(out)).NotTo(BeEmpty())
	})
})
