package rca

import (
	"time"
)

const stateFilename = "state.json"

// InitState creates a new CaseState starting at INIT.
func InitState(caseID, suiteID int64) *CaseState {
	return &CaseState{
		CaseID:      caseID,
		SuiteID:     suiteID,
		CurrentStep: StepInit,
		LoopCounts:  make(map[string]int),
		Status:      "running",
	}
}

// LoadState reads the persisted state from the case directory.
// Returns nil if no state file exists (new case).
func LoadState(caseDir string) (*CaseState, error) {
	state, err := ReadArtifact[CaseState](caseDir, stateFilename)
	if err != nil {
		return nil, err
	}
	return state, nil
}

// SaveState persists the case state to the case directory.
func SaveState(caseDir string, state *CaseState) error {
	return WriteArtifact(caseDir, stateFilename, state)
}

// AdvanceStep moves the state to the next step and records the transition.
func AdvanceStep(state *CaseState, nextStep CircuitStep, heuristicID, outcome string) {
	record := StepRecord{
		Step:        state.CurrentStep,
		Outcome:     outcome,
		HeuristicID: heuristicID,
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
	}
	state.History = append(state.History, record)
	state.CurrentStep = nextStep
	if nextStep == StepDone {
		state.Status = "done"
	}
}

// IncrementLoop increments the named loop counter and returns the new count.
func IncrementLoop(state *CaseState, loopName string) int {
	if state.LoopCounts == nil {
		state.LoopCounts = make(map[string]int)
	}
	state.LoopCounts[loopName]++
	return state.LoopCounts[loopName]
}

// IsLoopExhausted returns true if the named loop has reached or exceeded maxIterations.
func IsLoopExhausted(state *CaseState, loopName string, maxIterations int) bool {
	return state.LoopCounts[loopName] >= maxIterations
}

// LoopCount returns the current count for the named loop.
func LoopCount(state *CaseState, loopName string) int {
	return state.LoopCounts[loopName]
}
