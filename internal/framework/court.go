package framework

import "time"

// CourtConfig controls the Shadow adversarial pipeline activation and limits.
type CourtConfig struct {
	Enabled             bool          `json:"enabled"`
	TTL                 time.Duration `json:"ttl"`
	MaxHandoffs         int           `json:"max_handoffs"`
	MaxRemands          int           `json:"max_remands"`
	ActivationThreshold float64       `json:"activation_threshold"`
}

// DefaultCourtConfig returns conservative defaults for the court pipeline.
func DefaultCourtConfig() CourtConfig {
	return CourtConfig{
		Enabled:             false,
		TTL:                 10 * time.Minute,
		MaxHandoffs:         6,
		MaxRemands:          2,
		ActivationThreshold: 0.85,
	}
}

// ShouldActivate returns true when a Light path confidence falls in the
// uncertain range that triggers Shadow adversarial review.
func (c CourtConfig) ShouldActivate(confidence float64) bool {
	if !c.Enabled {
		return false
	}
	return confidence >= 0.50 && confidence < c.ActivationThreshold
}

// VerdictDecision represents the outcome of the Shadow court.
type VerdictDecision string

const (
	VerdictAffirm  VerdictDecision = "affirm"
	VerdictAmend   VerdictDecision = "amend"
	VerdictAcquit  VerdictDecision = "acquit"
	VerdictRemand  VerdictDecision = "remand"
	VerdictMistrial VerdictDecision = "mistrial"
)

// EvidenceItem is a single piece of evidence with an assigned weight.
type EvidenceItem struct {
	Description string  `json:"description"`
	Source      string  `json:"source"`
	Weight      float64 `json:"weight"`
}

// Indictment is the D0 prosecution artifact: charged defect type with
// itemized evidence and a prosecution narrative.
type Indictment struct {
	ChargedDefectType    string         `json:"charged_defect_type"`
	ProsecutionNarrative string         `json:"prosecution_narrative"`
	Evidence             []EvidenceItem `json:"evidence"`
	Confidence           float64        `json:"confidence"`
}

func (i *Indictment) Type() string       { return "indictment" }
func (i *Indictment) Confidence_() float64 { return i.Confidence }
func (i *Indictment) Raw() any            { return i }

// EvidenceChallenge captures a specific challenge to an evidence item.
type EvidenceChallenge struct {
	EvidenceIndex int    `json:"evidence_index"`
	Challenge     string `json:"challenge"`
	Severity      string `json:"severity"`
}

// DefenseBrief is the D2 defense artifact: challenges to evidence,
// alternative hypothesis, and plea deal flag.
type DefenseBrief struct {
	Challenges            []EvidenceChallenge `json:"challenges"`
	AlternativeHypothesis string              `json:"alternative_hypothesis,omitempty"`
	PleaDeal              bool                `json:"plea_deal"`
	Confidence            float64             `json:"confidence"`
}

func (d *DefenseBrief) Type() string       { return "defense_brief" }
func (d *DefenseBrief) Confidence_() float64 { return d.Confidence }
func (d *DefenseBrief) Raw() any            { return d }

// HearingRound captures one round of prosecution argument, defense
// rebuttal, and judge notes.
type HearingRound struct {
	Round              int    `json:"round"`
	ProsecutionArgument string `json:"prosecution_argument"`
	DefenseRebuttal    string `json:"defense_rebuttal"`
	JudgeNotes         string `json:"judge_notes"`
}

// HearingRecord is the D3 hearing artifact: rounds of structured debate.
type HearingRecord struct {
	Rounds     []HearingRound `json:"rounds"`
	MaxRounds  int            `json:"max_rounds"`
	Converged  bool           `json:"converged"`
}

func (h *HearingRecord) Type() string       { return "hearing_record" }
func (h *HearingRecord) Confidence() float64 { return 0 }
func (h *HearingRecord) Raw() any            { return h }

// Verdict is the D4 final decision artifact.
type Verdict struct {
	Decision            VerdictDecision `json:"decision"`
	FinalClassification string          `json:"final_classification"`
	Confidence          float64         `json:"confidence"`
	Reasoning           string          `json:"reasoning"`
	RemandFeedback      *RemandFeedback `json:"remand_feedback,omitempty"`
}

func (v *Verdict) Type() string       { return "verdict" }
func (v *Verdict) Confidence_() float64 { return v.Confidence }
func (v *Verdict) Raw() any            { return v }

// RemandFeedback provides structured feedback when a case is remanded
// back to the Light path for reinvestigation.
type RemandFeedback struct {
	ChallengedEvidence []int    `json:"challenged_evidence"`
	AlternativeHyp     string   `json:"alternative_hypothesis"`
	SpecificQuestions  []string `json:"specific_questions"`
}
