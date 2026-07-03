package teo_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/cloud-byte-consulting/teo"
)

func TestEncodeValue(t *testing.T) {
	for _, tt := range []struct {
		in   any
		want string
	}{
		{nil, "null"},
		{true, "true"},
		{42, "42"},
		{3.14, "3.14"},
		{"Fix login bug", "Fix login bug"},
		{"", `""`},
		{"Add dark mode, finally", `"Add dark mode, finally"`},
		{`He said "hi"`, `"He said \"hi\""`},
		{"null", `"null"`},
		{"42", `"42"`},
		{" x", `" x"`},
	} {
		eq(t, teo.EncodeValue(tt.in), tt.want)
	}
}

func TestDecodeValueRoundTrip(t *testing.T) {
	for _, in := range []any{nil, true, 8771, "alice", "Add dark mode, finally", `He said "hi"`, "true", "42", ""} {
		eq(t, teo.DecodeValue(teo.EncodeValue(in)), in)
	}
}

func TestDocumentRoundTrip(t *testing.T) {
	doc := teo.New()
	doc.Count(14, 8771)
	doc.Scalar("description", "open issues for acme/widgets")
	doc.Block("issues", "number", "title", "state", "author").
		Row(42, "Fix login bug", "open", "alice").
		Row(43, "Add dark mode, finally", "open", "bob").
		Row(44, "Crash on empty input", "open", nil)
	doc.Help("Run `air issue view <number> --teo`")

	out := doc.String()
	has(t, out, "issues[3]{number,title,state,author}:")
	has(t, out, `43,"Add dark mode, finally",open,bob`)
	has(t, out, "44,Crash on empty input,open,null")
	has(t, out, "count: 14 of 8771 total")

	parsed, err := teo.Parse(out)
	noerr(t, err)
	blk := parsed.FindBlock("issues")
	if blk == nil {
		t.Fatal("missing issues block")
	}
	eq(t, blk.Fields, []string{"number", "title", "state", "author"})
	eq(t, len(blk.Rows), 3)
	eq(t, blk.Rows[1], []any{43, "Add dark mode, finally", "open", "bob"})
	eq(t, blk.Rows[2], []any{44, "Crash on empty input", "open", nil})
}

func TestRecordBuilder(t *testing.T) {
	doc := teo.New().Record("meta",
		teo.KV{Key: "owner", Value: "alice"},
		teo.KV{Key: "count", Value: 3},
		teo.KV{Key: "active", Value: true},
	)
	out := doc.String()
	has(t, out, "meta:\n")
	has(t, out, "  owner: alice")
	has(t, out, "  count: 3")

	parsed, err := teo.Parse(out)
	noerr(t, err)
	eq(t, len(parsed.Items), 1)
	eq(t, parsed.Items[0].Kind, teo.KRecord)
	eq(t, parsed.Items[0].Record, []teo.KV{
		{Key: "owner", Value: "alice"},
		{Key: "count", Value: 3},
		{Key: "active", Value: true},
	})
}

func TestEmptyState(t *testing.T) {
	doc := teo.New()
	doc.Count(0)
	doc.Block("issues", "number", "title", "state")
	out := doc.String()
	has(t, out, "count: 0")
	has(t, out, "issues[0]{number,title,state}:")

	parsed, err := teo.Parse(out)
	noerr(t, err)
	eq(t, parsed.GetScalar("count"), 0)
	eq(t, len(parsed.FindBlock("issues").Rows), 0)
}

func TestValidate(t *testing.T) {
	for _, tt := range []struct {
		in      string
		wantErr bool
	}{
		{"count: 2\nxs[2]{a,b}:\n  1,2\n  3,4\n", false},
		{"xs[3]{a}:\n  1\n  2\n", true},
		{"  oops: 1\n", true},
		{"this is not teo\n", true},
	} {
		if (teo.Validate(tt.in) != nil) != tt.wantErr {
			t.Fatalf("Validate(%q) wantErr %v", tt.in, tt.wantErr)
		}
	}
}

func TestHelpBlock(t *testing.T) {
	parsed, err := teo.Parse(teo.New().Help("Run `air status --teo`", "Run `air install --teo`").String())
	noerr(t, err)
	eq(t, len(parsed.Items), 1)
	eq(t, len(parsed.Items[0].Help), 2)
	has(t, parsed.Items[0].Help[0], "air status --teo")
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
