package framework

// SchedulerContext provides all information a Scheduler needs to pick a walker.
type SchedulerContext struct {
	Node        Node
	Zone        *Zone
	Walkers     []Walker
	PriorWalker Walker
	WalkState   *WalkerState
}

// Scheduler selects which walker handles a given node. Implementations
// range from trivial (SingleScheduler) to sophisticated (affinity +
// element cycles + zone stickiness). The interface is deliberately
// narrow so implementations can be swapped without pain.
type Scheduler interface {
	Select(ctx SchedulerContext) Walker
}

// SingleScheduler always returns the same walker. Use it to wrap
// legacy single-walker behavior into the team walk API.
type SingleScheduler struct {
	Walker Walker
}

func (s *SingleScheduler) Select(_ SchedulerContext) Walker {
	return s.Walker
}

// AffinityScheduler picks the walker whose StepAffinity for the node
// is highest. Ties are broken by element match (walker element ==
// node element affinity). Falls back to the first walker.
type AffinityScheduler struct{}

func (s *AffinityScheduler) Select(ctx SchedulerContext) Walker {
	if len(ctx.Walkers) == 0 {
		return nil
	}
	if len(ctx.Walkers) == 1 {
		return ctx.Walkers[0]
	}

	nodeName := ctx.Node.Name()
	nodeElement := ctx.Node.ElementAffinity()

	var best Walker
	bestScore := -1.0
	bestElementMatch := false

	for _, w := range ctx.Walkers {
		id := w.Identity()
		score := id.StepAffinity[nodeName]
		elementMatch := id.Element == nodeElement

		better := false
		switch {
		case score > bestScore:
			better = true
		case score == bestScore && elementMatch && !bestElementMatch:
			better = true
		}

		if better {
			best = w
			bestScore = score
			bestElementMatch = elementMatch
		}
	}

	if best == nil {
		return ctx.Walkers[0]
	}
	return best
}

// zoneForNode finds the zone containing a node, or nil.
func zoneForNode(nodeName string, zones []Zone) *Zone {
	for i := range zones {
		for _, n := range zones[i].NodeNames {
			if n == nodeName {
				return &zones[i]
			}
		}
	}
	return nil
}
