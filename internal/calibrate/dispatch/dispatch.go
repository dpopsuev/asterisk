// Package dispatch provides transport mechanisms for delivering prompts to
// external agents and collecting their artifact responses. It implements the
// Strategy pattern: different dispatchers handle different communication
// channels (stdin, file polling, batch).
package dispatch

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"
)

// discardLogger returns a logger that discards all output.
func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// now is a helper for timestamps.
func now() string {
	return time.Now().UTC().Format(time.RFC3339)
}

// Dispatcher abstracts how a prompt is delivered to an external agent
// and how the resulting artifact is collected back.
type Dispatcher interface {
	// Dispatch delivers the prompt at PromptPath to the external agent and
	// blocks until the artifact appears at ArtifactPath.
	// Returns the raw artifact bytes or an error (e.g. timeout).
	Dispatch(ctx DispatchContext) ([]byte, error)
}

// DispatchContext carries all the metadata a dispatcher needs to deliver
// a prompt and collect an artifact.
type DispatchContext struct {
	DispatchID   int64  // unique ID assigned by the dispatcher for artifact routing
	CaseID       string // ground-truth case ID, e.g. "C1"
	Step         string // pipeline step name, e.g. "F0_RECALL"
	PromptPath   string // absolute path to the filled prompt file
	ArtifactPath string // absolute path where artifact JSON should appear
}

// ExternalDispatcher is the agent-facing side of a mux dispatcher.
// Any external agent (MCP server, CLI AI, HTTP API) uses this interface
// to pull pipeline steps and submit artifacts with correct routing.
type ExternalDispatcher interface {
	GetNextStep(ctx context.Context) (DispatchContext, error)
	SubmitArtifact(ctx context.Context, dispatchID int64, data []byte) error
}

// Finalizer is an optional interface for dispatchers that need post-dispatch
// cleanup (e.g. updating signal files). Adapters check for this interface
// instead of type-asserting specific dispatcher implementations.
type Finalizer interface {
	MarkDone(artifactPath string)
}

// Unwrapper is implemented by decorator dispatchers (e.g. TokenTrackingDispatcher)
// to expose the inner dispatcher for interface checks.
type Unwrapper interface {
	Inner() Dispatcher
}

// UnwrapFinalizer walks the dispatcher decorator chain and returns the first
// Finalizer found, or nil if none implements it.
func UnwrapFinalizer(d Dispatcher) Finalizer {
	for d != nil {
		if f, ok := d.(Finalizer); ok {
			return f
		}
		if u, ok := d.(Unwrapper); ok {
			d = u.Inner()
			continue
		}
		return nil
	}
	return nil
}

// --- StdinDispatcher (interactive, terminal-based) ---

// StdinDispatcher delivers prompts by printing a banner to stdout and
// blocking on stdin until the user presses Enter.
type StdinDispatcher struct {
	reader *bufio.Reader
}

// NewStdinDispatcher creates a dispatcher that reads from os.Stdin.
func NewStdinDispatcher() *StdinDispatcher {
	return &StdinDispatcher{reader: bufio.NewReader(os.Stdin)}
}

// Dispatch prints a banner with case/step/paths, blocks on stdin, then reads
// and validates the artifact file.
func (d *StdinDispatcher) Dispatch(ctx DispatchContext) ([]byte, error) {
	fmt.Println()
	fmt.Println("================================================================")
	fmt.Printf("  Case: %-6s  Step: %s\n", ctx.CaseID, ctx.Step)
	fmt.Println("================================================================")
	fmt.Printf("  Prompt:   %s\n", ctx.PromptPath)
	fmt.Printf("  Artifact: %s\n", ctx.ArtifactPath)
	fmt.Println("----------------------------------------------------------------")
	fmt.Println("  1. Open the prompt file and paste it into Cursor")
	fmt.Println("  2. Save Cursor's JSON response to the artifact path above")
	fmt.Println("  3. Press Enter to continue")
	fmt.Println("================================================================")
	fmt.Print("  > ")
	_, _ = d.reader.ReadString('\n')

	data, err := os.ReadFile(ctx.ArtifactPath)
	if err != nil {
		return nil, fmt.Errorf("artifact not found at %s: %w", ctx.ArtifactPath, err)
	}

	var raw json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("invalid JSON in %s: %w", ctx.ArtifactPath, err)
	}

	fmt.Printf("  Read artifact (%d bytes)\n", len(data))
	return raw, nil
}
