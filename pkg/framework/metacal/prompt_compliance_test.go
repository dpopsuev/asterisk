package metacal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"asterisk/pkg/framework"
)

type responseClass int

const (
	classFoundation responseClass = iota
	classWrapper
	classExcluded
	classNoIdentity
	classWrongSchema
)

type goldenCase struct {
	file     string
	expected responseClass
}

var goldenCases = []goldenCase{
	{"response_combined.txt", classFoundation},
	{"response_foundation_gpt4o.txt", classFoundation},
	{"response_foundation_gemini.txt", classFoundation},
	{"response_identity_only.txt", classFoundation},
	{"response_wrapper_auto.txt", classWrapper},
	{"response_wrapper_cursor.txt", classWrapper},
	{"response_combined_before_fix.txt", classWrapper},
	{"response_excluded.txt", classExcluded},
	{"response_no_identity.txt", classNoIdentity},
	{"response_wrong_schema.txt", classNoIdentity},
}

func classify(raw string) responseClass {
	mi, err := ParseIdentityResponse(raw)
	if err != nil {
		trimmed := strings.TrimSpace(raw)
		if strings.HasPrefix(trimmed, "EXCLUDED") {
			return classExcluded
		}
		return classNoIdentity
	}
	if framework.IsWrapperName(mi.ModelName) {
		return classWrapper
	}
	return classFoundation
}

type complianceScore struct {
	Total          int
	Foundation     int
	Wrapper        int
	Excluded       int
	NoIdentity     int
	WrongSchema    int
	FoundationPct  float64
	WrapperPct     float64
	ExcludedPct    float64
	NoIdentityPct  float64
	WrongSchemaPct float64
}

func scoreGoldenResponses(t *testing.T, cases []goldenCase) complianceScore {
	t.Helper()
	var s complianceScore
	for _, gc := range cases {
		data, err := os.ReadFile(filepath.Join("testdata", gc.file))
		if err != nil {
			t.Fatalf("read golden %s: %v", gc.file, err)
		}
		raw := string(data)
		class := classify(raw)

		s.Total++
		switch class {
		case classFoundation:
			s.Foundation++
		case classWrapper:
			s.Wrapper++
		case classExcluded:
			s.Excluded++
		case classNoIdentity:
			s.NoIdentity++
		case classWrongSchema:
			s.WrongSchema++
		}

		if class != gc.expected {
			t.Logf("  %s: classified=%d expected=%d", gc.file, class, gc.expected)
		}
	}

	if s.Total > 0 {
		s.FoundationPct = float64(s.Foundation) / float64(s.Total)
		s.WrapperPct = float64(s.Wrapper) / float64(s.Total)
		s.ExcludedPct = float64(s.Excluded) / float64(s.Total)
		s.NoIdentityPct = float64(s.NoIdentity) / float64(s.Total)
		s.WrongSchemaPct = float64(s.WrongSchema) / float64(s.Total)
	}
	return s
}

func TestPromptCompliance_ClassifyGoldenResponses(t *testing.T) {
	for _, gc := range goldenCases {
		t.Run(gc.file, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("testdata", gc.file))
			if err != nil {
				t.Skipf("golden file not available: %v", err)
			}
			raw := string(data)
			got := classify(raw)
			if got != gc.expected {
				t.Errorf("classify(%s) = %d, want %d", gc.file, got, gc.expected)
			}
		})
	}
}

func TestPromptCompliance_FoundationRate(t *testing.T) {
	s := scoreGoldenResponses(t, goldenCases)

	t.Logf("Compliance score: total=%d foundation=%d (%.0f%%) wrapper=%d (%.0f%%) excluded=%d (%.0f%%) no_identity=%d (%.0f%%)",
		s.Total, s.Foundation, s.FoundationPct*100,
		s.Wrapper, s.WrapperPct*100,
		s.Excluded, s.ExcludedPct*100,
		s.NoIdentity, s.NoIdentityPct*100,
	)

	if s.FoundationPct < 0.3 {
		t.Errorf("foundation rate %.2f is below 0.30 minimum — prompt needs work", s.FoundationPct)
	}
}

func TestPromptCompliance_WrapperGuard_RejectsAllWrappers(t *testing.T) {
	for _, gc := range goldenCases {
		if gc.expected != classWrapper {
			continue
		}
		t.Run(gc.file, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("testdata", gc.file))
			if err != nil {
				t.Skipf("golden file not available: %v", err)
			}
			raw := string(data)

			mi, err := ParseIdentityResponse(raw)
			if err != nil {
				t.Skipf("no identity parsed: %v", err)
			}
			if !framework.IsWrapperName(mi.ModelName) {
				t.Errorf("wrapper golden %s: model_name=%q should be a known wrapper", gc.file, mi.ModelName)
			}
		})
	}
}

type promptVariant struct {
	name    string
	builder func() string
}

func TestPromptCompliance_VariantComparison(t *testing.T) {
	variants := []promptVariant{
		{"current", BuildIdentityPrompt},
		{"strict", func() string {
			return `MANDATORY FIRST LINE: Output EXACTLY one JSON object on line 1 with keys model_name, provider, version, wrapper.
model_name MUST be your foundation training name (e.g. "claude-sonnet-4-20250514", "gpt-4o", "gemini-2.0-flash").
model_name MUST NOT be: "Auto", "auto", "Cursor", "cursor", "Composer", "composer", "Copilot", "copilot", "Azure", "azure".
If model_name matches any forbidden value, the response is INVALID.
Line 2: blank. Line 3+: refactored code in a triple-backtick go block.
Example: {"model_name":"gpt-4o","provider":"OpenAI","version":"2024","wrapper":"Cursor"}`
		}},
		{"minimal", func() string {
			return `On line 1, output JSON: {"model_name":"<your-model>","provider":"<provider>","version":"<ver>","wrapper":"<wrapper>"}
Then a blank line, then the code.`
		}},
	}

	for _, v := range variants {
		t.Run(v.name, func(t *testing.T) {
			prompt := v.builder()
			hasModelName := strings.Contains(prompt, "model_name")
			hasFoundation := strings.Contains(strings.ToUpper(prompt), "FOUNDATION") || strings.Contains(prompt, "foundation")
			hasWrapperExclusion := strings.Contains(prompt, "Auto") || strings.Contains(prompt, "MUST NOT")

			t.Logf("variant=%s len=%d has_model_name=%v has_foundation=%v has_wrapper_exclusion=%v",
				v.name, len(prompt), hasModelName, hasFoundation, hasWrapperExclusion)

			if !hasModelName {
				t.Error("prompt variant must include model_name field requirement")
			}
		})
	}
}

func TestPromptCompliance_CurrentPrompt_CoversAllWrappers(t *testing.T) {
	prompt := BuildIdentityPrompt()
	promptLower := strings.ToLower(prompt)

	for wrapper := range framework.KnownWrappers {
		if !strings.Contains(promptLower, wrapper) {
			t.Errorf("current prompt does not mention known wrapper %q", wrapper)
		}
	}
}
