package e2e

import (
	"bytes"
	"os/exec"
	"path/filepath"
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

	code, out, errOut := runBinary(t, bin, "convert", filepath.Join("..", "testdata", "issues.json"))
	if code != 0 {
		t.Fatalf("code = %d, stderr = %s", code, errOut)
	}
	if err := teo.Validate(out); err != nil {
		t.Fatal(err)
	}
	if want := `bob,43,open,"Add dark mode, finally"`; !bytes.Contains([]byte(out), []byte(want)) {
		t.Fatalf("missing %q in:\n%s", want, out)
	}
}

func runBinary(t *testing.T, bin string, args ...string) (int, string, string) {
	t.Helper()
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
		t.Fatalf("running binary: %v", err)
		return -1, "", ""
	}
}
