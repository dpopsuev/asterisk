package mcp_test

import (
	"context"
	"testing"
	"time"

	mcpserver "asterisk/internal/mcp"
)

func TestNewSession_StubAdapter_CompletesInstantly(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	sess, err := mcpserver.NewSession(ctx, mcpserver.StartCalibrationInput{
		Scenario: "ptp-mock",
		Adapter:  "stub",
	})
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}

	if sess.ID == "" {
		t.Fatal("expected non-empty session ID")
	}
	if sess.GetState() != mcpserver.StateRunning {
		t.Fatalf("expected StateRunning, got %s", sess.GetState())
	}
	if sess.TotalCases < 1 {
		t.Fatalf("expected at least 1 case, got %d", sess.TotalCases)
	}

	select {
	case <-sess.Done():
	case <-ctx.Done():
		t.Fatal("timed out waiting for session to complete")
	}

	if sess.Err() != nil {
		t.Fatalf("session error: %v", sess.Err())
	}

	report := sess.Report()
	if report == nil {
		t.Fatal("expected non-nil report")
	}
	if len(report.CaseResults) != sess.TotalCases {
		t.Errorf("expected %d case results, got %d", sess.TotalCases, len(report.CaseResults))
	}
	t.Logf("session %s completed: %d cases, scenario=%s", sess.ID, len(report.CaseResults), sess.Scenario)
}

func TestNewSession_GetNextStep_DoneImmediately(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	sess, err := mcpserver.NewSession(ctx, mcpserver.StartCalibrationInput{
		Scenario: "ptp-mock",
		Adapter:  "stub",
	})
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}

	// Wait for completion
	select {
	case <-sess.Done():
	case <-ctx.Done():
		t.Fatal("timed out")
	}

	_, done, err := sess.GetNextStep(ctx)
	if err != nil {
		t.Fatalf("GetNextStep: %v", err)
	}
	if !done {
		t.Fatal("expected done=true after stub completes")
	}
}

func TestNewSession_InvalidScenario(t *testing.T) {
	ctx := context.Background()
	_, err := mcpserver.NewSession(ctx, mcpserver.StartCalibrationInput{
		Scenario: "nonexistent",
		Adapter:  "stub",
	})
	if err == nil {
		t.Fatal("expected error for invalid scenario")
	}
}

func TestNewSession_RealIngest_VerifiedOnly(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	sess, err := mcpserver.NewSession(ctx, mcpserver.StartCalibrationInput{
		Scenario: "ptp-real-ingest",
		Adapter:  "stub",
	})
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}

	if sess.TotalCases != 18 {
		t.Fatalf("expected 18 verified cases, got %d", sess.TotalCases)
	}
	t.Logf("ptp-real-ingest: %d verified cases (candidates excluded from scoring)", sess.TotalCases)

	select {
	case <-sess.Done():
	case <-ctx.Done():
		t.Fatal("timed out")
	}

	if sess.Err() != nil {
		t.Fatalf("session error: %v", sess.Err())
	}

	report := sess.Report()
	if report == nil {
		t.Fatal("expected non-nil report")
	}
	if len(report.CaseResults) != 18 {
		t.Errorf("expected 18 case results (verified only), got %d", len(report.CaseResults))
	}
	if report.Dataset == nil {
		t.Fatal("expected non-nil dataset health")
	}
	if report.Dataset.VerifiedCount != 18 {
		t.Errorf("dataset.verified_count = %d, want 18", report.Dataset.VerifiedCount)
	}
	if report.Dataset.CandidateCount != 12 {
		t.Errorf("dataset.candidate_count = %d, want 12", report.Dataset.CandidateCount)
	}
	if len(report.Dataset.Candidates) != 12 {
		t.Errorf("dataset.candidates length = %d, want 12", len(report.Dataset.Candidates))
	}
}
