package calibrate

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dpopsuev/origami/format"
	"asterisk/internal/orchestrate"
)

// TranscriptEntry represents one round of the Asterisk-agent dialog.
type TranscriptEntry struct {
	Step        string `json:"step"`         // pipeline step code, e.g. "F0_RECALL"
	StepName    string `json:"step_name"`    // human-readable, e.g. "Recall"
	Prompt      string `json:"prompt"`       // markdown prompt content; empty when not available on disk
	Response    string `json:"response"`     // raw JSON response
	HeuristicID string `json:"heuristic_id"` // which rule fired, e.g. "H2"
	Decision    string `json:"decision"`     // outcome explanation
	Timestamp   string `json:"timestamp"`    // ISO 8601
}

// CaseTranscript holds the full dialog for one case.
type CaseTranscript struct {
	CaseID   string            `json:"case_id"`
	TestName string            `json:"test_name"`
	Version  string            `json:"version"`
	Job      string            `json:"job"`
	Path     []string          `json:"path"`
	Entries  []TranscriptEntry `json:"entries"`
}

// RCATranscript groups one or more cases that share the same Root Cause.
type RCATranscript struct {
	RCAID      int64            `json:"rca_id"`
	Component  string           `json:"component"`
	DefectType string           `json:"defect_type"`
	RCAMessage string           `json:"rca_message"`
	Primary    *CaseTranscript  `json:"primary"`
	Correlated []CaseTranscript `json:"correlated,omitempty"`
}

// WeaveTranscripts reads calibration artifacts from disk and produces one
// RCATranscript per distinct Root Cause.  Returns nil (not an error) when
// weaving is not possible (e.g. no artifact directories found).
func WeaveTranscripts(report *CalibrationReport) ([]RCATranscript, error) {
	if report == nil || len(report.CaseResults) == 0 {
		return nil, nil
	}

	groups := groupByRCA(report.CaseResults)
	var transcripts []RCATranscript

	for rcaID, cases := range groups {
		t := RCATranscript{RCAID: rcaID}

		// Pick the primary case: the one with the longest pipeline path
		// (i.e. the one that went through full investigation).
		primary := pickPrimary(cases)
		t.Component = primary.ActualComponent
		t.DefectType = primary.ActualDefectType
		t.RCAMessage = primary.ActualRCAMessage

		ct, err := buildCaseTranscript(report, primary)
		if err != nil {
			return nil, fmt.Errorf("weave case %s: %w", primary.CaseID, err)
		}
		t.Primary = ct

		for i := range cases {
			if cases[i].CaseID == primary.CaseID {
				continue
			}
			corr, err := buildCaseTranscript(report, &cases[i])
			if err != nil {
				return nil, fmt.Errorf("weave correlated case %s: %w", cases[i].CaseID, err)
			}
			t.Correlated = append(t.Correlated, *corr)
		}

		transcripts = append(transcripts, t)
	}

	return transcripts, nil
}

// RenderRCATranscript produces a Markdown document for one RCA transcript.
// Entries are rendered in reverse chronological order (conclusion first).
func RenderRCATranscript(t *RCATranscript) string {
	var b strings.Builder

	// Header
	b.WriteString(fmt.Sprintf("# RCA Transcript — %s: %s\n\n",
		t.Component, vocabNameWithCode(t.DefectType)))

	tbl := format.NewTable(format.Markdown)
	tbl.Header("Field", "Value")
	tbl.Row("RCA ID", fmt.Sprintf("%d", t.RCAID))
	tbl.Row("Component", t.Component)
	tbl.Row("Defect Type", vocabNameWithCode(t.DefectType))

	caseIDs := []string{t.Primary.CaseID + " (primary)"}
	for _, c := range t.Correlated {
		caseIDs = append(caseIDs, c.CaseID+" (correlated)")
	}
	tbl.Row("Cases", strings.Join(caseIDs, ", "))
	tbl.Row("Generated", time.Now().UTC().Format(time.RFC3339))
	b.WriteString(tbl.String())
	b.WriteString("\n\n---\n\n")

	// Primary case
	b.WriteString(fmt.Sprintf("## Primary Investigation: Case %s\n\n", t.Primary.CaseID))
	b.WriteString(fmt.Sprintf("**Test:** %s  \n", t.Primary.TestName))
	b.WriteString(fmt.Sprintf("**Path:** %s\n\n", vocabStagePath(t.Primary.Path)))
	renderEntries(&b, t.Primary.Entries)

	// Correlated cases
	for _, c := range t.Correlated {
		b.WriteString("---\n\n")
		b.WriteString(fmt.Sprintf("## Correlated Case: %s\n\n", c.CaseID))
		b.WriteString(fmt.Sprintf("**Test:** %s  \n", c.TestName))
		b.WriteString(fmt.Sprintf("**Path:** %s\n\n", vocabStagePath(c.Path)))
		renderEntries(&b, c.Entries)
	}

	return b.String()
}

// TranscriptSlug returns a filesystem-safe slug for naming the transcript file.
func TranscriptSlug(t *RCATranscript) string {
	comp := strings.ToLower(strings.ReplaceAll(t.Component, " ", "-"))
	dt := strings.ToLower(t.DefectType)
	if comp == "" {
		comp = "unknown"
	}
	if dt == "" {
		dt = "unknown"
	}
	return fmt.Sprintf("rca-transcript-%s-%s", comp, dt)
}

// --- internal helpers ---

// groupByRCA groups CaseResults by ActualRCAID.
// Cases with ActualRCAID == 0 each get their own group keyed by negative StoreCaseID
// to avoid collisions.
func groupByRCA(results []CaseResult) map[int64][]CaseResult {
	groups := make(map[int64][]CaseResult)
	for _, cr := range results {
		key := cr.ActualRCAID
		if key == 0 {
			key = -cr.StoreCaseID // unique negative key per orphan
		}
		groups[key] = append(groups[key], cr)
	}
	return groups
}

// pickPrimary selects the case with the longest pipeline path as the
// primary investigation (the one that went deepest into the pipeline).
func pickPrimary(cases []CaseResult) *CaseResult {
	best := &cases[0]
	for i := 1; i < len(cases); i++ {
		if len(cases[i].ActualPath) > len(best.ActualPath) {
			best = &cases[i]
		}
	}
	return best
}

// buildCaseTranscript reads state.json and per-step artifacts from disk
// to construct a CaseTranscript.
func buildCaseTranscript(report *CalibrationReport, cr *CaseResult) (*CaseTranscript, error) {
	ct := &CaseTranscript{
		CaseID:   cr.CaseID,
		TestName: cr.TestName,
		Version:  cr.Version,
		Job:      cr.Job,
		Path:     cr.ActualPath,
	}

	caseDir := orchestrate.CaseDir(report.BasePath, report.SuiteID, cr.StoreCaseID)

	state, err := orchestrate.LoadState(caseDir)
	if err != nil {
		return ct, fmt.Errorf("load state: %w", err)
	}
	if state == nil {
		// No state on disk (e.g. temp dir already cleaned); return what we have from CaseResult.
		return ct, nil
	}

	// Build entries from history — skip INIT (no artifact) and final DONE transition.
	for _, record := range state.History {
		step := record.Step
		if step == orchestrate.StepInit || step == orchestrate.StepDone {
			continue
		}

		entry := TranscriptEntry{
			Step:        string(step),
			StepName:    vocabName(string(step)),
			HeuristicID: record.HeuristicID,
			Decision:    record.Outcome,
			Timestamp:   record.Timestamp,
		}

		// Best-effort: read prompt from disk (LLMAdapter writes these).
		promptFile := orchestrate.PromptFilename(step, 0)
		if promptFile != "" {
			if data, err := os.ReadFile(filepath.Join(caseDir, promptFile)); err == nil {
				entry.Prompt = string(data)
			}
		}

		// Read artifact from disk (written by runner for all adapters).
		artifactFile := orchestrate.ArtifactFilename(step)
		if artifactFile != "" {
			if data, err := os.ReadFile(filepath.Join(caseDir, artifactFile)); err == nil {
				// Pretty-print if it's valid JSON.
				var buf json.RawMessage
				if json.Unmarshal(data, &buf) == nil {
					if pretty, err := json.MarshalIndent(buf, "", "  "); err == nil {
						entry.Response = string(pretty)
					} else {
						entry.Response = string(data)
					}
				} else {
					entry.Response = string(data)
				}
			}
		}

		ct.Entries = append(ct.Entries, entry)
	}

	return ct, nil
}

// renderEntries writes transcript entries in reverse order (conclusion first).
func renderEntries(b *strings.Builder, entries []TranscriptEntry) {
	for i := len(entries) - 1; i >= 0; i-- {
		e := entries[i]
		b.WriteString(fmt.Sprintf("### %s %s (%s)\n\n", e.Step, e.StepName, e.Timestamp))

		if e.Prompt != "" {
			b.WriteString("#### Prompt\n\n")
			// Indent prompt as blockquote for readability.
			for _, line := range strings.Split(e.Prompt, "\n") {
				b.WriteString("> " + line + "\n")
			}
			b.WriteString("\n")
		}

		if e.Response != "" {
			b.WriteString("#### Response\n\n")
			b.WriteString("```json\n")
			b.WriteString(e.Response)
			b.WriteString("\n```\n\n")
		}

		b.WriteString(fmt.Sprintf("#### Decision: %s — %s\n\n", e.HeuristicID, e.Decision))
	}
}
