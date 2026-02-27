package investigate

import "asterisk/adapters/rp"

// EnvelopeSource provides an envelope by launch ID (e.g. pre-investigation store).
type EnvelopeSource interface {
	Get(launchID int) (*rp.Envelope, error)
}
