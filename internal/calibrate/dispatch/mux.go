package dispatch

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
)

var (
	_ Dispatcher         = (*MuxDispatcher)(nil)
	_ ExternalDispatcher = (*MuxDispatcher)(nil)
)

// MuxDispatcher bridges the calibration runner (which calls Dispatch from
// potentially many goroutines) with an external agent (which calls
// GetNextStep / SubmitArtifact). Each Dispatch call gets a unique dispatch ID
// and its own response channel, so artifacts are routed to the correct caller
// even under high parallelism.
type MuxDispatcher struct {
	ctx     context.Context
	log     *slog.Logger
	mu      sync.Mutex
	nextID  int64
	pending map[int64]chan []byte
	closed  map[int64]struct{}

	promptCh chan DispatchContext
	abortCh  chan struct{}
	abortErr error
}

// NewMuxDispatcher creates a dispatcher with per-dispatch artifact routing.
// The provided context controls the dispatcher's lifetime.
func NewMuxDispatcher(ctx context.Context) *MuxDispatcher {
	return &MuxDispatcher{
		ctx:      ctx,
		log:      slog.Default(),
		pending:  make(map[int64]chan []byte),
		closed:   make(map[int64]struct{}),
		promptCh: make(chan DispatchContext),
		abortCh:  make(chan struct{}),
	}
}

// Dispatch assigns a unique dispatch ID, sends the prompt to the agent side,
// and blocks until the matching SubmitArtifact delivers the response.
// Satisfies the Dispatcher interface.
func (d *MuxDispatcher) Dispatch(ctx DispatchContext) ([]byte, error) {
	d.mu.Lock()
	d.nextID++
	id := d.nextID
	responseCh := make(chan []byte, 1)
	d.pending[id] = responseCh
	pendingCount := len(d.pending)
	d.mu.Unlock()

	ctx.DispatchID = id

	d.log.Debug("mux dispatch registered",
		slog.Int64("dispatch_id", id),
		slog.String("case_id", ctx.CaseID),
		slog.String("step", ctx.Step),
		slog.Int("pending_count", pendingCount),
	)

	// Send prompt to the agent side
	select {
	case d.promptCh <- ctx:
	case <-d.ctx.Done():
		d.removePending(id)
		d.log.Warn("mux dispatch cancelled while sending prompt",
			slog.String("case_id", ctx.CaseID),
			slog.String("step", ctx.Step),
			slog.Int64("dispatch_id", id),
		)
		return nil, fmt.Errorf("mux dispatch cancelled: %w", d.ctx.Err())
	case <-d.abortCh:
		d.removePending(id)
		d.log.Warn("mux dispatch aborted while sending prompt",
			slog.String("case_id", ctx.CaseID),
			slog.String("step", ctx.Step),
			slog.Int64("dispatch_id", id),
		)
		return nil, fmt.Errorf("mux dispatch aborted: %w", d.getAbortErr())
	}

	// Wait for the routed artifact
	select {
	case data, ok := <-responseCh:
		if !ok {
			return nil, fmt.Errorf("mux dispatch aborted: %w", d.getAbortErr())
		}
		d.log.Debug("mux dispatch completed",
			slog.Int64("dispatch_id", id),
			slog.String("case_id", ctx.CaseID),
			slog.String("step", ctx.Step),
			slog.Int("bytes", len(data)),
		)
		return data, nil
	case <-d.ctx.Done():
		d.removePending(id)
		d.log.Warn("mux dispatch cancelled while waiting for artifact",
			slog.String("case_id", ctx.CaseID),
			slog.String("step", ctx.Step),
			slog.Int64("dispatch_id", id),
		)
		return nil, fmt.Errorf("mux dispatch cancelled: %w", d.ctx.Err())
	case <-d.abortCh:
		d.removePending(id)
		return nil, fmt.Errorf("mux dispatch aborted: %w", d.getAbortErr())
	}
}

// GetNextStep blocks until the runner produces the next prompt context.
// Implements ExternalDispatcher.
func (d *MuxDispatcher) GetNextStep(ctx context.Context) (DispatchContext, error) {
	select {
	case <-ctx.Done():
		return DispatchContext{}, ctx.Err()
	case <-d.ctx.Done():
		return DispatchContext{}, fmt.Errorf("dispatcher shutdown: %w", d.ctx.Err())
	case dc, ok := <-d.promptCh:
		if !ok {
			return DispatchContext{}, fmt.Errorf("dispatcher closed")
		}
		return dc, nil
	}
}

// SubmitArtifact routes the artifact to the Dispatch call with the given ID.
// Implements ExternalDispatcher.
func (d *MuxDispatcher) SubmitArtifact(ctx context.Context, dispatchID int64, data []byte) error {
	d.mu.Lock()
	ch, ok := d.pending[dispatchID]
	if !ok {
		if _, wasClosed := d.closed[dispatchID]; wasClosed {
			d.mu.Unlock()
			d.log.Error("double submit detected",
				slog.Int64("dispatch_id", dispatchID),
			)
			return fmt.Errorf("dispatch_id %d already submitted", dispatchID)
		}
		pendingCount := len(d.pending)
		d.mu.Unlock()
		d.log.Warn("submit for unknown dispatch ID",
			slog.Int64("dispatch_id", dispatchID),
			slog.Int("active_dispatches", pendingCount),
		)
		return fmt.Errorf("unknown dispatch_id %d", dispatchID)
	}
	delete(d.pending, dispatchID)
	d.closed[dispatchID] = struct{}{}
	d.mu.Unlock()

	select {
	case ch <- data:
		d.log.Debug("mux artifact routed",
			slog.Int64("dispatch_id", dispatchID),
			slog.Int("bytes", len(data)),
		)
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-d.ctx.Done():
		return fmt.Errorf("dispatcher shutdown: %w", d.ctx.Err())
	}
}

// Abort broadcasts an error to all waiting Dispatch calls.
func (d *MuxDispatcher) Abort(err error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	select {
	case <-d.abortCh:
		return // already aborted
	default:
	}

	d.abortErr = err
	close(d.abortCh)
	d.log.Warn("mux dispatcher abort", slog.String("error", err.Error()))

	for id, ch := range d.pending {
		close(ch)
		delete(d.pending, id)
	}
}

// PromptCh returns the read-only prompt channel (for session integration).
func (d *MuxDispatcher) PromptCh() <-chan DispatchContext {
	return d.promptCh
}

func (d *MuxDispatcher) removePending(id int64) {
	d.mu.Lock()
	delete(d.pending, id)
	d.mu.Unlock()
}

func (d *MuxDispatcher) getAbortErr() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.abortErr != nil {
		return d.abortErr
	}
	return fmt.Errorf("aborted")
}
