# teo — Token-Efficient Output

A line-oriented, indentation-structured output format that declares repeated
structure once and drops JSON's per-value punctuation. This repo is the
canonical home of the TEO **library**, **converter**, and **CLI**; see
[`teo-format.md`](teo-format.md) for the grammar.

```
count: 3
issues[3]{number,title,state,author}:
  42,Fix login bug,open,alice
  43,"Add dark mode, finally",open,bob
  44,Crash on empty input,open,null
```

## Library (`truenas-scale-1.tail5a208d.ts.net/Cloud-Byte-Consulting/teo`)

Dependency-free builder + parser + validator. `parse(emit(x))` reconstructs `x`,
so it doubles as a round-trip test oracle.

```go
import "truenas-scale-1.tail5a208d.ts.net/Cloud-Byte-Consulting/teo"

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

## Converter (`truenas-scale-1.tail5a208d.ts.net/Cloud-Byte-Consulting/teo/convert`)

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

`github.com/cloud-byte/air` imports this module for its `--teo` output via a
normal versioned require on `truenas-scale-1.tail5a208d.ts.net/Cloud-Byte-Consulting/teo`,
resolved straight from the Cloud-Byte-Consulting Gitea over HTTPS (a valid
`go-import` meta tag is served on the default port). Consumers set
`GOPRIVATE=truenas-scale-1.tail5a208d.ts.net` so `go` fetches it directly,
skipping the public proxy and checksum database. No `replace` directive is
needed.

> The module path embeds the current Tailscale hostname; a stable custom domain
> (CNAME → the Gitea) can be swapped in later via a repo-wide find/replace.
