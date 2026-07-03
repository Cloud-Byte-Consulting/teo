package cli_test

import (
	"bytes"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/cloud-byte-consulting/teo"
	"github.com/cloud-byte-consulting/teo/internal/cli"
)

func TestConvertJSONFile(t *testing.T) {
	code, out, errOut := runCLI("", "convert", dataPath("issues.json"))
	eq(t, code, 0, errOut)
	noerr(t, teo.Validate(out))
	has(t, out, "issues[3]{author,number,state,title}:")
}

func TestConvertJSONAndYAMLMatch(t *testing.T) {
	_, jsonOut, _ := runCLI("", "convert", dataPath("issues.json"))
	code, yamlOut, errOut := runCLI("", "convert", dataPath("issues.yaml"))
	eq(t, code, 0, errOut)
	eq(t, yamlOut, jsonOut)
}

func TestConvertTabularFiles(t *testing.T) {
	code, out, errOut := runCLI("", "convert", dataPath("issues.csv"))
	eq(t, code, 0, errOut)
	noerr(t, teo.Validate(out))
	has(t, out, "items[3]{number,title,state,author}:")
	parsed, err := teo.Parse(out)
	noerr(t, err)
	eq(t, parsed.FindBlock("items").Rows[0], []any{"42", "Fix login bug", "open", "alice"})

	code, out, errOut = runCLI("", "convert", dataPath("issues.tsv"))
	eq(t, code, 0, errOut)
	noerr(t, teo.Validate(out))
	has(t, out, "items[3]{number,title,state,author}:")
}

func TestConvertJSONCAndNDJSON(t *testing.T) {
	code, out, errOut := runCLI("", "convert", dataPath("services.jsonc"))
	eq(t, code, 0, errOut)
	noerr(t, teo.Validate(out))
	has(t, out, "services[2]{name,replicas}:")

	code, out, errOut = runCLI("", "convert", "--name", "services", dataPath("services.ndjson"))
	eq(t, code, 0, errOut)
	noerr(t, teo.Validate(out))
	has(t, out, "services[2]{name,replicas}:")
}

func TestConvertDetectsStdin(t *testing.T) {
	code, out, _ := runCLI(`{"name":"acme","tags":["a","b"]}`, "convert")
	eq(t, code, 0)
	has(t, out, "name: acme")
	has(t, out, "tags[2]{value}:")

	code, out, _ = runCLI("name: acme\ncount: 2\n", "convert")
	eq(t, code, 0)
	has(t, out, "name: acme")
	has(t, out, "count: 2")

	code, out, errOut := runCLI("{\"name\":\"api\"}\n{\"name\":\"worker\"}\n", "convert", "--name", "services")
	eq(t, code, 0, errOut)
	has(t, out, "services[2]{name}:")
}

func TestConvertFlags(t *testing.T) {
	code, out, _ := runCLI(`{"a":1}`, "convert", "--from", "json", "-")
	eq(t, code, 0)
	has(t, out, "a: 1")

	code, out, errOut := runCLI("alice,open\nbob,closed\n", "convert", "--from", "csv", "--no-header", "--name", "rows")
	eq(t, code, 0, errOut)
	has(t, out, "rows[2]{col1,col2}:")

	code, out, _ = runCLI(`[{"x":1}]`, "convert", "--name", "rows")
	eq(t, code, 0)
	has(t, out, "rows[1]{x}:")
}

func TestConvertErrors(t *testing.T) {
	code, _, errOut := runCLI(`{bad`, "convert", "--from", "json")
	if code == 0 {
		t.Fatal("expected non-zero exit")
	}
	has(t, errOut, "parse json")

	code, _, _ = runCLI("", "convert", "does-not-exist.json")
	eq(t, code, 1)
}

func TestValidate(t *testing.T) {
	_, converted, _ := runCLI(`{"a":1}`, "convert")
	code, out, errOut := runCLI(converted, "validate")
	eq(t, code, 0, errOut)
	has(t, out, "ok")

	code, _, errOut = runCLI("this is not teo\n", "validate")
	eq(t, code, 1)
	has(t, errOut, "invalid TEO")
}

func TestDispatch(t *testing.T) {
	code, _, errOut := runCLI("", "frobnicate")
	eq(t, code, 2)
	has(t, errOut, "unknown command")

	code, _, errOut = runCLI("")
	eq(t, code, 2)
	has(t, errOut, "usage:")

	code, out, _ := runCLI("", "version")
	eq(t, code, 0)
	if strings.TrimSpace(out) == "" {
		t.Fatal("empty version")
	}
}

func runCLI(stdin string, args ...string) (int, string, string) {
	var out, errBuf bytes.Buffer
	code := cli.Run(args, strings.NewReader(stdin), &out, &errBuf)
	return code, out.String(), errBuf.String()
}

func dataPath(name string) string { return filepath.Join("..", "..", "testdata", name) }

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

func eq(t *testing.T, got, want any, msg ...any) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v: %v", got, want, msg)
	}
}
