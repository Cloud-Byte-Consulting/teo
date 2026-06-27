package cli_test

import (
	"bytes"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/cloud-byte-consulting/teo"
	"github.com/cloud-byte-consulting/teo/internal/cli"
)

// runCLI exercises the in-process entry point and returns code, stdout, stderr.
func runCLI(stdin string, args ...string) (int, string, string) {
	var out, errBuf bytes.Buffer
	code := cli.Run(args, strings.NewReader(stdin), &out, &errBuf)
	return code, out.String(), errBuf.String()
}

func dataPath(name string) string { return filepath.Join("..", "..", "testdata", name) }

var _ = Describe("CLI", func() {
	Describe("convert", func() {
		It("converts a JSON file to valid TEO", func() {
			code, out, errOut := runCLI("", "convert", dataPath("issues.json"))
			Expect(code).To(Equal(0), errOut)
			Expect(teo.Validate(out)).To(Succeed())
			// fields are sorted (decoders drop source key order)
			Expect(out).To(ContainSubstring("issues[3]{author,number,state,title}:"))
		})

		It("produces identical output for JSON and YAML", func() {
			_, jsonOut, _ := runCLI("", "convert", dataPath("issues.json"))
			code, yamlOut, errOut := runCLI("", "convert", dataPath("issues.yaml"))
			Expect(code).To(Equal(0), errOut)
			Expect(yamlOut).To(Equal(jsonOut))
		})

		It("auto-detects JSON from stdin", func() {
			code, out, _ := runCLI(`{"name":"acme","tags":["a","b"]}`, "convert")
			Expect(code).To(Equal(0))
			Expect(out).To(ContainSubstring("name: acme"))
			Expect(out).To(ContainSubstring("tags[2]{value}:"))
		})

		It("auto-detects YAML from stdin", func() {
			code, out, _ := runCLI("name: acme\ncount: 2\n", "convert")
			Expect(code).To(Equal(0))
			Expect(out).To(ContainSubstring("name: acme"))
			Expect(out).To(ContainSubstring("count: 2"))
		})

		It("honors an explicit --from json", func() {
			code, out, _ := runCLI(`{"a":1}`, "convert", "--from", "json", "-")
			Expect(code).To(Equal(0))
			Expect(out).To(ContainSubstring("a: 1"))
		})

		It("honors --name for a root array", func() {
			code, out, _ := runCLI(`[{"x":1}]`, "convert", "--name", "rows")
			Expect(code).To(Equal(0))
			Expect(out).To(ContainSubstring("rows[1]{x}:"))
		})

		It("exits non-zero on invalid JSON", func() {
			code, _, errOut := runCLI(`{bad`, "convert", "--from", "json")
			Expect(code).NotTo(Equal(0))
			Expect(errOut).To(ContainSubstring("parse json"))
		})

		It("exits 1 on a missing file", func() {
			code, _, _ := runCLI("", "convert", "does-not-exist.json")
			Expect(code).To(Equal(1))
		})
	})

	Describe("validate", func() {
		It("accepts converter output", func() {
			_, converted, _ := runCLI(`{"a":1}`, "convert")
			code, out, errOut := runCLI(converted, "validate")
			Expect(code).To(Equal(0), errOut)
			Expect(out).To(ContainSubstring("ok"))
		})

		It("rejects non-TEO input", func() {
			code, _, errOut := runCLI("this is not teo\n", "validate")
			Expect(code).To(Equal(1))
			Expect(errOut).To(ContainSubstring("invalid TEO"))
		})
	})

	Describe("dispatch", func() {
		It("exits 2 on an unknown command", func() {
			code, _, errOut := runCLI("", "frobnicate")
			Expect(code).To(Equal(2))
			Expect(errOut).To(ContainSubstring("unknown command"))
		})

		It("shows usage with no args", func() {
			code, _, errOut := runCLI("")
			Expect(code).To(Equal(2))
			Expect(errOut).To(ContainSubstring("usage:"))
		})

		It("prints a non-empty version", func() {
			code, out, _ := runCLI("", "version")
			Expect(code).To(Equal(0))
			Expect(strings.TrimSpace(out)).NotTo(BeEmpty())
		})
	})
})
