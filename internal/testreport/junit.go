package testreport

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// TokenMetric is one fixture's tiktoken comparison.
type TokenMetric struct {
	Fixture     string  `json:"fixture"`
	SourceTok   int     `json:"source_tokens"`
	TeoTok      int     `json:"teo_tokens"`
	Saved       int     `json:"saved"`
	SavedPct    float64 `json:"saved_pct"`
	MustSave    bool    `json:"must_save"`
	SavedTokens bool    `json:"saved_tokens"`
}

// TokenReport is written during tests and merged into JUnit XML after gotestsum.
type TokenReport struct {
	Encoding string        `json:"encoding"`
	Metrics  []TokenMetric `json:"metrics"`
}

const defaultMetricsRel = "test-results/token-benefit.json"

// MetricsPath returns the token report path, overridable via TEO_TOKEN_REPORT.
func MetricsPath() string {
	if p := os.Getenv("TEO_TOKEN_REPORT"); p != "" {
		return p
	}
	root, err := repoRoot()
	if err != nil {
		return defaultMetricsRel
	}
	return filepath.Join(root, defaultMetricsRel)
}

func defaultJUnitPath() string {
	root, err := repoRoot()
	if err != nil {
		return "test-results/junit.xml"
	}
	return filepath.Join(root, "test-results/junit.xml")
}

func repoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found")
		}
		dir = parent
	}
}

// WriteTokenReport persists metrics for post-test JUnit embedding.
func WriteTokenReport(report TokenReport) error {
	path := MetricsPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

// FormatTable renders metrics as plain text for JUnit system-out.
func FormatTable(report TokenReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "TEO token benefit (tiktoken %s)\n", report.Encoding)
	fmt.Fprintf(&b, "%-28s %14s %11s %7s %6s\n", "fixture", "source_tokens", "teo_tokens", "saved", "pct")
	fmt.Fprintln(&b, strings.Repeat("-", 80))
	for _, m := range report.Metrics {
		fmt.Fprintf(&b, "%-28s %14d %11d %7d %5.1f%%", m.Fixture, m.SourceTok, m.TeoTok, m.Saved, m.SavedPct)
		if !m.SavedTokens {
			fmt.Fprint(&b, "  (no savings)")
		}
		fmt.Fprintln(&b)
	}
	return b.String()
}

// EmbedTokenReportInJUnit adds token metrics to test-results/junit.xml when both files exist.
func EmbedTokenReportInJUnit(junitPath string) error {
	if junitPath == "" {
		junitPath = defaultJUnitPath()
	}
	metricsPath := MetricsPath()
	metricsBytes, err := os.ReadFile(metricsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	junitBytes, err := os.ReadFile(junitPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var report TokenReport
	if err := json.Unmarshal(metricsBytes, &report); err != nil {
		return err
	}

	table := FormatTable(report)
	out := `<system-out><![CDATA[` + table + `]]></system-out>`

	needle := `name="TestTokenBenefitReport" time="`
	idx := strings.Index(string(junitBytes), needle)
	if idx < 0 {
		needle = `name="TestTokenBenefitReport"`
		idx = strings.Index(string(junitBytes), needle)
		if idx < 0 {
			return fmt.Errorf("junit: TestTokenBenefitReport testcase not found")
		}
	}
	close := strings.Index(string(junitBytes[idx:]), "</testcase>")
	if close < 0 {
		return fmt.Errorf("junit: testcase close tag not found")
	}
	insertAt := idx + close
	updated := string(junitBytes[:insertAt]) + out + string(junitBytes[insertAt:])

	// Per-fixture properties on subtest cases.
	for _, m := range report.Metrics {
		sub := fmt.Sprintf(`name="TestTokenBenefitReport/%s"`, m.Fixture)
		subIdx := strings.Index(updated, sub)
		if subIdx < 0 {
			continue
		}
		subClose := strings.Index(updated[subIdx:], "</testcase>")
		if subClose < 0 {
			continue
		}
		props := fmt.Sprintf(
			`<properties><property name="source_tokens" value="%d"></property><property name="teo_tokens" value="%d"></property><property name="saved_tokens" value="%d"></property><property name="saved_pct" value="%.1f"></property></properties>`,
			m.SourceTok, m.TeoTok, m.Saved, m.SavedPct,
		)
		insert := subIdx + subClose
		updated = updated[:insert] + props + updated[insert:]
	}

	suiteProps := fmt.Sprintf(
		`<property name="teo.token.encoding" value="%s"></property><property name="teo.token.fixtures" value="%d"></property>`,
		report.Encoding, len(report.Metrics),
	)
	updated = strings.Replace(updated, `<properties>`, `<properties>`+suiteProps, 1)

	return os.WriteFile(junitPath, []byte(updated), 0o644)
}
