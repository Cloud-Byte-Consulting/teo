package convert

import (
	"encoding/json"
	"fmt"
)

// normalize coerces decoder-specific number/map representations into the small
// set TEO understands: int, float64, bool, string, nil, map[string]any, []any.
// JSON decoded with UseNumber yields json.Number; some YAML inputs yield
// map[any]any. Both are rewritten here so downstream code only sees normalized
// shapes.
func normalize(v any) any {
	switch t := v.(type) {
	case json.Number:
		if i, err := t.Int64(); err == nil {
			return int(i)
		}
		if f, err := t.Float64(); err == nil {
			return f
		}
		return t.String()
	case map[string]any:
		for k, val := range t {
			t[k] = normalize(val)
		}
		return t
	case map[any]any:
		m := make(map[string]any, len(t))
		for k, val := range t {
			m[fmt.Sprint(k)] = normalize(val)
		}
		return m
	case []any:
		for i, val := range t {
			t[i] = normalize(val)
		}
		return t
	default:
		return v
	}
}
