package e2e

import (
	"bytes"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cloud-byte-consulting/teo"
)

func TestBinaryConvertsJSONFile(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "teo")
	build := exec.Command("go", "build", "-o", bin, "./cmd/teo")
	build.Dir = ".."
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("building teo binary: %v\n%s", err, out)
	}

	code, out, errOut := run(bin, "convert", filepath.Join("..", "testdata", "schema_misc_examples.json"))
	if code != 0 {
		t.Fatalf("code = %d, stderr = %s", code, errOut)
	}
	if err := teo.Validate(out); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"vegetables[3]{veggieLike,veggieName}:",
		"true,potato",
		"false,broccoli",
		"person_hobbies[2]{value}:",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
}

func run(bin string, args ...string) (int, string, string) {
	cmd := exec.Command(bin, args...)
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
