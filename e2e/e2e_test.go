package e2e

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cloud-byte-consulting/teo"
)

var teoBin string

func TestMain(m *testing.M) {
	tempDir, err := os.MkdirTemp("", "teo-e2e")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer os.RemoveAll(tempDir)

	teoBin = filepath.Join(tempDir, "teo")
	build := exec.Command("go", "build", "-o", teoBin, "./cmd/teo")
	build.Dir = ".."
	if out, err := build.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "building teo binary: %v\n%s", err, out)
		os.Exit(1)
	}
	os.Exit(m.Run())
}

func TestBinaryConvertsJSONFile(t *testing.T) {
	code, out, errOut := run("", "convert", dataPath("issues.json"))
	eq(t, code, 0, errOut)
	noerr(t, teo.Validate(out))
	has(t, out, "issues[3]{author,number,state,title}:")
	has(t, out, `bob,43,open,"Add dark mode, finally"`)
}

func TestBinaryMatchesJSONAndYAML(t *testing.T) {
	_, jsonOut, _ := run("", "convert", dataPath("issues.json"))
	code, yamlOut, errOut := run("", "convert", dataPath("issues.yaml"))
	eq(t, code, 0, errOut)
	eq(t, yamlOut, jsonOut)
}

func TestBinaryConvertsCSVAndJSONC(t *testing.T) {
	code, out, errOut := run("", "convert", dataPath("issues.csv"))
	eq(t, code, 0, errOut)
	noerr(t, teo.Validate(out))
	has(t, out, "items[3]{number,title,state,author}:")

	code, out, errOut = run("", "convert", dataPath("services.jsonc"))
	eq(t, code, 0, errOut)
	noerr(t, teo.Validate(out))
	has(t, out, "services[2]{name,replicas}:")
}

func TestBinaryStdinValidateAndErrors(t *testing.T) {
	code, out, errOut := run(`{"name":"acme","count":2}`, "convert")
	eq(t, code, 0, errOut)
	has(t, out, "name: acme")
	has(t, out, "count: 2")

	_, converted, _ := run("", "convert", dataPath("issues.json"))
	code, out, errOut = run(converted, "validate")
	eq(t, code, 0, errOut)
	has(t, out, "ok")

	code, _, _ = run(`{bad json`, "convert", "--from", "json")
	eq(t, code, 1)

	code, out, _ = run("", "version")
	eq(t, code, 0)
	if strings.TrimSpace(out) == "" {
		t.Fatal("empty version")
	}
}

func run(stdin string, args ...string) (int, string, string) {
	cmd := exec.Command(teoBin, args...)
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	var out, errBuf bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errBuf

	switch err := cmd.Run().(type) {
	case nil:
		return 0, out.String(), errBuf.String()
	case *exec.ExitError:
		return err.ExitCode(), out.String(), errBuf.String()
	default:
		return -1, "", err.Error()
	}
}

func dataPath(name string) string { return filepath.Join("..", "testdata", name) }

func noerr(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func has(t *testing.T, s, want string) {
	t.Helper()
	if !strings.Contains(s, want) {
		t.Fatalf("missing %q in:\n%s", want, s)
	}
}

func eq[T comparable](t *testing.T, got, want T, msg ...any) {
	t.Helper()
	if got != want {
		t.Fatalf("got %#v, want %#v: %v", got, want, msg)
	}
}
