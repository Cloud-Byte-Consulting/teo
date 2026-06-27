// Package teo emits and parses Token-Efficient Output: a line-oriented,
// indentation-structured format that declares repeated structure once and drops
// JSON's per-value punctuation. See teo-format.md for the grammar. The package
// provides a builder (Document), a Parser, and Validate, so emit/parse
// round-trips can be used as a test oracle: parse(emit(data)) reconstructs data.
//
// The core package is dependency-free (stdlib only). Conversion from
// JSON/YAML lives in the sibling package truenas-scale-1.tail5a208d.ts.net/Cloud-Byte-Consulting/teo/convert so
// that importers needing only the builder/parser take on no extra dependencies.
package teo

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// ----- value encoding / decoding -------------------------------------------------

// EncodeValue renders a scalar per the grammar (null/bool/number bare; strings
// quoted only when necessary).
func EncodeValue(v any) string {
	switch x := v.(type) {
	case nil:
		return "null"
	case bool:
		if x {
			return "true"
		}
		return "false"
	case int:
		return strconv.Itoa(x)
	case int64:
		return strconv.FormatInt(x, 10)
	case float64:
		return strconv.FormatFloat(x, 'g', -1, 64)
	case string:
		return encodeString(x)
	default:
		return encodeString(fmt.Sprint(x))
	}
}

func encodeString(s string) string {
	if s == "" {
		return `""` // empty string, distinct from null
	}
	if needsQuote(s) {
		r := strings.ReplaceAll(s, `"`, `\"`)
		r = strings.ReplaceAll(r, "\n", `\n`)
		return `"` + r + `"`
	}
	return s
}

func needsQuote(s string) bool {
	if s == "null" || s == "true" || s == "false" || looksNumeric(s) {
		return true // would be mistaken for a typed token
	}
	if s != strings.TrimSpace(s) {
		return true // leading/trailing space
	}
	if strings.ContainsAny(s, ",\"\n") {
		return true
	}
	return strings.HasPrefix(s, ": ")
}

func looksNumeric(s string) bool {
	_, err := strconv.ParseFloat(s, 64)
	return err == nil
}

// DecodeValue is the inverse of EncodeValue for a single field token.
func DecodeValue(s string) any {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		inner := s[1 : len(s)-1]
		inner = strings.ReplaceAll(inner, `\"`, "\"")
		inner = strings.ReplaceAll(inner, `\n`, "\n")
		return inner
	}
	switch s {
	case "null":
		return nil
	case "true":
		return true
	case "false":
		return false
	}
	if i, err := strconv.Atoi(s); err == nil {
		return i
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return f
	}
	return s
}

// splitRow splits a comma-separated row, honoring double-quoted values.
func splitRow(s string) []string {
	var out []string
	var cur strings.Builder
	inq := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if inq {
			cur.WriteByte(c)
			if c == '\\' && i+1 < len(s) {
				cur.WriteByte(s[i+1])
				i++
				continue
			}
			if c == '"' {
				inq = false
			}
			continue
		}
		switch c {
		case '"':
			inq = true
			cur.WriteByte(c)
		case ',':
			out = append(out, cur.String())
			cur.Reset()
		default:
			cur.WriteByte(c)
		}
	}
	out = append(out, cur.String())
	return out
}

// ----- document model -----------------------------------------------------------

type Kind int

const (
	KScalar Kind = iota
	KBlock
	KHelp
	KRecord
)

// KV is a key/value pair (scalar or record field).
type KV struct {
	Key   string
	Value any
}

// Item is one logical element of a TEO document.
type Item struct {
	Kind   Kind
	Key    string   // scalar key / block name / record key
	Value  any      // scalar value
	Fields []string // block field schema
	Rows   [][]any  // block rows (typed)
	Help   []string // help suggestion lines
	Record []KV     // record fields
}

// Document is an ordered list of items.
type Document struct {
	Items []Item
}

// ----- builder ------------------------------------------------------------------

// New returns an empty document for building.
func New() *Document { return &Document{} }

// Scalar appends a `key: value` line.
func (d *Document) Scalar(key string, v any) *Document {
	d.Items = append(d.Items, Item{Kind: KScalar, Key: key, Value: v})
	return d
}

// Count appends a count metadata line: `count: n` or `count: e of t total`.
func (d *Document) Count(emitted int, total ...int) *Document {
	if len(total) == 1 && total[0] != emitted {
		return d.Scalar("count", fmt.Sprintf("%d of %d total", emitted, total[0]))
	}
	return d.Scalar("count", emitted)
}

// Help appends a help block.
func (d *Document) Help(lines ...string) *Document {
	d.Items = append(d.Items, Item{Kind: KHelp, Help: lines})
	return d
}

// Record appends a record: a `key:` header followed by indented scalar fields.
// It is the builder counterpart to the KRecord form the parser already
// reconstructs, and is used by the converter for nested all-scalar objects.
func (d *Document) Record(key string, kvs ...KV) *Document {
	d.Items = append(d.Items, Item{Kind: KRecord, Key: key, Record: kvs})
	return d
}

// Block appends a block and returns a handle to add rows.
func (d *Document) Block(name string, fields ...string) *BlockHandle {
	d.Items = append(d.Items, Item{Kind: KBlock, Key: name, Fields: fields})
	return &BlockHandle{item: &d.Items[len(d.Items)-1]}
}

// BlockHandle accumulates rows for a block.
type BlockHandle struct{ item *Item }

// Row adds one positional row to the block.
func (b *BlockHandle) Row(vals ...any) *BlockHandle {
	b.item.Rows = append(b.item.Rows, vals)
	return b
}

// Encode renders the document as TEO text.
func (d *Document) String() string {
	var b strings.Builder
	for _, it := range d.Items {
		switch it.Kind {
		case KScalar:
			fmt.Fprintf(&b, "%s: %s\n", it.Key, EncodeValue(it.Value))
		case KRecord:
			fmt.Fprintf(&b, "%s:\n", it.Key)
			for _, kv := range it.Record {
				fmt.Fprintf(&b, "  %s: %s\n", kv.Key, EncodeValue(kv.Value))
			}
		case KBlock:
			fmt.Fprintf(&b, "%s[%d]{%s}:\n", it.Key, len(it.Rows), strings.Join(it.Fields, ","))
			for _, row := range it.Rows {
				cells := make([]string, len(row))
				for i, v := range row {
					cells[i] = EncodeValue(v)
				}
				fmt.Fprintf(&b, "  %s\n", strings.Join(cells, ","))
			}
		case KHelp:
			fmt.Fprintf(&b, "help[%d]:\n", len(it.Help))
			for _, l := range it.Help {
				fmt.Fprintf(&b, "  %s\n", l)
			}
		}
	}
	return b.String()
}

// ----- parser -------------------------------------------------------------------

var (
	reBlock  = regexp.MustCompile(`^([a-z][a-z0-9_]*)\[(\d+)\]\{([^}]*)\}:$`)
	reHelp   = regexp.MustCompile(`^help\[(\d+)\]:$`)
	reRecord = regexp.MustCompile(`^([a-z][a-z0-9_]*):$`)
	reScalar = regexp.MustCompile(`^([a-z][a-z0-9_]*): (.*)$`)
)

func indented(line string) bool { return strings.HasPrefix(line, "  ") }

// Parse reconstructs a Document from TEO text. It is strict: declared block/help
// counts must match the number of indented lines that follow.
func Parse(s string) (*Document, error) {
	lines := strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
	// drop a single trailing empty line from the final newline
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	doc := &Document{}
	i := 0
	for i < len(lines) {
		line := lines[i]
		if line == "" {
			i++
			continue
		}
		if indented(line) {
			return nil, fmt.Errorf("line %d: unexpected indentation: %q", i+1, line)
		}
		switch {
		case reBlock.MatchString(line):
			m := reBlock.FindStringSubmatch(line)
			n, _ := strconv.Atoi(m[2])
			var fields []string
			if m[3] != "" {
				fields = strings.Split(m[3], ",")
			}
			rows := make([][]any, 0, n)
			i++
			for r := 0; r < n; r++ {
				if i >= len(lines) || !indented(lines[i]) {
					return nil, fmt.Errorf("block %q declared %d rows, found %d", m[1], n, r)
				}
				cells := splitRow(strings.TrimPrefix(lines[i], "  "))
				row := make([]any, len(cells))
				for c, cell := range cells {
					row[c] = DecodeValue(cell)
				}
				rows = append(rows, row)
				i++
			}
			doc.Items = append(doc.Items, Item{Kind: KBlock, Key: m[1], Fields: fields, Rows: rows})
		case reHelp.MatchString(line):
			n, _ := strconv.Atoi(reHelp.FindStringSubmatch(line)[1])
			help := make([]string, 0, n)
			i++
			for r := 0; r < n; r++ {
				if i >= len(lines) || !indented(lines[i]) {
					return nil, fmt.Errorf("help declared %d lines, found %d", n, r)
				}
				help = append(help, strings.TrimPrefix(lines[i], "  "))
				i++
			}
			doc.Items = append(doc.Items, Item{Kind: KHelp, Help: help})
		case reScalar.MatchString(line):
			m := reScalar.FindStringSubmatch(line)
			doc.Items = append(doc.Items, Item{Kind: KScalar, Key: m[1], Value: DecodeValue(m[2])})
			i++
		case reRecord.MatchString(line):
			m := reRecord.FindStringSubmatch(line)
			var fields []KV
			i++
			for i < len(lines) && indented(lines[i]) {
				kvm := reScalar.FindStringSubmatch(strings.TrimPrefix(lines[i], "  "))
				if kvm == nil {
					return nil, fmt.Errorf("record %q: bad field line %q", m[1], lines[i])
				}
				fields = append(fields, KV{Key: kvm[1], Value: DecodeValue(kvm[2])})
				i++
			}
			doc.Items = append(doc.Items, Item{Kind: KRecord, Key: m[1], Record: fields})
		default:
			return nil, fmt.Errorf("line %d: not valid TEO: %q", i+1, line)
		}
	}
	return doc, nil
}

// Validate reports whether s is a well-formed TEO document.
func Validate(s string) error {
	_, err := Parse(s)
	return err
}

// FindBlock returns the first block with the given name (nil if absent).
func (d *Document) FindBlock(name string) *Item {
	for i := range d.Items {
		if d.Items[i].Kind == KBlock && d.Items[i].Key == name {
			return &d.Items[i]
		}
	}
	return nil
}

// GetScalar returns the value of the first scalar with the given key (nil if absent).
func (d *Document) GetScalar(key string) any {
	for _, it := range d.Items {
		if it.Kind == KScalar && it.Key == key {
			return it.Value
		}
	}
	return nil
}
