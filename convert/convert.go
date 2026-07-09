// Package convert turns standard data formats into a TEO document.
//
// TEO is two-level: top-level scalars, named blocks (arrays of uniform
// objects), and records (an object of scalar fields). Arbitrary structured
// input has no single canonical TEO projection, so this package applies a fixed,
// documented policy:
//
//   - Root object → each key emitted by value shape (below).
//   - Root array of objects → one block named by Options.RootName ("items").
//   - Root array of scalars → block RootName with a single "value" column.
//   - Root scalar → `value: <scalar>`.
//
// Per key inside a structured object:
//
//   - scalar (string/number/bool/null) → `key: value`.
//   - array of objects → block keyed by the union of element keys (sorted;
//     decoders drop source key order); missing fields are null; non-scalar
//     cells are JSON-encoded.
//   - array of scalars/mixed → block `key[n]{value}` with one column.
//   - object whose values are all scalars → record.
//   - object with nested objects/arrays → scalar fields emitted as a record;
//     nested fields emitted as prefixed records/blocks.
//
// Object/record/block *names* are sanitized to the TEO key grammar
// (`[a-z][a-z0-9_]*`): lowercased, non-conforming runes replaced with `_`, and
// a `k` prefixed when the first rune is not a lowercase letter. Block *field*
// names are emitted as-is except for the field delimiters (`,`, `}`, and
// newlines), which are replaced with `_` so the row still parses. Sanitization
// can collide (e.g. "a-b" and "a_b"); that is an accepted limitation of
// projecting onto the stricter TEO key space.
package convert

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/cloud-byte-consulting/teo"
	yaml "go.yaml.in/yaml/v3"
)

// Options configures conversion.
type Options struct {
	// RootName is the block name used when the document root is an array.
	// Defaults to "items".
	RootName string
}

func (o *Options) rootName() string {
	if o != nil && o.RootName != "" {
		return o.RootName
	}
	return "items"
}

// FromJSON parses JSON bytes and converts them to a TEO document.
func FromJSON(data []byte, o *Options) (*teo.Document, error) {
	v, err := decodeJSON(data)
	if err != nil {
		return nil, fmt.Errorf("parse json: %w", err)
	}
	return FromValue(normalize(v), o)
}

// FromJSONC parses JSON with comments and trailing commas, then converts it to
// a TEO document.
func FromJSONC(data []byte, o *Options) (*teo.Document, error) {
	clean, err := stripJSONC(data)
	if err != nil {
		return nil, fmt.Errorf("parse jsonc: %w", err)
	}
	v, err := decodeJSON(clean)
	if err != nil {
		return nil, fmt.Errorf("parse jsonc: %w", err)
	}
	return FromValue(normalize(v), o)
}

// FromNDJSON parses newline-delimited JSON bytes and converts them to a TEO
// document. Blank lines are ignored. FromJSONL is an alias for callers that use
// that name for the same line-delimited JSON format.
func FromNDJSON(data []byte, o *Options) (*teo.Document, error) {
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	values := make([]any, 0, len(lines))
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		v, err := decodeJSON([]byte(line))
		if err != nil {
			return nil, fmt.Errorf("parse ndjson line %d: %w", i+1, err)
		}
		values = append(values, normalize(v))
	}
	return FromValue(values, o)
}

// FromJSONL parses JSON Lines input. It is equivalent to FromNDJSON.
func FromJSONL(data []byte, o *Options) (*teo.Document, error) {
	return FromNDJSON(data, o)
}

func decodeJSON(data []byte) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("multiple JSON values; use ndjson/jsonl for line-delimited input")
		}
		return nil, err
	}
	return v, nil
}

func stripJSONC(data []byte) ([]byte, error) {
	withoutComments, err := stripJSONCComments(data)
	if err != nil {
		return nil, err
	}
	return stripTrailingCommas(withoutComments), nil
}

func stripJSONCComments(data []byte) ([]byte, error) {
	out := make([]byte, 0, len(data))
	inString := false
	escaped := false
	for i := 0; i < len(data); i++ {
		c := data[i]
		if inString {
			out = append(out, c)
			if escaped {
				escaped = false
				continue
			}
			switch c {
			case '\\':
				escaped = true
			case '"':
				inString = false
			}
			continue
		}

		if c == '"' {
			inString = true
			out = append(out, c)
			continue
		}
		if c == '/' && i+1 < len(data) {
			switch data[i+1] {
			case '/':
				i += 2
				for i < len(data) && data[i] != '\n' && data[i] != '\r' {
					i++
				}
				if i < len(data) {
					out = append(out, data[i])
				}
				continue
			case '*':
				i += 2
				closed := false
				for i < len(data)-1 {
					if data[i] == '\n' || data[i] == '\r' {
						out = append(out, data[i])
					}
					if data[i] == '*' && data[i+1] == '/' {
						i++
						closed = true
						break
					}
					i++
				}
				if !closed {
					return nil, fmt.Errorf("unterminated block comment")
				}
				continue
			}
		}
		out = append(out, c)
	}
	return out, nil
}

func stripTrailingCommas(data []byte) []byte {
	out := make([]byte, 0, len(data))
	inString := false
	escaped := false
	for i := 0; i < len(data); i++ {
		c := data[i]
		if inString {
			out = append(out, c)
			if escaped {
				escaped = false
				continue
			}
			switch c {
			case '\\':
				escaped = true
			case '"':
				inString = false
			}
			continue
		}

		if c == '"' {
			inString = true
			out = append(out, c)
			continue
		}
		if c == ',' {
			j := i + 1
			for j < len(data) {
				switch data[j] {
				case ' ', '\t', '\r', '\n':
					j++
				default:
					goto checked
				}
			}
		checked:
			if j < len(data) && (data[j] == '}' || data[j] == ']') {
				continue
			}
		}
		out = append(out, c)
	}
	return out
}

// FromYAML parses YAML bytes and converts them to a TEO document.
func FromYAML(data []byte, o *Options) (*teo.Document, error) {
	var v any
	if err := yaml.Unmarshal(data, &v); err != nil {
		return nil, fmt.Errorf("parse yaml: %w", err)
	}
	return FromValue(normalize(v), o)
}

// FromValue converts an already-decoded value (the result of unmarshalling
// into any) to a TEO document. Numbers should be normalized first; callers
// using FromJSON/FromYAML get this automatically.
func FromValue(v any, o *Options) (*teo.Document, error) {
	d := teo.New()
	switch t := v.(type) {
	case map[string]any:
		emitObject(d, t)
	case []any:
		emitArray(d, o.rootName(), t)
	default:
		d.Scalar("value", scalarOrJSON(v))
	}
	return d, nil
}

// emitObject renders each key of a root-level object. Keys are emitted in
// sorted order for deterministic output.
func emitObject(d *teo.Document, m map[string]any) {
	for _, k := range sortedKeys(m) {
		emitValue(d, sanitizeKey(k), m[k])
	}
}

func emitValue(d *teo.Document, name string, val any) {
	switch t := val.(type) {
	case []any:
		emitArray(d, name, t)
	case map[string]any:
		emitNestedObject(d, name, t)
	default:
		d.Scalar(name, scalarOrJSON(val))
	}
}

func emitNestedObject(d *teo.Document, name string, m map[string]any) {
	var kvs []teo.KV
	for _, k := range sortedKeys(m) {
		if isScalar(m[k]) {
			kvs = append(kvs, teo.KV{Key: sanitizeKey(k), Value: m[k]})
		}
	}
	if len(kvs) > 0 || len(m) == 0 {
		d.Record(name, kvs...)
	}
	for _, k := range sortedKeys(m) {
		if !isScalar(m[k]) {
			childName := sanitizeKey(name + "_" + k)
			if k == "rows" {
				if fields := fieldsFromColdesc(m["coldesc"]); len(fields) > 0 && emitArrayWithFields(d, childName, m[k], fields) {
					continue
				}
			}
			emitValue(d, childName, m[k])
		}
	}
}

// emitArray renders an array as a block. Arrays of objects become a
// multi-column block keyed by the union of element keys; anything else becomes
// a single-column "value" block.
func emitArray(d *teo.Document, name string, arr []any) {
	if objs, ok := allObjects(arr); ok {
		emitObjectBlock(d, name, objs, unionKeys(objs))
		return
	}
	bh := d.Block(name, "value")
	for _, el := range arr {
		bh.Row(scalarOrJSON(el))
	}
}

func emitArrayWithFields(d *teo.Document, name string, arr any, fields []string) bool {
	items, ok := arr.([]any)
	if !ok {
		return false
	}
	objs, ok := allObjects(items)
	if !ok {
		return false
	}
	emitObjectBlock(d, name, objs, appendMissingFields(fields, objs))
	return true
}

func emitObjectBlock(d *teo.Document, name string, objs []map[string]any, fields []string) {
	headers := make([]string, len(fields))
	for i, f := range fields {
		headers[i] = sanitizeField(f)
	}
	bh := d.Block(name, headers...)
	for _, obj := range objs {
		row := make([]any, len(fields))
		for i, f := range fields {
			if cell, present := obj[f]; present {
				row[i] = scalarOrJSON(cell)
			} else {
				row[i] = nil
			}
		}
		bh.Row(row...)
	}
}

func fieldsFromColdesc(v any) []string {
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	for _, el := range arr {
		m, ok := el.(map[string]any)
		if !ok {
			continue
		}
		name, ok := m["name"].(string)
		if ok && strings.TrimSpace(name) != "" && !seen[name] {
			seen[name] = true
			out = append(out, name)
		}
	}
	return out
}

func appendMissingFields(fields []string, objs []map[string]any) []string {
	seen := map[string]bool{}
	out := append([]string(nil), fields...)
	for _, f := range out {
		seen[f] = true
	}
	for _, f := range unionKeys(objs) {
		if !seen[f] {
			out = append(out, f)
		}
	}
	return out
}

// ----- value classification ------------------------------------------------------

func isScalar(v any) bool {
	switch v.(type) {
	case nil, bool, int, int64, float64, string:
		return true
	default:
		return false
	}
}

// allObjects reports whether arr is non-empty and every element is an object,
// returning the elements typed as maps.
func allObjects(arr []any) ([]map[string]any, bool) {
	if len(arr) == 0 {
		return nil, false
	}
	objs := make([]map[string]any, 0, len(arr))
	for _, el := range arr {
		m, ok := el.(map[string]any)
		if !ok {
			return nil, false
		}
		objs = append(objs, m)
	}
	return objs, true
}

// scalarOrJSON returns scalars unchanged and JSON-encodes everything else so a
// non-scalar cell still round-trips losslessly as a TEO string.
func scalarOrJSON(v any) any {
	if isScalar(v) {
		return v
	}
	return jsonString(v)
}

func jsonString(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprint(v)
	}
	return string(b)
}

// ----- key handling --------------------------------------------------------------

// unionKeys returns the union of keys across objects in sorted order. Source
// key order is unavailable: encoding/json and the YAML decoder both unmarshal
// objects into Go maps, which do not preserve insertion order.
func unionKeys(objs []map[string]any) []string {
	seen := map[string]bool{}
	var out []string
	for _, m := range objs {
		for _, k := range sortedKeys(m) {
			if !seen[k] {
				seen[k] = true
				out = append(out, k)
			}
		}
	}
	return out
}

func sortedKeys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// sanitizeField makes a block field name safe to emit inside `{f1,f2,...}`. The
// block grammar allows any text there except the field delimiters, so only the
// comma, closing brace, and newlines (which the parser uses to split fields and
// rows) are neutralized; everything else — camelCase, dots, hyphens — is kept.
func sanitizeField(f string) string {
	if !strings.ContainsAny(f, ",}\n\r") {
		return f
	}
	return strings.NewReplacer(",", "_", "}", "_", "\n", "_", "\r", "_").Replace(f)
}

// sanitizeKey projects an arbitrary key onto the TEO key grammar
// `[a-z][a-z0-9_]*`.
func sanitizeKey(k string) string {
	var b []rune
	runes := []rune(k)
	for i, r := range runes {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '_':
			b = append(b, r)
		case r >= 'A' && r <= 'Z':
			if len(b) > 0 && b[len(b)-1] != '_' && (isLowerOrDigit(runes[i-1]) || (isUpper(runes[i-1]) && i+1 < len(runes) && isLower(runes[i+1]))) {
				b = append(b, '_')
			}
			b = append(b, r+('a'-'A'))
		default:
			b = append(b, '_')
		}
	}
	if len(b) == 0 {
		return "k"
	}
	// The grammar requires a leading lowercase letter, so a key that starts
	// with a digit or underscore is prefixed with 'k' (e.g. "2nd" -> "k2nd").
	if !(b[0] >= 'a' && b[0] <= 'z') {
		b = append([]rune{'k'}, b...)
	}
	return string(b)
}

func isLower(r rune) bool { return r >= 'a' && r <= 'z' }

func isUpper(r rune) bool { return r >= 'A' && r <= 'Z' }

func isLowerOrDigit(r rune) bool {
	return isLower(r) || (r >= '0' && r <= '9')
}
