package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/cloud-byte-consulting/teo"
	"github.com/cloud-byte-consulting/teo/convert"
)

const hookUsage = `teo hook manages post-tool hooks for AI coding tools

usage:
  teo hook install [--provider claude|codex|copilot|gemini|opencode|cursor|all] [--scope project|user] [--force]
  teo hook run --provider claude|codex|copilot|gemini|opencode|cursor [--min-bytes N]
`

var hookProviders = []string{"claude", "codex", "copilot", "gemini", "opencode", "cursor"}

func runHook(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, hookUsage)
		return 2
	}
	switch args[0] {
	case "install":
		return runHookInstall(args[1:], stdout, stderr)
	case "run":
		return runHookRun(args[1:], stdin, stdout, stderr)
	case "-h", "--help", "help":
		fmt.Fprint(stdout, hookUsage)
		return 0
	default:
		fmt.Fprintf(stderr, "teo hook: unknown command %q\n\n%s", args[0], hookUsage)
		return 2
	}
}

func runHookInstall(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("hook install", flag.ContinueOnError)
	fs.SetOutput(stderr)
	provider := fs.String("provider", "all", "provider to install, or all")
	scope := fs.String("scope", "project", "install scope: project|user")
	force := fs.Bool("force", false, "overwrite an existing hook file")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() > 0 && *provider == "all" {
		*provider = fs.Arg(0)
	}

	providers, err := selectedHookProviders(*provider)
	if err != nil {
		fmt.Fprintf(stderr, "teo hook install: %v\n", err)
		return 1
	}
	for _, name := range providers {
		path, err := hookInstallPath(name, *scope)
		if err != nil {
			fmt.Fprintf(stderr, "teo hook install: %v\n", err)
			return 1
		}
		if err := writeHookFile(path, hookInstallContent(name), *force); err != nil {
			fmt.Fprintf(stderr, "teo hook install: %s: %v\n", name, err)
			return 1
		}
		fmt.Fprintf(stdout, "installed %s hook at %s\n", name, path)
	}
	return 0
}

func runHookRun(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("hook run", flag.ContinueOnError)
	fs.SetOutput(stderr)
	provider := fs.String("provider", "", "provider hook protocol")
	minBytes := fs.Int("min-bytes", 512, "minimum tool output bytes before conversion")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if !isHookProvider(*provider) {
		fmt.Fprintf(stderr, "teo hook run: unknown --provider %q\n", *provider)
		return 2
	}
	data, err := io.ReadAll(stdin)
	if err != nil {
		fmt.Fprintf(stderr, "teo hook run: read stdin: %v\n", err)
		return 1
	}
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		fmt.Fprintf(stderr, "teo hook run: parse hook json: %v\n", err)
		return 1
	}

	hit := extractHookText(*provider, payload)
	converted, ok := compactToolText(hit.text, *minBytes)
	if !ok {
		fmt.Fprintln(stdout, "{}")
		return 0
	}
	resp := hookProviderResponse(*provider, payload, hit, converted)
	if err := json.NewEncoder(stdout).Encode(resp); err != nil {
		fmt.Fprintf(stderr, "teo hook run: write response: %v\n", err)
		return 1
	}
	return 0
}

func selectedHookProviders(name string) ([]string, error) {
	if name == "all" || name == "" {
		return hookProviders, nil
	}
	if isHookProvider(name) {
		return []string{name}, nil
	}
	return nil, fmt.Errorf("unknown provider %q", name)
}

func isHookProvider(name string) bool {
	for _, provider := range hookProviders {
		if name == provider {
			return true
		}
	}
	return false
}

func hookInstallPath(provider, scope string) (string, error) {
	if scope != "project" && scope != "user" {
		return "", fmt.Errorf("unknown --scope %q (want project|user)", scope)
	}
	if scope == "project" {
		switch provider {
		case "claude":
			return filepath.Join(".claude", "settings.json"), nil
		case "codex":
			return filepath.Join(".codex", "hooks.json"), nil
		case "copilot":
			return filepath.Join(".github", "hooks", "teo-post-tool.json"), nil
		case "gemini":
			return filepath.Join(".gemini", "settings.json"), nil
		case "opencode":
			return filepath.Join(".opencode", "plugins", "teo-post-tool.js"), nil
		case "cursor":
			return filepath.Join(".cursor", "hooks.json"), nil
		}
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	switch provider {
	case "claude":
		return filepath.Join(home, ".claude", "settings.json"), nil
	case "codex":
		return filepath.Join(home, ".codex", "hooks.json"), nil
	case "copilot":
		return filepath.Join(home, ".copilot", "hooks", "teo-post-tool.json"), nil
	case "gemini":
		return filepath.Join(home, ".gemini", "settings.json"), nil
	case "opencode":
		return filepath.Join(home, ".config", "opencode", "plugins", "teo-post-tool.js"), nil
	case "cursor":
		return filepath.Join(home, ".cursor", "hooks.json"), nil
	}
	return "", fmt.Errorf("unknown provider %q", provider)
}

func writeHookFile(path string, content []byte, force bool) error {
	if !force {
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("file exists; pass --force to replace it")
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, content, 0o644)
}

func hookInstallContent(provider string) []byte {
	if provider == "opencode" {
		return []byte(opencodePlugin)
	}
	return prettyJSON(hookInstallJSON(provider))
}

func hookInstallJSON(provider string) any {
	command := "teo hook run --provider " + provider
	switch provider {
	case "claude":
		return map[string]any{
			"hooks": map[string]any{
				"PostToolUse": []any{map[string]any{
					"matcher": "*",
					"hooks": []any{map[string]any{
						"type":    "command",
						"command": command,
						"timeout": 30,
					}},
				}},
			},
		}
	case "codex":
		return map[string]any{
			"hooks": map[string]any{
				"PostToolUse": []any{map[string]any{
					"matcher": "*",
					"hooks": []any{map[string]any{
						"type":          "command",
						"command":       command,
						"timeout":       30,
						"statusMessage": "Compacting tool output with TEO",
					}},
				}},
			},
		}
	case "copilot":
		return map[string]any{
			"version": 1,
			"hooks": map[string]any{
				"postToolUse": []any{map[string]any{
					"type":       "command",
					"command":    command,
					"timeoutSec": 30,
				}},
			},
		}
	case "gemini":
		return map[string]any{
			"hooks": map[string]any{
				"AfterTool": []any{map[string]any{
					"matcher": "*",
					"hooks": []any{map[string]any{
						"type":        "command",
						"command":     command,
						"name":        "teo-post-tool",
						"timeout":     30000,
						"description": "Convert large structured tool output to TEO",
					}},
				}},
			},
		}
	case "cursor":
		return map[string]any{
			"version": 1,
			"hooks": map[string]any{
				"postToolUse": []any{map[string]any{
					"command": command,
					"timeout": 30,
				}},
			},
		}
	default:
		return map[string]any{}
	}
}

func prettyJSON(v any) []byte {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return []byte("{}\n")
	}
	return append(data, '\n')
}

const opencodePlugin = `import { spawnSync } from "node:child_process"

function pickText(output) {
  if (typeof output === "string") return output
  if (!output || typeof output !== "object") return ""
  for (const key of ["output", "text", "content", "message"]) {
    if (typeof output[key] === "string") return output[key]
  }
  return ""
}

function setText(output, text) {
  if (!output || typeof output !== "object") return
  for (const key of ["output", "text", "content", "message"]) {
    if (typeof output[key] === "string") {
      output[key] = text
      return
    }
  }
}

export const TeoPostTool = async () => ({
  "tool.execute.after": async (input, output) => {
    const text = pickText(output)
    if (!text) return

    const child = spawnSync("teo", ["hook", "run", "--provider", "opencode"], {
      input: JSON.stringify({ tool_name: input?.tool, tool_response: text }),
      encoding: "utf8",
    })
    if (child.status !== 0 || !child.stdout.trim()) return

    try {
      const parsed = JSON.parse(child.stdout)
      if (typeof parsed.text === "string") setText(output, parsed.text)
    } catch {
      return
    }
  },
})
`

type hookText struct {
	text   string
	topKey string
}

func extractHookText(provider string, payload map[string]any) hookText {
	switch provider {
	case "copilot":
		if text, ok := stringAt(payload, "toolResult", "textResultForLlm"); ok {
			return hookText{text: text}
		}
		if text, ok := stringAt(payload, "tool_result", "text_result_for_llm"); ok {
			return hookText{text: text}
		}
	case "cursor":
		for _, key := range []string{"tool_output", "toolOutput", "tool_response", "toolResponse"} {
			if text, ok := textValue(payload[key]); ok {
				return hookText{text: text, topKey: key}
			}
		}
	case "gemini":
		if text, ok := stringAt(payload, "tool_response", "llmContent"); ok {
			return hookText{text: text}
		}
		if text, ok := stringAt(payload, "toolResponse", "llmContent"); ok {
			return hookText{text: text}
		}
	}

	for _, key := range []string{"tool_response", "toolResponse", "tool_result", "toolResult"} {
		if text, ok := textValue(payload[key]); ok {
			return hookText{text: text, topKey: key}
		}
	}
	return hookText{}
}

func hookProviderResponse(provider string, payload map[string]any, hit hookText, text string) map[string]any {
	switch provider {
	case "claude":
		return map[string]any{
			"updatedToolOutput": updatedToolOutput(payload, hit, text),
		}
	case "codex":
		return map[string]any{
			"decision": "block",
			"reason":   text,
		}
	case "copilot":
		return map[string]any{
			"modifiedResult": map[string]any{
				"resultType":       "success",
				"textResultForLlm": text,
			},
		}
	case "gemini":
		return map[string]any{
			"decision": "deny",
			"reason":   text,
		}
	case "opencode":
		return map[string]any{"text": text}
	case "cursor":
		return map[string]any{
			"modified_result": map[string]any{
				"result_type":         "success",
				"text_result_for_llm": text,
			},
			"additional_context": "Tool output was converted to TEO.",
		}
	default:
		return map[string]any{}
	}
}

func updatedToolOutput(payload map[string]any, hit hookText, text string) any {
	if hit.topKey == "" {
		return text
	}
	resp, ok := payload[hit.topKey].(map[string]any)
	if !ok {
		return text
	}
	clone := cloneMap(resp)
	for _, key := range []string{"stdout", "llmContent", "returnDisplay", "output", "text", "content", "message"} {
		if _, ok := clone[key].(string); ok {
			clone[key] = text
			return clone
		}
	}
	return text
}

func compactToolText(text string, minBytes int) (string, bool) {
	trimmed := strings.TrimSpace(text)
	if len(trimmed) < minBytes {
		return "", false
	}
	if teo.Validate(trimmed) == nil {
		return "", false
	}

	type candidate struct {
		enabled bool
		convert func([]byte) (*teo.Document, error)
	}
	data := []byte(trimmed)
	first := byte(0)
	for i := 0; i < len(data); i++ {
		if !isSpace(data[i]) {
			first = data[i]
			break
		}
	}
	candidates := []candidate{
		{enabled: looksLikeJSONLines(data), convert: func(b []byte) (*teo.Document, error) {
			return convert.FromNDJSON(b, &convert.Options{RootName: "items"})
		}},
		{enabled: first == '{' || first == '[', convert: func(b []byte) (*teo.Document, error) {
			return convert.FromJSON(b, &convert.Options{RootName: "items"})
		}},
		{enabled: first == '{' || first == '[', convert: func(b []byte) (*teo.Document, error) {
			return convert.FromJSONC(b, &convert.Options{RootName: "items"})
		}},
		{enabled: first != '{' && first != '[' && hasRowsAndDelimiter(trimmed, ","), convert: func(b []byte) (*teo.Document, error) {
			return convert.FromCSV(b, &convert.Options{RootName: "items"})
		}},
		{enabled: first != '{' && first != '[' && hasRowsAndDelimiter(trimmed, "\t"), convert: func(b []byte) (*teo.Document, error) {
			return convert.FromTSV(b, &convert.Options{RootName: "items"})
		}},
		{enabled: strings.Contains(trimmed, ":\n") || strings.Contains(trimmed, ":\r\n"), convert: func(b []byte) (*teo.Document, error) {
			return convert.FromYAML(b, &convert.Options{RootName: "items"})
		}},
	}

	for _, c := range candidates {
		if !c.enabled {
			continue
		}
		doc, err := c.convert(data)
		if err != nil {
			continue
		}
		out := strings.TrimRight(doc.String(), "\n")
		if teo.Validate(out) == nil && len(out) < len(trimmed) {
			return out, true
		}
	}
	return "", false
}

func hasRowsAndDelimiter(text, delimiter string) bool {
	return strings.Count(text, "\n") > 0 && strings.Contains(text, delimiter)
}

func isSpace(b byte) bool {
	return b == ' ' || b == '\n' || b == '\r' || b == '\t'
}

func textValue(v any) (string, bool) {
	switch t := v.(type) {
	case string:
		return t, strings.TrimSpace(t) != ""
	case map[string]any:
		for _, key := range []string{"stdout", "llmContent", "returnDisplay", "output", "text", "content", "message"} {
			if text, ok := t[key].(string); ok && strings.TrimSpace(text) != "" {
				return text, true
			}
		}
	}
	return "", false
}

func stringAt(m map[string]any, keys ...string) (string, bool) {
	var cur any = m
	for _, key := range keys {
		next, ok := cur.(map[string]any)
		if !ok {
			return "", false
		}
		cur = next[key]
	}
	text, ok := cur.(string)
	return text, ok && strings.TrimSpace(text) != ""
}

func cloneMap(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}
