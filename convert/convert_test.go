package convert_test

import (
	"os"
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

func TestFromJSONNestedObjectFlattened(t *testing.T) {
	doc, err := convert.FromJSON([]byte(`{"cfg":{"db":{"host":"h"}}}`), nil)
	noerr(t, err)
	out := valid(t, doc)
	has(t, out, "cfg_db:\n")
	has(t, out, "  host: h")
}

func TestFromJSONSchemaMiscExamples(t *testing.T) {
	data, err := os.ReadFile("../testdata/schema_misc_examples.json")
	noerr(t, err)
	doc, err := convert.FromJSON(data, nil)
	noerr(t, err)
	out := valid(t, doc)
	has(t, out, "vegetables[3]{veggieLike,veggieName}:")
	has(t, out, "person_hobbies[2]{value}:")
	has(t, out, "enumerated_data[3]{value}:")
	eq(t, mustParse(t, out).FindBlock("vegetables").Rows[1], []any{false, "broccoli"})
}

func TestFromJSONSampleRowset(t *testing.T) {
	data, err := os.ReadFile("../testdata/sample.json")
	noerr(t, err)
	doc, err := convert.FromJSON(data, nil)
	noerr(t, err)
	out := valid(t, doc)
	eq(t, out, `rowset:
  affected_rows: 13
  dbname: alerts
  osname: NCOMS
  tblname: status
rowset_coldesc[11]{name,size,type}:
  Identifier,255,string
  Serial,4,integer
  Node,64,string
  NodeAlias,64,string
  AlertKey,255,string
  Severity,4,integer
  Summary,255,string
  StateChange,4,utc
  FirstOccurrence,4,utc
  LastOccurrence,4,utc
  RowSerial,4,integer
rowset_rows[3]{Identifier,Serial,Node,NodeAlias,AlertKey,Severity,Summary,StateChange,FirstOccurrence,LastOccurrence,RowSerial}:
  Startup@sol9-build1,12469,sol9-build1,"","",0,ObjectServer NCOMS on sol9-build1 started at Wed Jul 04 15:27:57 2012,1341412082,1341411978,1341412077,12469
  ProfilerEnableToggle@NCOMS:sol9-build1,12468,sol9-build1,"","",0,ObjectServer NCOMS Profiler enabled at Wed Jul 04 15:27:56 2012,1341412077,1341411976,1341412076,12468
  Shutdown@sol9-build1,null,null,null,null,null,null,null,null,null,12519
`)
}

func TestFromJSONSanitizesKeys(t *testing.T) {
	doc, err := convert.FromJSON([]byte(`{"First-Name":"a","2nd":"b","affectedRows":3,"URLValue":4}`), nil)
	noerr(t, err)
	out := valid(t, doc)
	has(t, out, "first_name: a")
	has(t, out, "k2nd: b")
	has(t, out, "affected_rows: 3")
	has(t, out, "url_value: 4")
}

func TestFromJSONRootScalar(t *testing.T) {
	doc, err := convert.FromJSON([]byte(`"hello"`), nil)
	noerr(t, err)
	has(t, valid(t, doc), "value: hello")
}

func TestFromJSONErrors(t *testing.T) {
	if _, err := convert.FromJSON([]byte(`{not json`), nil); err == nil {
		t.Fatal("expected invalid JSON error")
	}
	if _, err := convert.FromJSON([]byte("{}\n{}"), nil); err == nil || !strings.Contains(err.Error(), "ndjson/jsonl") {
		t.Fatalf("multiple JSON values error = %v", err)
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

func TestFromJSONC(t *testing.T) {
	in := []byte(`{
		// line comments are allowed
		"services": [
			{"name": "api", "url": "https://example.test",},
		],
	}`)
	doc, err := convert.FromJSONC(in, nil)
	noerr(t, err)
	out := valid(t, doc)
	has(t, out, "services[1]{name,url}:")
	has(t, out, "api,https://example.test")
}

func TestFromNDJSON(t *testing.T) {
	doc, err := convert.FromNDJSON([]byte("{\"name\":\"api\",\"replicas\":2}\n{\"name\":\"worker\",\"replicas\":1}\n"), &convert.Options{RootName: "services"})
	noerr(t, err)
	out := valid(t, doc)
	has(t, out, "services[2]{name,replicas}:")
	eq(t, mustParse(t, out).FindBlock("services").Rows, [][]any{
		{"api", 2},
		{"worker", 1},
	})
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
