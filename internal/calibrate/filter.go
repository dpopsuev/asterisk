package calibrate

import "strings"

// FilterByGrade returns a new Scenario containing only cases whose linked RCA
// has an EvidenceGrade matching one of the comma-separated grades (e.g. "A" or
// "A,B"). RCAs, symptoms, and workspace are carried over; only cases are pruned.
func FilterByGrade(s *Scenario, grades string) *Scenario {
	allow := make(map[string]bool)
	for _, g := range strings.Split(grades, ",") {
		g = strings.TrimSpace(strings.ToUpper(g))
		if g != "" {
			allow[g] = true
		}
	}

	rcaGrade := make(map[string]string, len(s.RCAs))
	for _, r := range s.RCAs {
		rcaGrade[r.ID] = r.EvidenceGrade
	}

	var filtered []GroundTruthCase
	for _, c := range s.Cases {
		if grade, ok := rcaGrade[c.RCAID]; ok && allow[grade] {
			filtered = append(filtered, c)
		}
	}

	return &Scenario{
		Name:        s.Name,
		Description: s.Description,
		RCAs:        s.RCAs,
		Symptoms:    s.Symptoms,
		Cases:       filtered,
		Workspace:   s.Workspace,
	}
}
