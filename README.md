# teo — Token-Efficient Output

A line-oriented, indentation-structured output format that declares repeated
structure once and drops JSON's per-value punctuation. This repo is the
canonical home of the TEO **library**, **converter**, and **CLI**; see
[`teo-format.md`](teo-format.md) for the grammar.

## Part of the Governed Agentic Platform

teo is the **token/cost plane** of the Open Engine platform — a token-efficient
output format that shrinks LLM payloads. Start at
[the platform getting-started guide](https://github.com/Cloud-Byte-Consulting/agentic-harness/blob/main/GETTING_STARTED.md).
Sibling pillars:

- [agentic-harness](https://github.com/Cloud-Byte-Consulting/agentic-harness) — the governed `air` CLI; its `--teo` output mode consumes this module.
- [Cachy](https://github.com/Cloud-Byte-Consulting/Cachy) — caching that complements TEO on the same token/cost pillar.
- [token-dashboard](https://github.com/Cloud-Byte-Consulting/token-dashboard) — visualizes the token/cost savings TEO produces.

```
count: 3
issues[3]{number,title,state,author}:
  42,Fix login bug,open,alice
  43,"Add dark mode, finally",open,bob
  44,Crash on empty input,open,null
```

## Library (`github.com/cloud-byte-consulting/teo`)

Dependency-free builder + parser + validator. `parse(emit(x))` reconstructs `x`,
so it doubles as a round-trip test oracle.

```go
import "github.com/cloud-byte-consulting/teo"

doc := teo.New()
doc.Count(3)
doc.Block("issues", "number", "title", "state").
    Row(42, "Fix login bug", "open").
    Row(43, "Add dark mode, finally", "open")
fmt.Print(doc.String())

parsed, err := teo.Parse(out)   // strict: declared row counts must match
err = teo.Validate(out)         // well-formedness check
```

Builders: `Scalar`, `Count`, `Record`, `Block`/`Row`, `Help`. Accessors:
`FindBlock`, `GetScalar`. Value codecs: `EncodeValue` / `DecodeValue`.

## Converter (`github.com/cloud-byte-consulting/teo/convert`)

JSON/YAML → TEO. Kept in a sibling package so the core library stays
dependency-free (only the converter pulls in a YAML decoder).

```go
doc, err := convert.FromJSON(data, nil)
doc, err := convert.FromYAML(data, &convert.Options{RootName: "rows"})
```

**Projection policy** (TEO is two-level, so arbitrary JSON/YAML is mapped
deterministically):

| Input shape                         | TEO output                                   |
|-------------------------------------|----------------------------------------------|
| scalar value                        | `key: value`                                 |
| array of objects                    | block keyed by the union of keys (sorted)    |
| array of scalars / mixed            | single-column block `key[n]{value}`          |
| object of all-scalar fields         | record (`key:` + indented fields)            |
| object containing objects/arrays    | JSON-encoded onto one scalar line (lossless) |

Object/record/block **names** are sanitized to the key grammar
`[a-z][a-z0-9_]*` (lowercased; non-conforming runes → `_`; a `k` is prefixed
when the first rune is not a letter). Source object key order is **not**
preserved — Go map decoding drops it, so keys are emitted sorted.

## CLI (`cmd/teo`)

```
teo convert [--from auto|json|yaml] [--name NAME] [file]   # stdin if no file / "-"
teo validate [file]                                        # well-formedness check
teo version
```

```sh
go build ./cmd/teo
teo convert data.json | teo validate
echo '{"svc":"api","replicas":3}' | teo convert
```

`--from auto` (default) picks the format from the file extension, falling back
to content sniffing for stdin.

## Tests

```sh
go test ./...      # unit + integration + e2e
go test ./e2e      # e2e only (builds the binary, drives it as a subprocess)
```

- **Unit** — `teo_test.go` (library round-trips), `convert/convert_test.go`
  (projection policy; every conversion must re-parse).
- **Integration** — `internal/cli/cli_test.go` exercises the CLI in-process.
- **E2E** — `e2e/e2e_test.go` builds the real binary and drives it over
  argv/stdin/exit-codes, including `convert | validate`.

## Consumers

The [agentic-harness](https://github.com/Cloud-Byte-Consulting/agentic-harness)
`air` CLI imports this module for its `--teo` output via a normal versioned
require on `github.com/cloud-byte-consulting/teo` (see its `go.mod`). No
`replace` directive is needed.
