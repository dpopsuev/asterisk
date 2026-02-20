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
	ConfidenceScore      float64        `json:"confidence"`
}

func (i *Indictment) Type() string       { return "indictment" }
func (i *Indictment) Confidence() float64 { return i.ConfidenceScore }
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
	ConfidenceScore       float64             `json:"confidence"`
}

func (d *DefenseBrief) Type() string       { return "defense_brief" }
func (d *DefenseBrief) Confidence() float64 { return d.ConfidenceScore }
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
	ConfidenceScore     float64         `json:"confidence"`
	Reasoning           string          `json:"reasoning"`
	RemandFeedback      *RemandFeedback `json:"remand_feedback,omitempty"`
}

func (v *Verdict) Type() string       { return "verdict" }
func (v *Verdict) Confidence() float64 { return v.ConfidenceScore }
func (v *Verdict) Raw() any            { return v }

// RemandFeedback provides structured feedback when a case is remanded
// back to the Light path for reinvestigation.
type RemandFeedback struct {
	ChallengedEvidence []int    `json:"challenged_evidence"`
	AlternativeHyp     string   `json:"alternative_hypothesis"`
	SpecificQuestions  []string `json:"specific_questions"`
}

// CourtEvidenceGap captures a gap discovered during adversarial review.
type CourtEvidenceGap struct {
	Description string `json:"description"`
	Source      string `json:"source"`
	Severity    string `json:"severity"`
	SuggestedAction string `json:"suggested_action,omitempty"`
}

// BuildCourtEdgeFactory returns an EdgeFactory with skeleton court heuristic
// evaluators (HD1-HD12) for the defect-court pipeline. Each evaluator checks
// the artifact type and court-specific conditions.
func BuildCourtEdgeFactory(cfg CourtConfig) EdgeFactory {
	return EdgeFactory{
		"HD1": courtEdgeFactory(func(a Artifact, _ *WalkerState) *Transition {
			ind, ok := unwrapIndictment(a)
			if !ok {
				return nil
			}
			if ind.ConfidenceScore >= 0.95 {
				return &Transition{NextNode: "defend", Explanation: "fast-track: prosecution confidence >= 0.95"}
			}
			return nil
		}),
		"HD2": courtEdgeFactory(func(a Artifact, _ *WalkerState) *Transition {
			brief, ok := unwrapDefenseBrief(a)
			if !ok {
				return nil
			}
			if brief.PleaDeal {
				return &Transition{NextNode: "verdict", Explanation: "plea deal: defense concedes"}
			}
			return nil
		}),
		"HD3": courtEdgeFactory(func(a Artifact, _ *WalkerState) *Transition {
			brief, ok := unwrapDefenseBrief(a)
			if !ok {
				return nil
			}
			if len(brief.Challenges) > 0 && brief.AlternativeHypothesis == "" {
				return &Transition{NextNode: "hearing", Explanation: "motion to dismiss: challenges without alternative"}
			}
			return nil
		}),
		"HD4": courtEdgeFactory(func(a Artifact, _ *WalkerState) *Transition {
			brief, ok := unwrapDefenseBrief(a)
			if !ok {
				return nil
			}
			if brief.AlternativeHypothesis != "" {
				return &Transition{NextNode: "hearing", Explanation: "alternative hypothesis presented"}
			}
			return nil
		}),
		"HD5": courtEdgeFactory(func(a Artifact, s *WalkerState) *Transition {
			rec, ok := unwrapHearingRecord(a)
			if !ok {
				return nil
			}
			if rec.Converged || len(rec.Rounds) >= rec.MaxRounds {
				return &Transition{NextNode: "verdict", Explanation: "hearing complete"}
			}
			return nil
		}),
		"HD6": courtEdgeFactory(func(a Artifact, _ *WalkerState) *Transition {
			v, ok := unwrapVerdict(a)
			if !ok {
				return nil
			}
			if v.Decision == VerdictAffirm {
				return &Transition{NextNode: "_done", Explanation: "verdict: affirm"}
			}
			return nil
		}),
		"HD7": courtEdgeFactory(func(a Artifact, _ *WalkerState) *Transition {
			v, ok := unwrapVerdict(a)
			if !ok {
				return nil
			}
			if v.Decision == VerdictAmend {
				return &Transition{NextNode: "_done", Explanation: "verdict: amend"}
			}
			return nil
		}),
		"HD8": courtEdgeFactory(func(a Artifact, s *WalkerState) *Transition {
			v, ok := unwrapVerdict(a)
			if !ok {
				return nil
			}
			if v.Decision == VerdictRemand && s.LoopCounts["verdict"] < cfg.MaxRemands {
				return &Transition{
					NextNode:    "indict",
					Explanation: "verdict: remand for reinvestigation",
				}
			}
			return nil
		}),
		"HD9": courtEdgeFactory(func(a Artifact, _ *WalkerState) *Transition {
			v, ok := unwrapVerdict(a)
			if !ok {
				return nil
			}
			if v.Decision == VerdictAcquit {
				return &Transition{NextNode: "_done", Explanation: "verdict: acquit (evidence gap brief)"}
			}
			return nil
		}),
		"HD10": courtEdgeFactory(func(_ Artifact, s *WalkerState) *Transition {
			if s.LoopCounts["_handoff"] > cfg.MaxHandoffs {
				return &Transition{NextNode: "_done", Explanation: "mistrial: handoff limit exceeded"}
			}
			return nil
		}),
		"HD11": courtEdgeFactory(func(_ Artifact, s *WalkerState) *Transition {
			if s.LoopCounts["_handoff"] > cfg.MaxHandoffs {
				return &Transition{NextNode: "_done", Explanation: "mistrial: handoff counter exceeded"}
			}
			return nil
		}),
		"HD12": courtEdgeFactory(func(a Artifact, _ *WalkerState) *Transition {
			v, ok := unwrapVerdict(a)
			if !ok {
				return nil
			}
			if v.Decision == VerdictMistrial {
				return &Transition{NextNode: "_done", Explanation: "verdict: mistrial declared by judge"}
			}
			return nil
		}),
	}
}

type courtEvalFunc func(Artifact, *WalkerState) *Transition

func courtEdgeFactory(eval courtEvalFunc) func(EdgeDef) Edge {
	return func(def EdgeDef) Edge {
		return &courtEdge{def: def, eval: eval}
	}
}

type courtEdge struct {
	def  EdgeDef
	eval courtEvalFunc
}

func (e *courtEdge) ID() string       { return e.def.ID }
func (e *courtEdge) From() string     { return e.def.From }
func (e *courtEdge) To() string       { return e.def.To }
func (e *courtEdge) IsShortcut() bool { return e.def.Shortcut }
func (e *courtEdge) IsLoop() bool     { return e.def.Loop }
func (e *courtEdge) Evaluate(a Artifact, s *WalkerState) *Transition {
	return e.eval(a, s)
}

func unwrapIndictment(a Artifact) (*Indictment, bool) {
	if a == nil {
		return nil, false
	}
	ind, ok := a.Raw().(*Indictment)
	return ind, ok
}

func unwrapDefenseBrief(a Artifact) (*DefenseBrief, bool) {
	if a == nil {
		return nil, false
	}
	brief, ok := a.Raw().(*DefenseBrief)
	return brief, ok
}

func unwrapHearingRecord(a Artifact) (*HearingRecord, bool) {
	if a == nil {
		return nil, false
	}
	rec, ok := a.Raw().(*HearingRecord)
	return rec, ok
}

func unwrapVerdict(a Artifact) (*Verdict, bool) {
	if a == nil {
		return nil, false
	}
	v, ok := a.Raw().(*Verdict)
	return v, ok
}
