// Package cli implements the `teo` command's behavior behind an in-process
// Run function so it can be exercised by integration tests without spawning a
// process. cmd/teo is a thin main that forwards os.Args to Run.
package cli

import (
	"bytes"
	"encoding/csv"
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

// Version is overridden at build time via -ldflags "-X .../cli.Version=...".
var Version = "dev"

const usage = `teo — Token-Efficient Output toolkit

usage:
  teo convert [--from auto|json|yaml|jsonc|csv|tsv|ndjson|jsonl] [--name NAME] [--no-header] [file]
                                                              convert standard input formats to TEO (stdin if no file)
  teo validate [file]                                         validate that input is well-formed TEO (stdin if no file)
  teo version                                                 print version
`

// Run dispatches a teo subcommand. It returns a process exit code and never
// calls os.Exit, so tests can assert on the code and captured output.
func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	switch args[0] {
	case "convert":
		return runConvert(args[1:], stdin, stdout, stderr)
	case "validate":
		return runValidate(args[1:], stdin, stdout, stderr)
	case "version":
		fmt.Fprintln(stdout, Version)
		return 0
	case "-h", "--help", "help":
		fmt.Fprint(stdout, usage)
		return 0
	default:
		fmt.Fprintf(stderr, "teo: unknown command %q\n\n%s", args[0], usage)
		return 2
	}
}

func runConvert(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("convert", flag.ContinueOnError)
	fs.SetOutput(stderr)
	from := fs.String("from", "auto", "input format: auto|json|yaml|jsonc|csv|tsv|ndjson|jsonl")
	name := fs.String("name", "items", "block name to use when the root is an array")
	noHeader := fs.Bool("no-header", false, "treat CSV/TSV rows as data and generate col1, col2, etc.")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	path, data, err := readInput(fs.Arg(0), stdin)
	if err != nil {
		fmt.Fprintf(stderr, "teo convert: %v\n", err)
		return 1
	}

	format, err := resolveFormat(*from, path, data)
	if err != nil {
		fmt.Fprintf(stderr, "teo convert: %v\n", err)
		return 1
	}

	opts := &convert.Options{RootName: *name, NoHeader: *noHeader}
	var doc *teo.Document
	switch format {
	case "json":
		doc, err = convert.FromJSON(data, opts)
	case "yaml":
		doc, err = convert.FromYAML(data, opts)
	case "jsonc":
		doc, err = convert.FromJSONC(data, opts)
	case "csv":
		doc, err = convert.FromCSV(data, opts)
	case "tsv":
		doc, err = convert.FromTSV(data, opts)
	case "ndjson", "jsonl":
		doc, err = convert.FromNDJSON(data, opts)
	}
	if err != nil {
		fmt.Fprintf(stderr, "teo convert: %v\n", err)
		return 1
	}
	fmt.Fprint(stdout, doc.String())
	return 0
}

func runValidate(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	fs.SetOutput(stderr)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	_, data, err := readInput(fs.Arg(0), stdin)
	if err != nil {
		fmt.Fprintf(stderr, "teo validate: %v\n", err)
		return 1
	}
	if err := teo.Validate(string(data)); err != nil {
		fmt.Fprintf(stderr, "teo validate: invalid TEO: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, "ok")
	return 0
}

// readInput returns the path (empty for stdin) and bytes for a file argument or
// stdin when the argument is absent or "-".
func readInput(arg string, stdin io.Reader) (string, []byte, error) {
	if arg == "" || arg == "-" {
		data, err := io.ReadAll(stdin)
		if err != nil {
			return "", nil, fmt.Errorf("read stdin: %w", err)
		}
		return "", data, nil
	}
	data, err := os.ReadFile(arg)
	if err != nil {
		return "", nil, err
	}
	return arg, data, nil
}

// resolveFormat picks a format from the explicit flag, the file extension, or
// content sniffing as a last resort.
func resolveFormat(from, path string, data []byte) (string, error) {
	switch from {
	case "json", "yaml", "jsonc", "csv", "tsv", "ndjson", "jsonl":
		return from, nil
	case "auto", "":
		switch strings.ToLower(filepath.Ext(path)) {
		case ".json":
			return "json", nil
		case ".jsonc":
			return "jsonc", nil
		case ".yaml", ".yml":
			return "yaml", nil
		case ".csv":
			return "csv", nil
		case ".tsv":
			return "tsv", nil
		case ".ndjson":
			return "ndjson", nil
		case ".jsonl":
			return "jsonl", nil
		}
		// No usable extension (e.g. stdin): sniff common unambiguous shapes.
		if looksLikeJSONLines(data) {
			return "ndjson", nil
		}
		for _, b := range data {
			switch b {
			case ' ', '\t', '\r', '\n':
				continue
			case '{', '[':
				return "json", nil
			}
			break
		}
		if looksDelimited(data, '\t') {
			return "tsv", nil
		}
		if looksDelimited(data, ',') {
			return "csv", nil
		}
		return "yaml", nil
	default:
		return "", fmt.Errorf("unknown --from %q (want auto|json|yaml|jsonc|csv|tsv|ndjson|jsonl)", from)
	}
}

func looksLikeJSONLines(data []byte) bool {
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	nonblank := 0
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		nonblank++
		if line[0] != '{' && line[0] != '[' {
			return false
		}
		if !json.Valid([]byte(line)) {
			return false
		}
	}
	return nonblank > 1
}

func looksDelimited(data []byte, comma rune) bool {
	r := csv.NewReader(bytes.NewReader(data))
	r.Comma = comma
	records, err := r.ReadAll()
	return err == nil && len(records) > 1 && len(records[0]) > 1
}
