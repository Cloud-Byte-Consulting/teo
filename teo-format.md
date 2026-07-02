# TEO format grammar

The precise grammar for Token-Efficient Output. Read this before writing a formatter so every command emits a consistent, parseable shape. TEO is line-oriented and indentation-structured. It is designed to be dense for downstream tools to read while remaining trivial to parse and unambiguous.

## Contents
- Design goals
- Line types
- Scalar lines
- Block lines (lists / tables)
- Help blocks
- Quoting and escaping
- Nulls, booleans, numbers
- Metadata lines
- Empty states
- Truncation markers
- Nesting guidance
- Round-trip check
- Full worked example

## Design goals
1. Declare repeated structure (field names) once, not per row.
2. Drop punctuation that JSON repeats per value (braces, quotes-when-unneeded, commas-as-separators-only).
3. Stay unambiguous: a parser must reconstruct the data without guessing.
4. Keep it diffable and greppable so programs can pipe it through shell tools.

## Line types
A TEO document is a sequence of lines of three kinds:
- **Scalar line** — `key: value`
- **Block** — a header line `name[count]{field1,field2,...}:` followed by zero or more indented row lines.
- **Help block** — a header line `help[n]:` followed by `n` indented suggestion lines.

Indentation is two spaces per level. A line's membership in a block is determined by being indented exactly one level under the block header.

## Scalar lines
```
key: value
```
- One space after the colon.
- Keys are lowercase, `snake_case`, no spaces.
- Used for single-record fields and top-level metadata.
- A record (single object) is rendered as a labeled group: a `key:` header with no `[...]{...}`, followed by indented `key: value` lines.

```
pull_request:
  number: 51772
  state: merged
```

## Block lines (lists / tables)
The core token-saving structure. Header declares the count and the field schema once; each row is positional values in schema order.

```
name[count]{field1,field2,field3}:
  v1,v2,v3
  v1,v2,v3
```
- `name` — entity name, plural, `snake_case`.
- `count` — number of rows actually emitted in this block (not the grand total; the grand total goes on a separate `count:` metadata line).
- `{...}` — comma-separated field names, defining row order.
- Each row: values in the same order, comma-separated, one row per line, indented one level.
- Rows contain no field names — position is the binding.

## Help blocks
```
help[2]:
  Run `teo convert data.json`
  Run `teo validate output.teo`
```
- `n` in `help[n]` is the number of suggestion lines.
- Each suggestion is a concrete next command. Carry forward fixed flags (like `--name rows`); leave runtime values as `<placeholder>` rather than guessing them.

## Quoting and escaping
Values are bare by default. Quote with double quotes **only when necessary**, i.e. when the raw value contains any of:
- a comma `,`
- a double quote `"`
- a newline
- a leading or trailing space
- a colon-space `: ` at the start (would look like a scalar)

Rules:
- To quote, wrap the whole value in `"..."`.
- Inside a quoted value, escape `"` as `\"` and newline as `\n`.
- A value that is exactly empty renders as `""` (two quotes) to distinguish it from null.

Examples:
```
title: Fix login bug                 # no quoting needed
title: "Fix login, then logout"      # contains a comma
note: "He said \"hi\""               # contains quotes
```

## Nulls, booleans, numbers
- **null / absent** → the bare token `null` (unquoted). Distinct from `""` (empty string).
- **booleans** → `true` / `false` (unquoted, lowercase).
- **numbers** → bare, no thousands separators (`8771`, `3.14`, `-2`).
- A literal string that happens to equal `null`/`true`/`false`/a number must be quoted to disambiguate: `state: "true"`.

## Metadata lines
Emit these top-level scalar lines where relevant, before the main block:
- `count: <n>` — for lists, the grand total available. If paginated, `count: <emitted> of <total> total`.
- `bin: <path>` — optional; the executable's own path, home prefix rendered as `~` (useful for content-first home views).
- `description: <one sentence>` — optional; what this command/output represents.

A rolled-up status is just a scalar line whose value summarizes sub-resources:
```
checks: "27 passed, 0 failed, 10 skipped"
```

## Empty states
Never emit silent empty output. For an empty list, still emit the count and the (row-less) block header:
```
count: 0
issues[0]{number,title,state}:
help[1]:
  Run `teo convert issues.csv --name issues`
```
Optionally add a human-meaningful `note:` line (`note: no open issues`). The point is that `count: 0` is an explicit, unambiguous signal that the query succeeded and returned nothing.

## Truncation markers
When a free-text field is capped, append a parenthetical hint as part of the (quoted) value:
```
body: "The login flow breaks when... (truncated, 2847 chars total — use --full)"
```
- State the true total size.
- Name the exact escape hatch flag that returns the full content.
- Keep enough leading content to be useful for most tasks.

## Nesting guidance
Keep TEO flat. Deeply nested structures cost tokens and parsing ambiguity. Prefer:
- For a one-to-many relationship, emit the parent record, then a separate named block for the children.
- Reference related entities by id rather than inlining them.
- If a value is itself a small list (e.g. labels), join with a non-comma delimiter inside a quoted value to avoid colliding with row separators: `labels: "bug|P1"`. Document the delimiter once.

Avoid emitting JSON-in-TEO; if a field is irreducibly nested, that field is a candidate for a dedicated subcommand instead.

## Round-trip check
A correct TEO emitter satisfies: parse(emit(data)) reconstructs data (modulo intentionally truncated fields and intentionally dropped fields). A minimal parser:
1. Read lines; split into top-level scalars, blocks, and help blocks by header pattern and indentation.
2. For a block header `name[count]{f1,f2}:`, read the next `count` indented lines, split each on unquoted commas into values, zip with `[f1,f2]`.
3. Respect quoting when splitting (a comma inside `"..."` is data, not a separator).
4. Map `null`/`true`/`false`/numerics to their typed values; everything else is a string (unquote if quoted).

Use this as a test oracle: emit a known structure, parse it back, assert equality on the non-truncated fields.

## Full worked example
Input data: a paginated issue list (14 shown of 8771), plus next-step guidance.

```
count: 14 of 8771 total
description: open issues for acme/widgets
issues[14]{number,title,state,author}:
  42,Fix login bug,open,alice
  43,"Add dark mode, finally",open,bob
  44,Crash on empty input,open,null
  ...
help[2]:
  Run `teo convert next-page.json --name issues`
  Run `teo validate issues.teo`
```
Notes on this example:
- `count:` carries the grand total so consumers know the list is partial.
- Row 43's title is quoted because it contains a comma.
- Row 44's author is `null` (unassigned), distinct from an empty string.
- The `help[]` block carries forward concrete follow-up commands.
