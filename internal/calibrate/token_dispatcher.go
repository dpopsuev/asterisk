package calibrate

import (
	"os"
	"time"
)

// TokenTrackingDispatcher wraps any Dispatcher and records token usage
// for each dispatch call. It measures prompt file size, artifact response
// size, and wall-clock time per dispatch.
type TokenTrackingDispatcher struct {
	inner   Dispatcher
	tracker TokenTracker
}

// NewTokenTrackingDispatcher wraps a dispatcher with token tracking.
func NewTokenTrackingDispatcher(inner Dispatcher, tracker TokenTracker) *TokenTrackingDispatcher {
	return &TokenTrackingDispatcher{inner: inner, tracker: tracker}
}

// Dispatch delegates to the inner dispatcher while recording token metrics.
func (d *TokenTrackingDispatcher) Dispatch(ctx DispatchContext) ([]byte, error) {
	// Measure prompt size before dispatch
	promptBytes := 0
	if info, err := os.Stat(ctx.PromptPath); err == nil {
		promptBytes = int(info.Size())
	}

	start := time.Now()
	data, err := d.inner.Dispatch(ctx)
	elapsed := time.Since(start)

	if err != nil {
		return data, err
	}

	artifactBytes := len(data)

	d.tracker.Record(TokenRecord{
		CaseID:         ctx.CaseID,
		Step:           ctx.Step,
		PromptBytes:    promptBytes,
		ArtifactBytes:  artifactBytes,
		PromptTokens:   EstimateTokens(promptBytes),
		ArtifactTokens: EstimateTokens(artifactBytes),
		Timestamp:      start,
		WallClockMs:    elapsed.Milliseconds(),
	})

	return data, nil
}

// Inner returns the wrapped dispatcher for type-specific operations
// (e.g., calling FileDispatcher.MarkDone).
func (d *TokenTrackingDispatcher) Inner() Dispatcher {
	return d.inner
}
