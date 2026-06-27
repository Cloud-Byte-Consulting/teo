package convert_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/cloud-byte-consulting/teo"
	"github.com/cloud-byte-consulting/teo/convert"
)

// mustValidTEO asserts the rendered document re-parses, the invariant every
// conversion must hold, and returns the rendered text.
func mustValidTEO(doc *teo.Document) string {
	out := doc.String()
	ExpectWithOffset(1, teo.Validate(out)).To(Succeed(), "converted output is not valid TEO:\n%s", out)
	return out
}

var _ = Describe("convert", func() {
	Describe("FromJSON", func() {
		It("renders root object scalars", func() {
			doc, err := convert.FromJSON([]byte(`{"name":"acme","open":true,"count":3,"ratio":0.5,"note":null}`), nil)
			Expect(err).NotTo(HaveOccurred())
			out := mustValidTEO(doc)
			for _, want := range []string{"name: acme", "open: true", "count: 3", "ratio: 0.5", "note: null"} {
				Expect(out).To(ContainSubstring(want))
			}
		})

		It("turns an array of objects into a block, quoting commas", func() {
			doc, err := convert.FromJSON([]byte(`{"issues":[{"number":1,"title":"a"},{"number":2,"title":"b, c"}]}`), nil)
			Expect(err).NotTo(HaveOccurred())
			out := mustValidTEO(doc)
			Expect(out).To(ContainSubstring("issues[2]{number,title}:"))
			Expect(out).To(ContainSubstring(`2,"b, c"`)) // comma stays inside one cell
			parsed, _ := teo.Parse(out)
			blk := parsed.FindBlock("issues")
			Expect(blk.Rows).To(HaveLen(2))
			Expect(blk.Rows[1][1]).To(Equal("b, c"))
		})

		It("unions ragged object keys, filling gaps with null", func() {
			doc, err := convert.FromJSON([]byte(`[{"a":1,"b":2},{"a":3,"c":4}]`), &convert.Options{RootName: "rows"})
			Expect(err).NotTo(HaveOccurred())
			out := mustValidTEO(doc)
			Expect(out).To(ContainSubstring("rows[2]{a,b,c}:"))
			parsed, _ := teo.Parse(out)
			rows := parsed.FindBlock("rows").Rows
			Expect(rows[0][2]).To(BeNil()) // missing c
			Expect(rows[1][1]).To(BeNil()) // missing b
		})

		It("renders an array of scalars as a single-column block", func() {
			doc, err := convert.FromJSON([]byte(`{"tags":["x","y","z"]}`), nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(mustValidTEO(doc)).To(ContainSubstring("tags[3]{value}:"))
		})

		It("escapes block field names that contain TEO delimiters", func() {
			// keys with ',' and '}' would otherwise break the block header
			doc, err := convert.FromJSON([]byte(`[{"a,b":1,"c}d":2,"ok":3}]`), &convert.Options{RootName: "rows"})
			Expect(err).NotTo(HaveOccurred())
			out := mustValidTEO(doc) // must re-parse despite the nasty keys
			parsed, _ := teo.Parse(out)
			blk := parsed.FindBlock("rows")
			Expect(blk.Fields).To(HaveLen(3))                          // no spurious comma-split
			Expect(blk.Fields).To(Equal([]string{"a_b", "c_d", "ok"})) // delimiters -> '_'
			Expect(blk.Rows[0]).To(Equal([]any{1, 2, 3}))
		})

		It("renders a nested all-scalar object as a record", func() {
			doc, err := convert.FromJSON([]byte(`{"meta":{"owner":"alice","count":2}}`), nil)
			Expect(err).NotTo(HaveOccurred())
			out := mustValidTEO(doc)
			Expect(out).To(ContainSubstring("meta:\n"))
			Expect(out).To(ContainSubstring("  owner: alice"))
		})

		It("JSON-encodes a deeper object onto one scalar line", func() {
			doc, err := convert.FromJSON([]byte(`{"cfg":{"db":{"host":"h"}}}`), nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(mustValidTEO(doc)).To(ContainSubstring(`cfg: "{\"db\":{\"host\":\"h\"}}"`))
		})

		It("sanitizes keys to the TEO grammar", func() {
			doc, err := convert.FromJSON([]byte(`{"First-Name":"a","2nd":"b"}`), nil)
			Expect(err).NotTo(HaveOccurred())
			out := mustValidTEO(doc)
			Expect(out).To(ContainSubstring("first_name: a")) // lowercased, '-' -> '_'
			Expect(out).To(ContainSubstring("k2nd: b"))       // letter-prefixed
		})

		It("renders a root scalar as value", func() {
			doc, err := convert.FromJSON([]byte(`"hello"`), nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(mustValidTEO(doc)).To(ContainSubstring("value: hello"))
		})

		It("errors on invalid JSON", func() {
			_, err := convert.FromJSON([]byte(`{not json`), nil)
			Expect(err).To(HaveOccurred())
		})

		It("is deterministic across repeated runs", func() {
			in := []byte(`{"c":1,"a":2,"b":3}`)
			first, _ := convert.FromJSON(in, nil)
			for i := 0; i < 20; i++ {
				again, _ := convert.FromJSON(in, nil)
				Expect(again.String()).To(Equal(first.String()))
			}
		})
	})

	Describe("FromYAML", func() {
		It("produces the same document as the equivalent JSON", func() {
			yamlIn := "issues:\n  - number: 1\n    title: a\n  - number: 2\n    title: b\n"
			jsonIn := `{"issues":[{"number":1,"title":"a"},{"number":2,"title":"b"}]}`
			yDoc, err := convert.FromYAML([]byte(yamlIn), nil)
			Expect(err).NotTo(HaveOccurred())
			jDoc, err := convert.FromJSON([]byte(jsonIn), nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(yDoc.String()).To(Equal(jDoc.String()))
		})
	})
})
