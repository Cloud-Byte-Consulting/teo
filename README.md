# teo — Token-Efficient Output

A line-oriented, indentation-structured output format that declares repeated
structure once and drops JSON's per-value punctuation. This repo is the
canonical home of the TEO **library**, **converter**, and **CLI**; see
[`teo-format.md`](teo-format.md) for the grammar.

## Purpose

teo is a tool-agnostic conversion layer for standard machine-readable outputs.
It accepts common structured and tabular inputs, then emits the same data in a
compact TEO shape that is easy to diff, grep, validate, and pass through other
programs.

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

Standard inputs -> TEO. Kept in a sibling package so the core library stays
dependency-free.

```go
doc, err := convert.FromJSON(data, nil)
doc, err := convert.FromYAML(data, &convert.Options{RootName: "rows"})
doc, err := convert.FromJSONC(data, nil)
doc, err := convert.FromNDJSON(data, &convert.Options{RootName: "events"})
doc, err := convert.FromCSV(data, &convert.Options{RootName: "rows"})
doc, err := convert.FromTSV(data, &convert.Options{RootName: "rows", NoHeader: true})
```

Supported input formats:

- JSON (`.json`)
- YAML (`.yaml`, `.yml`)
- JSONC (`.jsonc`)
- NDJSON / JSON Lines (`.ndjson`, `.jsonl`)
- CSV (`.csv`)
- TSV (`.tsv`)

**Projection policy** (TEO is two-level, so arbitrary input is mapped
deterministically):

| Input shape                         | TEO output                                   |
|-------------------------------------|----------------------------------------------|
| scalar value                        | `key: value`                                 |
| array of objects                    | block keyed by the union of keys (sorted)    |
| array of scalars / mixed            | single-column block `key[n]{value}`          |
| object of all-scalar fields         | record (`key:` + indented fields)            |
| object containing objects/arrays    | JSON-encoded onto one scalar line (lossless) |
| NDJSON / JSON Lines                 | root block from one JSON value per line      |
| CSV / TSV with headers              | one block using the first row as fields      |
| CSV / TSV without headers           | one block using `col1`, `col2`, etc.         |

Object/record/block **names** are sanitized to the key grammar
`[a-z][a-z0-9_]*` (lowercased; non-conforming runes → `_`; a `k` is prefixed
when the first rune is not a letter). Source object key order is **not**
preserved — Go map decoding drops it, so keys are emitted sorted.

CSV and TSV values are preserved as strings because those formats do not carry
native type information. By default, the first row is treated as a header row;
set `Options.NoHeader` or pass `--no-header` in the CLI to generate `col1`,
`col2`, etc.

## CLI (`cmd/teo`)

```
teo convert [--from auto|json|yaml|jsonc|csv|tsv|ndjson|jsonl] [--name NAME] [--no-header] [file]
teo validate [file]   # well-formedness check
teo hook install [--provider claude|codex|copilot|gemini|opencode|cursor|all]
teo hook run --provider claude|codex|copilot|gemini|opencode|cursor
teo version
```

```sh
go build ./cmd/teo
teo convert data.json | teo validate
teo convert data.csv --name rows
teo convert --from csv --no-header --name rows < data.txt
echo '{"svc":"api","replicas":3}' | teo convert
```

`--from auto` (default) picks the format from the file extension. Stdin is
sniffed for JSON and NDJSON, then treated as YAML; use `--from csv` or
`--from tsv` for delimited stdin.

## Agent Hooks

TEO can install post-tool hooks for Claude Code, Codex, GitHub Copilot CLI,
Gemini CLI, OpenCode CLI, and Cursor. The hook converts large structured tool
output to TEO before the next model call when doing so makes the output smaller.

```sh
teo hook install --provider all
```

See [`docs/hooks.md`](docs/hooks.md) for provider paths, caveats, and manual
setup details.

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
- **CI** — `.gitea/workflows/test.yml` runs `go test ./...`.
