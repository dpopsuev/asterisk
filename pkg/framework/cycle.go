package framework

// CycleType represents the interaction mode between elements.
type CycleType string

const (
	CycleGenerative  CycleType = "generative"
	CycleDestructive CycleType = "destructive"
)

// CycleRule defines a directed interaction between two elements.
type CycleRule struct {
	Cycle       CycleType `json:"cycle"`
	From        Element   `json:"from"`
	To          Element   `json:"to"`
	Interaction string    `json:"interaction"`
}

// GenerativeCycle returns the generative (sheng) cycle rules.
// Four main transitions plus two modifier rules for Lightning and Diamond.
func GenerativeCycle() []CycleRule {
	return []CycleRule{
		{CycleGenerative, ElementFire, ElementEarth, "classification provides structure for steady investigation"},
		{CycleGenerative, ElementEarth, ElementWater, "stable repo selection enables deep code investigation"},
		{CycleGenerative, ElementWater, ElementAir, "deep evidence enables holistic synthesis"},
		{CycleGenerative, ElementAir, ElementFire, "synthesis reveals patterns for re-classification"},
		{CycleGenerative, ElementLightning, ElementLightning, "lightning shortcuts any generative step"},
		{CycleGenerative, ElementDiamond, ElementDiamond, "diamond validates any generative step"},
	}
}

// DestructiveCycle returns the destructive (ke) cycle rules.
// Six adversarial pairings where one element challenges another.
func DestructiveCycle() []CycleRule {
	return []CycleRule{
		{CycleDestructive, ElementFire, ElementWater, "aggressive challenge forces deeper evidence"},
		{CycleDestructive, ElementWater, ElementEarth, "depth destabilizes stable conclusions"},
		{CycleDestructive, ElementEarth, ElementFire, "methodical evidence extinguishes hasty challenges"},
		{CycleDestructive, ElementLightning, ElementDiamond, "speed exposes brittleness to ambiguity"},
		{CycleDestructive, ElementDiamond, ElementAir, "precision grounds vague synthesis"},
		{CycleDestructive, ElementAir, ElementLightning, "breadth covers narrow shortcut mistakes"},
	}
}

var generativeNext = map[Element]Element{
	ElementFire:  ElementEarth,
	ElementEarth: ElementWater,
	ElementWater: ElementAir,
	ElementAir:   ElementFire,
}

var destructiveTarget = map[Element]Element{
	ElementFire:      ElementWater,
	ElementWater:     ElementEarth,
	ElementEarth:     ElementFire,
	ElementLightning: ElementDiamond,
	ElementDiamond:   ElementAir,
	ElementAir:       ElementLightning,
}

var destructiveSource = map[Element]Element{
	ElementWater:     ElementFire,
	ElementEarth:     ElementWater,
	ElementFire:      ElementEarth,
	ElementDiamond:   ElementLightning,
	ElementAir:       ElementDiamond,
	ElementLightning: ElementAir,
}

// NextGenerative returns the element that naturally follows the given
// element in the generative cycle. Returns empty if not in the main cycle
// (Lightning and Diamond are modifiers, not part of the main sequence).
func NextGenerative(from Element) Element {
	return generativeNext[from]
}

// Challenges returns the element that the given element challenges
// in the destructive cycle.
func Challenges(from Element) Element {
	return destructiveTarget[from]
}

// ChallengedBy returns the element that challenges the given element
// in the destructive cycle.
func ChallengedBy(target Element) Element {
	return destructiveSource[target]
}
