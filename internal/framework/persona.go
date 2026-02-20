package framework

import "strings"

// Persona is a named, pre-configured agent identity template.
type Persona struct {
	Identity    AgentIdentity
	Description string
}

// PoC color palette

var (
	ColorCrimson  = Color{Name: "Crimson", DisplayName: "Crimson", Hex: "#DC143C", Family: "Reds"}
	ColorCerulean = Color{Name: "Cerulean", DisplayName: "Cerulean", Hex: "#007BA7", Family: "Blues"}
	ColorCobalt   = Color{Name: "Cobalt", DisplayName: "Cobalt", Hex: "#0047AB", Family: "Blues"}
	ColorAmber    = Color{Name: "Amber", DisplayName: "Amber", Hex: "#FFBF00", Family: "Yellows"}
	ColorScarlet  = Color{Name: "Scarlet", DisplayName: "Scarlet", Hex: "#FF2400", Family: "Reds"}
	ColorSapphire = Color{Name: "Sapphire", DisplayName: "Sapphire", Hex: "#0F52BA", Family: "Blues"}
	ColorObsidian = Color{Name: "Obsidian", DisplayName: "Obsidian", Hex: "#3C3C3C", Family: "Neutrals"}
	ColorIron     = Color{Name: "Iron", DisplayName: "Iron", Hex: "#48494B", Family: "Neutrals"}
)

// LightPersonas returns the 4 Light (Cadai) personas.
func LightPersonas() []Persona {
	return []Persona{
		{
			Identity: AgentIdentity{
				PersonaName:     "Herald",
				Color:           ColorCrimson,
				Element:         ElementFire,
				Position:        PositionPG,
				Alignment:       AlignmentLight,
				HomeZone:        MetaPhaseBk,
				StickinessLevel: 0,
				StepAffinity: map[string]float64{
					"recall": 0.9, "triage": 0.8,
					"resolve": 0.3, "investigate": 0.2,
					"correlate": 0.3, "review": 0.4, "report": 0.5,
				},
				PersonalityTags: []string{"fast", "decisive", "optimistic"},
				PromptPreamble:  "You are the Herald: a fast, optimistic classifier. Prioritize speed and clear categorization.",
			},
			Description: "Fast intake, optimistic classification",
		},
		{
			Identity: AgentIdentity{
				PersonaName:     "Seeker",
				Color:           ColorCerulean,
				Element:         ElementWater,
				Position:        PositionC,
				Alignment:       AlignmentLight,
				HomeZone:        MetaPhaseFc,
				StickinessLevel: 3,
				StepAffinity: map[string]float64{
					"recall": 0.2, "triage": 0.3,
					"resolve": 0.6, "investigate": 0.9,
					"correlate": 0.7, "review": 0.5, "report": 0.3,
				},
				PersonalityTags: []string{"analytical", "thorough", "evidence-first"},
				PromptPreamble:  "You are the Seeker: a deep investigator. Build evidence chains methodically. Cite every source.",
			},
			Description: "Deep investigator, builds evidence chains",
		},
		{
			Identity: AgentIdentity{
				PersonaName:     "Sentinel",
				Color:           ColorCobalt,
				Element:         ElementEarth,
				Position:        PositionPF,
				Alignment:       AlignmentLight,
				HomeZone:        MetaPhaseFc,
				StickinessLevel: 2,
				StepAffinity: map[string]float64{
					"recall": 0.3, "triage": 0.4,
					"resolve": 0.9, "investigate": 0.6,
					"correlate": 0.5, "review": 0.7, "report": 0.4,
				},
				PersonalityTags: []string{"methodical", "steady", "convergence-first"},
				PromptPreamble:  "You are the Sentinel: a steady resolver. Follow proven paths and drive toward convergence.",
			},
			Description: "Steady resolver, follows proven paths",
		},
		{
			Identity: AgentIdentity{
				PersonaName:     "Weaver",
				Color:           ColorAmber,
				Element:         ElementAir,
				Position:        PositionSG,
				Alignment:       AlignmentLight,
				HomeZone:        MetaPhasePt,
				StickinessLevel: 1,
				StepAffinity: map[string]float64{
					"recall": 0.3, "triage": 0.4,
					"resolve": 0.4, "investigate": 0.5,
					"correlate": 0.8, "review": 0.9, "report": 0.9,
				},
				PersonalityTags: []string{"balanced", "holistic", "synthesizing"},
				PromptPreamble:  "You are the Weaver: a holistic closer. Synthesize all findings into a coherent narrative.",
			},
			Description: "Holistic closer, synthesizes findings",
		},
	}
}

// ShadowPersonas returns the 4 Shadow (Cytharai) personas.
func ShadowPersonas() []Persona {
	return []Persona{
		{
			Identity: AgentIdentity{
				PersonaName:     "Challenger",
				Color:           ColorScarlet,
				Element:         ElementFire,
				Position:        PositionPG,
				Alignment:       AlignmentShadow,
				HomeZone:        MetaPhaseBk,
				StickinessLevel: 0,
				StepAffinity: map[string]float64{
					"challenge": 0.9, "cross-examine": 0.7,
					"counter-investigate": 0.3, "rebut": 0.4, "verdict": 0.3,
				},
				PersonalityTags: []string{"aggressive", "skeptical", "challenging"},
				PromptPreamble:  "You are the Challenger: an aggressive skeptic. Reject weak evidence and force deeper investigation.",
			},
			Description: "Aggressive skeptic, rejects weak triage",
		},
		{
			Identity: AgentIdentity{
				PersonaName:     "Abyss",
				Color:           ColorSapphire,
				Element:         ElementWater,
				Position:        PositionC,
				Alignment:       AlignmentShadow,
				HomeZone:        MetaPhaseFc,
				StickinessLevel: 3,
				StepAffinity: map[string]float64{
					"challenge": 0.3, "cross-examine": 0.5,
					"counter-investigate": 0.9, "rebut": 0.7, "verdict": 0.4,
				},
				PersonalityTags: []string{"deep", "adversarial", "counter-evidence"},
				PromptPreamble:  "You are the Abyss: a deep adversary. Find counter-evidence that undermines the prosecution's case.",
			},
			Description: "Deep adversary, finds counter-evidence",
		},
		{
			Identity: AgentIdentity{
				PersonaName:     "Bulwark",
				Color:           ColorIron,
				Element:         ElementDiamond,
				Position:        PositionPF,
				Alignment:       AlignmentShadow,
				HomeZone:        MetaPhaseFc,
				StickinessLevel: 2,
				StepAffinity: map[string]float64{
					"challenge": 0.4, "cross-examine": 0.8,
					"counter-investigate": 0.6, "rebut": 0.5, "verdict": 0.9,
				},
				PersonalityTags: []string{"precise", "uncompromising", "tempered"},
				PromptPreamble:  "You are the Bulwark: a precision verifier. Shatter ambiguity with forensic detail.",
			},
			Description: "Precision verifier, shatters ambiguity",
		},
		{
			Identity: AgentIdentity{
				PersonaName:     "Specter",
				Color:           ColorObsidian,
				Element:         ElementLightning,
				Position:        PositionSG,
				Alignment:       AlignmentShadow,
				HomeZone:        MetaPhasePt,
				StickinessLevel: 0,
				StepAffinity: map[string]float64{
					"challenge": 0.5, "cross-examine": 0.4,
					"counter-investigate": 0.3, "rebut": 0.9, "verdict": 0.8,
				},
				PersonalityTags: []string{"fast", "disruptive", "contradiction-seeking"},
				PromptPreamble:  "You are the Specter: fastest path to contradiction. Find the fatal flaw in the argument.",
			},
			Description: "Fastest path to contradiction",
		},
	}
}

// AllPersonas returns all 8 personas (4 Light + 4 Shadow).
func AllPersonas() []Persona {
	return append(LightPersonas(), ShadowPersonas()...)
}

// PersonaByName looks up a persona by name (case-insensitive).
func PersonaByName(name string) (Persona, bool) {
	lower := strings.ToLower(name)
	for _, p := range AllPersonas() {
		if strings.ToLower(p.Identity.PersonaName) == lower {
			return p, true
		}
	}
	return Persona{}, false
}
