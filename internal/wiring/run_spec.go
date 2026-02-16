package wiring

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	"asterisk/internal/postinvest"
	"asterisk/internal/preinvest"
)

var _ = ginkgo.Describe("Run", func() {
	ginkgo.It("stores envelope, writes artifact, and records push", func() {
		launchID := 33195
		env := loadFixtureEnvelopeGinkgo()
		gomega.Expect(env).NotTo(gomega.BeNil(), "fixture envelope should load")

		fetcher := preinvest.NewStubFetcher(env)
		envelopeStore := preinvest.NewMemStore()
		dir := ginkgo.GinkgoT().TempDir()
		artifactPath := filepath.Join(dir, "artifact.json")
		pushStore := postinvest.NewMemPushStore()

		err := Run(fetcher, envelopeStore, launchID, artifactPath, pushStore, "PROJ-456", "https://jira.example.com/PROJ-456")
		gomega.Expect(err).To(gomega.Succeed())

		gotEnv, err := envelopeStore.Get(launchID)
		gomega.Expect(err).To(gomega.Succeed())
		gomega.Expect(gotEnv).NotTo(gomega.BeNil())
		gomega.Expect(gotEnv.RunID).To(gomega.Equal(env.RunID))

		data, err := os.ReadFile(artifactPath)
		gomega.Expect(err).To(gomega.Succeed())
		var a struct {
			LaunchID   string `json:"launch_id"`
			CaseIDs    []int  `json:"case_ids"`
			DefectType string `json:"defect_type"`
		}
		gomega.Expect(json.Unmarshal(data, &a)).To(gomega.Succeed())
		gomega.Expect(a.LaunchID).To(gomega.Equal(env.RunID))

		gotPush := pushStore.LastPushed()
		gomega.Expect(gotPush).NotTo(gomega.BeNil())
		gomega.Expect(gotPush.DefectType).NotTo(gomega.BeEmpty())
		gomega.Expect(gotPush.JiraTicketID).To(gomega.Equal("PROJ-456"))
	})
})

func loadFixtureEnvelopeGinkgo() *preinvest.Envelope {
	path := filepath.Join("..", "..", "examples", "pre-investigation-33195-4.21", "envelope_33195_4.21.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var env preinvest.Envelope
	if err := json.Unmarshal(data, &env); err != nil {
		return nil
	}
	return &env
}
