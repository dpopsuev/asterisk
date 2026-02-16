package investigate

import "asterisk/internal/preinvest"

// CaseIDsFromEnvelope returns one case ID per failure (step) for artifact shape.
func CaseIDsFromEnvelope(env *preinvest.Envelope) []int {
	if env == nil {
		return nil
	}
	ids := make([]int, 0, len(env.FailureList))
	for _, f := range env.FailureList {
		ids = append(ids, f.ID)
	}
	return ids
}
