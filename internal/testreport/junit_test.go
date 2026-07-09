package testreport_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cloud-byte-consulting/teo/internal/testreport"
)

func TestEmbedTokenReportInJUnit(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TEO_TOKEN_REPORT", filepath.Join(dir, "token-benefit.json"))

	report := testreport.TokenReport{
		Encoding: "cl100k_base",
		Metrics: []testreport.TokenMetric{{
			Fixture: "schema_misc_examples.json", SourceTok: 356, TeoTok: 266,
			Saved: 90, SavedPct: 25.3, SavedTokens: true,
		}},
	}
	if err := testreport.WriteTokenReport(report); err != nil {
		t.Fatal(err)
	}

	junitPath := filepath.Join(dir, "junit.xml")
	junit := `<?xml version="1.0" encoding="UTF-8"?>
<testsuites tests="2">
  <testsuite tests="2" name="convert">
    <properties></properties>
    <testcase classname="convert" name="TestTokenBenefitReport/schema_misc_examples.json" time="0.01"></testcase>
    <testcase classname="convert" name="TestTokenBenefitReport" time="0.02"></testcase>
  </testsuite>
</testsuites>`
	if err := os.WriteFile(junitPath, []byte(junit), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := testreport.EmbedTokenReportInJUnit(junitPath); err != nil {
		t.Fatal(err)
	}
	out, err := os.ReadFile(junitPath)
	if err != nil {
		t.Fatal(err)
	}
	text := string(out)
	for _, want := range []string{
		"<system-out><![CDATA[",
		"schema_misc_examples.json",
		"25.3%",
		`name="source_tokens" value="356"`,
		`name="teo.token.encoding" value="cl100k_base"`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in junit:\n%s", want, text)
		}
	}
}
