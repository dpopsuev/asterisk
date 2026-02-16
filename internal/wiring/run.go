package wiring

import (
	"asterisk/internal/postinvest"
	"asterisk/internal/preinvest"

	"asterisk/internal/investigate"
)

// Run executes the full mock flow: FetchAndSave → Analyze → Push.
// Fetcher and envelopeStore are used for pre-investigation; envelopeStore is also the envelope source for Analyze.
// artifactPath is where the investigation artifact is written and then read for push.
// pushStore records the push result (defect type, optional Jira fields).
// Contract: .cursor/contracts/mock-wiring.md
func Run(
	fetcher preinvest.Fetcher,
	envelopeStore *preinvest.MemStore,
	launchID int,
	artifactPath string,
	pushStore postinvest.PushStore,
	jiraTicketID, jiraLink string,
) error {
	if err := preinvest.FetchAndSave(fetcher, envelopeStore, launchID); err != nil {
		return err
	}
	if err := investigate.Analyze(envelopeStore, launchID, artifactPath); err != nil {
		return err
	}
	return postinvest.Push(artifactPath, pushStore, jiraTicketID, jiraLink)
}
