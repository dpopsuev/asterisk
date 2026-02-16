package preinvest

// FetchAndSave runs pre-investigation: fetch envelope from fetcher and save to store.
// Contract: .cursor/contracts/mock-pre-investigation.md
func FetchAndSave(f Fetcher, s Store, launchID int) error {
	env, err := f.Fetch(launchID)
	if err != nil {
		return err
	}
	return s.Save(launchID, env)
}
