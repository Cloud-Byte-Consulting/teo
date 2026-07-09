package convert_test

import (
	"fmt"
	"os"
	"sync"
	"testing"

	"github.com/cloud-byte-consulting/teo"
	"github.com/cloud-byte-consulting/teo/convert"
	"github.com/cloud-byte-consulting/teo/internal/testreport"
	"github.com/cloud-byte-consulting/teo/internal/tokencount"
)

var tokenReportMu sync.Mutex

func TestTokenBenefitReport(t *testing.T) {
	type caseDef struct {
		fixture  string
		mustSave bool
		convert  func([]byte) (string, error)
	}

	cases := []caseDef{
		{
			fixture:  "schema_misc_examples.json",
			mustSave: true,
			convert: func(data []byte) (string, error) {
				doc, err := convert.FromJSON(data, nil)
				if err != nil {
					return "", err
				}
				return doc.String(), nil
			},
		},
		{
			fixture:  "schema_misc_examples.yaml",
			mustSave: false,
			convert: func(data []byte) (string, error) {
				doc, err := convert.FromYAML(data, nil)
				if err != nil {
					return "", err
				}
				return doc.String(), nil
			},
		},
		{
			fixture:  "schema_misc_examples.jsonl",
			mustSave: false,
			convert: func(data []byte) (string, error) {
				doc, err := convert.FromNDJSON(data, &convert.Options{RootName: "items"})
				if err != nil {
					return "", err
				}
				return doc.String(), nil
			},
		},
		{
			fixture:  "sample.json",
			mustSave: true,
			convert: func(data []byte) (string, error) {
				doc, err := convert.FromJSON(data, nil)
				if err != nil {
					return "", err
				}
				return doc.String(), nil
			},
		},
		{
			fixture:  "issues.json",
			mustSave: true,
			convert: func(data []byte) (string, error) {
				doc, err := convert.FromJSON(data, nil)
				if err != nil {
					return "", err
				}
				return doc.String(), nil
			},
		},
	}

	var metrics []testreport.TokenMetric
	t.Cleanup(func() {
		tokenReportMu.Lock()
		defer tokenReportMu.Unlock()
		if err := testreport.WriteTokenReport(testreport.TokenReport{
			Encoding: tokencount.Encoding,
			Metrics:  metrics,
		}); err != nil {
			t.Errorf("write token report: %v", err)
		}
	})

	t.Logf("TEO token benefit (tiktoken %s)", tokencount.Encoding)
	t.Logf("%-28s %14s %11s %7s %6s", "fixture", "source_tokens", "teo_tokens", "saved", "pct")
	t.Log("--------------------------------------------------------------------------------")

	for _, c := range cases {
		c := c
		t.Run(c.fixture, func(t *testing.T) {
			path := fmt.Sprintf("../testdata/%s", c.fixture)
			source, err := os.ReadFile(path)
			noerr(t, err)

			teoText, err := c.convert(source)
			noerr(t, err)
			noerr(t, teo.Validate(teoText))

			srcTok, err := tokencount.Count(string(source))
			noerr(t, err)
			teoTok, err := tokencount.Count(teoText)
			noerr(t, err)

			saved := srcTok - teoTok
			pct := 0.0
			if srcTok > 0 {
				pct = float64(saved) * 100 / float64(srcTok)
			}

			m := testreport.TokenMetric{
				Fixture:     c.fixture,
				SourceTok:   srcTok,
				TeoTok:      teoTok,
				Saved:       saved,
				SavedPct:    pct,
				MustSave:    c.mustSave,
				SavedTokens: teoTok < srcTok,
			}
			tokenReportMu.Lock()
			metrics = append(metrics, m)
			tokenReportMu.Unlock()

			line := fmt.Sprintf("%-28s %14d %11d %7d %5.1f%%", c.fixture, srcTok, teoTok, saved, pct)
			t.Log(line)

			if c.mustSave && teoTok >= srcTok {
				t.Fatalf("TEO tokens (%d) should be fewer than source (%d)", teoTok, srcTok)
			}
			if teoTok >= srcTok {
				t.Logf("note: no token savings for %s (expected for some formats)", c.fixture)
			}
		})
	}
}
