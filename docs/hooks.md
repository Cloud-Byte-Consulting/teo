# Agent Hook Install

TEO works best as a post-tool hook. The tool has already run, so the hook can
replace large JSON, YAML, CSV, TSV, or NDJSON output before the next model call
sees it. A pre-tool hook cannot do that because no tool output exists yet.

Install the CLI first:

```sh
go install github.com/cloud-byte-consulting/teo/cmd/teo@latest
```

For a checkout of this repo:

```sh
go install ./cmd/teo
```

Make sure `teo` is on `PATH`, then install provider hooks:

```sh
teo hook install --provider all
```

To install one provider:

```sh
teo hook install --provider claude
teo hook install --provider codex
teo hook install --provider copilot
teo hook install --provider gemini
teo hook install --provider opencode
teo hook install --provider cursor
```

By default, files are written into the current project. Use `--scope user` for
a user-level install where the provider supports user hook files:

```sh
teo hook install --provider codex --scope user
```

Existing files are not overwritten. Pass `--force` only when you have checked
the current hook file and want TEO to replace it.

## Installed Files

| Provider | Project file | Hook event | Output path |
| --- | --- | --- | --- |
| Claude Code | `.claude/settings.json` | `PostToolUse` | `updatedToolOutput` |
| Codex | `.codex/hooks.json` | `PostToolUse` | replacement feedback |
| GitHub Copilot CLI | `.github/hooks/teo-post-tool.json` | `postToolUse` | `modifiedResult.textResultForLlm` |
| Gemini CLI | `.gemini/settings.json` | `AfterTool` | `decision: deny`, `reason` |
| OpenCode CLI | `.opencode/plugins/teo-post-tool.js` | `tool.execute.after` | plugin mutates output text |
| Cursor | `.cursor/hooks.json` | `postToolUse` | best effort replacement payload |

The Cursor hook API is still moving. The installed hook emits Cursor-style
snake-case fields and keeps the original output when Cursor does not accept a
replacement.

## Hook Runner

Provider hooks call:

```sh
teo hook run --provider PROVIDER
```

The runner reads the provider hook JSON from stdin, finds the model-visible tool
output, and tries to convert it to TEO. It only returns a replacement when all
of these are true:

- the output is at least 512 bytes, unless `--min-bytes` is set
- the output looks like JSON, YAML, JSONC, NDJSON, CSV, or TSV
- conversion produces valid TEO
- the TEO text is smaller than the original output

Otherwise it prints `{}` and the provider keeps the original tool result.

## Provider Notes

Claude Code supports `PostToolUse` and `updatedToolOutput`, so Bash `stdout`
and similar text fields can be replaced directly.

Codex supports `PostToolUse` hooks from `.codex/hooks.json`. For post-tool
replacement, TEO returns a block decision with the converted output as feedback,
which Codex uses instead of the original tool result.

GitHub Copilot CLI reads hooks from `.github/hooks/*.json`. Its `postToolUse`
hook supports `modifiedResult.textResultForLlm`, which is the cleanest
replacement path for TEO.

Gemini CLI `AfterTool` hooks can hide the original result by returning
`decision: "deny"` with `reason`. TEO uses that replacement path only when the
converted output is smaller.

OpenCode uses plugins instead of a JSON hook file. TEO installs a tiny local
plugin that listens for `tool.execute.after`, runs `teo hook run`, and mutates
the output text object when OpenCode gives the plugin a mutable result.

Cursor project hooks live in `.cursor/hooks.json`. Current Cursor builds vary
in which post-tool fields are honored, so this integration is intentionally
best effort.

## References

- Claude Code hooks: <https://docs.anthropic.com/en/docs/claude-code/hooks>
- Codex hooks: <https://developers.openai.com/codex/hooks>
- GitHub Copilot hooks: <https://docs.github.com/en/copilot/reference/hooks-reference>
- Gemini CLI hooks: <https://geminicli.com/docs/hooks/reference/>
- OpenCode plugins: <https://opencode.ai/docs/plugins/>
- Cursor hooks: <https://cursor.com/docs/hooks>
