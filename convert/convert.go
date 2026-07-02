// Package convert turns JSON or YAML into a TEO document.
//
// TEO is two-level: top-level scalars, named blocks (arrays of uniform
// objects), and records (an object of scalar fields). Arbitrary JSON/YAML has
// no single canonical TEO projection, so this package applies a fixed,
// documented policy:
//
//   - Root object → each key emitted by value shape (below).
//   - Root array of objects → one block named by Options.RootName ("items").
//   - Root array of scalars → block RootName with a single "value" column.
//   - Root scalar → `value: <scalar>`.
//
// Per key inside an object:
//
//   - scalar (string/number/bool/null) → `key: value`.
//   - array of objects → block keyed by the union of element keys (sorted;
//     decoders drop source key order); missing fields are null; non-scalar
//     cells are JSON-encoded.
//   - array of scalars/mixed → block `key[n]{value}` with one column.
//   - object whose values are all scalars → record.
//   - object with nested objects/arrays → JSON-encoded onto a single scalar
//     line (depth beyond what TEO models is preserved losslessly as JSON text).
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
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, fmt.Errorf("parse json: %w", err)
	}
	return FromValue(normalize(v), o)
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
		val := m[k]
		name := sanitizeKey(k)
		switch t := val.(type) {
		case []any:
			emitArray(d, name, t)
		case map[string]any:
			if allScalar(t) {
				d.Record(name, recordKVs(t)...)
			} else {
				d.Scalar(name, jsonString(t))
			}
		default:
			d.Scalar(name, scalarOrJSON(val))
		}
	}
}

// emitArray renders an array as a block. Arrays of objects become a
// multi-column block keyed by the union of element keys; anything else becomes
// a single-column "value" block.
func emitArray(d *teo.Document, name string, arr []any) {
	if objs, ok := allObjects(arr); ok {
		fields := unionKeys(objs) // original keys, used for map lookup
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
		return
	}
	bh := d.Block(name, "value")
	for _, el := range arr {
		bh.Row(scalarOrJSON(el))
	}
}

func recordKVs(m map[string]any) []teo.KV {
	keys := sortedKeys(m)
	kvs := make([]teo.KV, 0, len(keys))
	for _, k := range keys {
		kvs = append(kvs, teo.KV{Key: sanitizeKey(k), Value: m[k]})
	}
	return kvs
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

func allScalar(m map[string]any) bool {
	for _, v := range m {
		if !isScalar(v) {
			return false
		}
	}
	return true
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
	for _, r := range k {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '_':
			b = append(b, r)
		case r >= 'A' && r <= 'Z':
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
