package rca

import (
	"fmt"
	"strings"

	"github.com/dpopsuev/origami/logging"
	"github.com/dpopsuev/origami/adapters/rp"
)

// ResolveRPCases fetches real failure data from ReportPortal for cases that
// have RPLaunchID set, updating their ErrorMessage and LogSnippet in place.
// Cases without RPLaunchID are left unchanged. Envelopes are cached by launch
// ID so multiple cases sharing a launch only trigger one API call.
func ResolveRPCases(fetcher rp.EnvelopeFetcher, scenario *Scenario) error {
	logger := logging.New("rp-source")
	cache := make(map[int]*rp.Envelope)

	for i := range scenario.Cases {
		c := &scenario.Cases[i]
		if c.RPLaunchID <= 0 {
			continue
		}

		env, ok := cache[c.RPLaunchID]
		if !ok {
			var err error
			env, err = fetcher.Fetch(c.RPLaunchID)
			if err != nil {
				return fmt.Errorf("fetch RP launch %d for case %s: %w", c.RPLaunchID, c.ID, err)
			}
			cache[c.RPLaunchID] = env
			logger.Info("fetched RP launch",
				"launch_id", c.RPLaunchID, "name", env.Name, "failures", len(env.FailureList))
		}

		item := matchFailureItem(env, c)
		if item == nil {
			return fmt.Errorf("case %s: no matching failure item in RP launch %d (test=%q, item_id=%d)",
				c.ID, c.RPLaunchID, c.TestName, c.RPItemID)
		}

		logger.Info("matched RP item", "case_id", c.ID, "item_id", item.ID, "item_name", item.Name)

		if item.Description != "" {
			c.ErrorMessage = item.Description
		}
		if c.LogSnippet == "" && item.IssueComment != "" {
			c.LogSnippet = item.IssueComment
		}
		c.RPIssueType = item.IssueType
		c.RPAutoAnalyzed = item.AutoAnalyzed
	}

	return nil
}

// matchFailureItem finds the FailureItem that corresponds to a GroundTruthCase.
// Matching priority: exact RPItemID > test_id tag in name > test name substring.
func matchFailureItem(env *rp.Envelope, c *GroundTruthCase) *rp.FailureItem {
	if c.RPItemID > 0 {
		for i := range env.FailureList {
			if env.FailureList[i].ID == c.RPItemID {
				return &env.FailureList[i]
			}
		}
	}

	if c.TestID != "" {
		tag := "test_id:" + c.TestID
		for i := range env.FailureList {
			if strings.Contains(env.FailureList[i].Name, tag) {
				return &env.FailureList[i]
			}
		}
	}

	testLower := strings.ToLower(c.TestName)
	if testLower != "" {
		for i := range env.FailureList {
			nameLower := strings.ToLower(env.FailureList[i].Name)
			if strings.Contains(nameLower, testLower) {
				return &env.FailureList[i]
			}
			if strings.Contains(testLower, nameLower) {
				return &env.FailureList[i]
			}
		}
	}

	return nil
}
