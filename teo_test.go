package teo_test

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"truenas-scale-1.tail5a208d.ts.net/Cloud-Byte-Consulting/teo"
)

var _ = Describe("TEO", func() {
	Describe("value encoding", func() {
		DescribeTable("EncodeValue",
			func(in any, want string) { Expect(teo.EncodeValue(in)).To(Equal(want)) },
			Entry("null", nil, "null"),
			Entry("true", true, "true"),
			Entry("int", 42, "42"),
			Entry("float", 3.14, "3.14"),
			Entry("plain string", "Fix login bug", "Fix login bug"),
			Entry("empty string", "", `""`),
			Entry("comma → quoted", "Add dark mode, finally", `"Add dark mode, finally"`),
			Entry("quotes → escaped", `He said "hi"`, `"He said \"hi\""`),
			Entry("reserved word as string → quoted", "null", `"null"`),
			Entry("numeric string → quoted", "42", `"42"`),
			Entry("leading space → quoted", " x", `" x"`),
		)

		DescribeTable("round-trips through DecodeValue",
			func(in any) {
				got := teo.DecodeValue(teo.EncodeValue(in))
				if in == nil {
					Expect(got).To(BeNil())
					return
				}
				Expect(got).To(Equal(in))
			},
			Entry("null", nil),
			Entry("bool", true),
			Entry("int", 8771),
			Entry("plain string", "alice"),
			Entry("comma string", "Add dark mode, finally"),
			Entry("quoted string", `He said "hi"`),
			Entry("reserved as string", "true"),
			Entry("numeric as string", "42"),
			Entry("empty string", ""),
		)
	})

	Describe("document round-trip (the spec oracle)", func() {
		It("reconstructs blocks, quoting, and null", func() {
			doc := teo.New()
			doc.Count(14, 8771)
			doc.Scalar("description", "open issues for acme/widgets")
			doc.Block("issues", "number", "title", "state", "author").
				Row(42, "Fix login bug", "open", "alice").
				Row(43, "Add dark mode, finally", "open", "bob").
				Row(44, "Crash on empty input", "open", nil)
			doc.Help("Run `air issue view <number> --teo`")

			out := doc.String()
			Expect(out).To(ContainSubstring("issues[3]{number,title,state,author}:"))
			Expect(out).To(ContainSubstring(`43,"Add dark mode, finally",open,bob`))
			Expect(out).To(ContainSubstring("44,Crash on empty input,open,null"))
			Expect(out).To(ContainSubstring("count: 14 of 8771 total"))

			parsed, err := teo.Parse(out)
			Expect(err).NotTo(HaveOccurred())
			blk := parsed.FindBlock("issues")
			Expect(blk).NotTo(BeNil())
			Expect(blk.Fields).To(Equal([]string{"number", "title", "state", "author"}))
			Expect(blk.Rows).To(HaveLen(3))
			// row 43 keeps its comma; row 44 author is typed null
			Expect(blk.Rows[1]).To(Equal([]any{43, "Add dark mode, finally", "open", "bob"}))
			Expect(blk.Rows[2]).To(Equal([]any{44, "Crash on empty input", "open", nil}))
		})
	})

	Describe("record builder", func() {
		It("emits a key header with indented scalar fields that re-parse", func() {
			doc := teo.New().Record("meta",
				teo.KV{Key: "owner", Value: "alice"},
				teo.KV{Key: "count", Value: 3},
				teo.KV{Key: "active", Value: true},
			)
			out := doc.String()
			Expect(out).To(ContainSubstring("meta:\n"))
			Expect(out).To(ContainSubstring("  owner: alice"))
			Expect(out).To(ContainSubstring("  count: 3"))

			parsed, err := teo.Parse(out)
			Expect(err).NotTo(HaveOccurred())
			Expect(parsed.Items).To(HaveLen(1))
			Expect(parsed.Items[0].Kind).To(Equal(teo.KRecord))
			Expect(parsed.Items[0].Record).To(Equal([]teo.KV{
				{Key: "owner", Value: "alice"},
				{Key: "count", Value: 3},
				{Key: "active", Value: true},
			}))
		})
	})

	Describe("empty state", func() {
		It("emits count 0 and a row-less block that parses", func() {
			doc := teo.New()
			doc.Count(0)
			doc.Block("issues", "number", "title", "state")
			out := doc.String()
			Expect(out).To(ContainSubstring("count: 0"))
			Expect(out).To(ContainSubstring("issues[0]{number,title,state}:"))
			parsed, err := teo.Parse(out)
			Expect(err).NotTo(HaveOccurred())
			Expect(parsed.GetScalar("count")).To(Equal(0))
			Expect(parsed.FindBlock("issues").Rows).To(BeEmpty())
		})
	})

	Describe("Validate", func() {
		It("accepts well-formed TEO", func() {
			Expect(teo.Validate("count: 2\nxs[2]{a,b}:\n  1,2\n  3,4\n")).To(Succeed())
		})
		It("rejects a block whose row count is wrong", func() {
			Expect(teo.Validate("xs[3]{a}:\n  1\n  2\n")).NotTo(Succeed())
		})
		It("rejects stray indentation", func() {
			Expect(teo.Validate("  oops: 1\n")).NotTo(Succeed())
		})
		It("rejects a non-TEO line", func() {
			Expect(teo.Validate("this is not teo\n")).NotTo(Succeed())
		})
	})

	Describe("help block", func() {
		It("round-trips suggestion lines verbatim", func() {
			doc := teo.New().Help("Run `air status --teo`", "Run `air install --teo`")
			parsed, err := teo.Parse(doc.String())
			Expect(err).NotTo(HaveOccurred())
			Expect(parsed.Items).To(HaveLen(1))
			Expect(parsed.Items[0].Help).To(HaveLen(2))
			Expect(strings.Contains(parsed.Items[0].Help[0], "air status --teo")).To(BeTrue())
		})
	})
})
