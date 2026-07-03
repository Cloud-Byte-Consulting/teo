package convert_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/cloud-byte-consulting/teo"
	"github.com/cloud-byte-consulting/teo/convert"
)

func TestFromJSONScalars(t *testing.T) {
	doc, err := convert.FromJSON([]byte(`{"name":"acme","open":true,"count":3,"ratio":0.5,"note":null}`), nil)
	noerr(t, err)
	out := valid(t, doc)
	for _, want := range []string{"name: acme", "open: true", "count: 3", "ratio: 0.5", "note: null"} {
		has(t, out, want)
	}
}

func TestFromJSONArrayOfObjects(t *testing.T) {
	doc, err := convert.FromJSON([]byte(`{"issues":[{"number":1,"title":"a"},{"number":2,"title":"b, c"}]}`), nil)
	noerr(t, err)
	out := valid(t, doc)
	has(t, out, "issues[2]{number,title}:")
	has(t, out, `2,"b, c"`)

	blk := mustParse(t, out).FindBlock("issues")
	eq(t, len(blk.Rows), 2)
	eq(t, blk.Rows[1][1], "b, c")
}

func TestFromJSONRaggedObjectKeys(t *testing.T) {
	doc, err := convert.FromJSON([]byte(`[{"a":1,"b":2},{"a":3,"c":4}]`), &convert.Options{RootName: "rows"})
	noerr(t, err)
	out := valid(t, doc)
	has(t, out, "rows[2]{a,b,c}:")

	rows := mustParse(t, out).FindBlock("rows").Rows
	eq(t, rows[0][2], nil)
	eq(t, rows[1][1], nil)
}

func TestFromJSONArrayOfScalars(t *testing.T) {
	doc, err := convert.FromJSON([]byte(`{"tags":["x","y","z"]}`), nil)
	noerr(t, err)
	has(t, valid(t, doc), "tags[3]{value}:")
}

func TestFromJSONEscapesBlockFieldNames(t *testing.T) {
	doc, err := convert.FromJSON([]byte(`[{"a,b":1,"c}d":2,"ok":3}]`), &convert.Options{RootName: "rows"})
	noerr(t, err)
	blk := mustParse(t, valid(t, doc)).FindBlock("rows")
	eq(t, blk.Fields, []string{"a_b", "c_d", "ok"})
	eq(t, blk.Rows[0], []any{1, 2, 3})
}

func TestFromJSONNestedScalarObject(t *testing.T) {
	doc, err := convert.FromJSON([]byte(`{"meta":{"owner":"alice","count":2}}`), nil)
	noerr(t, err)
	out := valid(t, doc)
	has(t, out, "meta:\n")
	has(t, out, "  owner: alice")
}

func TestFromJSONNestedObjectAsScalar(t *testing.T) {
	doc, err := convert.FromJSON([]byte(`{"cfg":{"db":{"host":"h"}}}`), nil)
	noerr(t, err)
	has(t, valid(t, doc), `cfg: "{\"db\":{\"host\":\"h\"}}"`)
}

func TestFromJSONSanitizesKeys(t *testing.T) {
	doc, err := convert.FromJSON([]byte(`{"First-Name":"a","2nd":"b"}`), nil)
	noerr(t, err)
	out := valid(t, doc)
	has(t, out, "first_name: a")
	has(t, out, "k2nd: b")
}

func TestFromJSONRootScalar(t *testing.T) {
	doc, err := convert.FromJSON([]byte(`"hello"`), nil)
	noerr(t, err)
	has(t, valid(t, doc), "value: hello")
}

func TestFromJSONInvalid(t *testing.T) {
	if _, err := convert.FromJSON([]byte(`{not json`), nil); err == nil {
		t.Fatal("expected error")
	}
}

func TestFromJSONDeterministic(t *testing.T) {
	in := []byte(`{"c":1,"a":2,"b":3}`)
	first, err := convert.FromJSON(in, nil)
	noerr(t, err)
	for i := 0; i < 20; i++ {
		again, err := convert.FromJSON(in, nil)
		noerr(t, err)
		eq(t, again.String(), first.String())
	}
}

func TestFromYAMLMatchesJSON(t *testing.T) {
	yDoc, err := convert.FromYAML([]byte("issues:\n  - number: 1\n    title: a\n  - number: 2\n    title: b\n"), nil)
	noerr(t, err)
	jDoc, err := convert.FromJSON([]byte(`{"issues":[{"number":1,"title":"a"},{"number":2,"title":"b"}]}`), nil)
	noerr(t, err)
	eq(t, yDoc.String(), jDoc.String())
}

func valid(t *testing.T, doc *teo.Document) string {
	t.Helper()
	out := doc.String()
	noerr(t, teo.Validate(out))
	return out
}

func mustParse(t *testing.T, s string) *teo.Document {
	t.Helper()
	doc, err := teo.Parse(s)
	noerr(t, err)
	return doc
}

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

func eq(t *testing.T, got, want any) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}
