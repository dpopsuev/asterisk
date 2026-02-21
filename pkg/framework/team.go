package framework

// Team bundles multiple walkers with scheduling and observability for
// team-based graph traversal. A single-walker team with a nil observer
// is semantically equivalent to the original Walk method.
type Team struct {
	Walkers   []Walker
	Scheduler Scheduler
	Observer  WalkObserver
	MaxSteps  int // 0 = unlimited
}
