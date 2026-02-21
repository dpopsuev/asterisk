package curate

import (
	"asterisk/internal/framework"
	"context"
	"fmt"
	"log/slog"
)

// CurationWalker is a framework.Walker that walks the curation pipeline.
// It uses configured Sources and Extractors to fetch raw data, extract
// fields, validate against a Schema, and promote complete records.
type CurationWalker struct {
	identity   framework.AgentIdentity
	state      *framework.WalkerState
	schema     Schema
	sources    []Source
	extractors []Extractor
	record     Record
	promoted   bool
}

// CurationWalkerConfig holds the configuration for constructing a CurationWalker.
type CurationWalkerConfig struct {
	RecordID   string
	Schema     Schema
	Sources    []Source
	Extractors []Extractor
	InitialRecord *Record
}

// NewCurationWalker creates a walker configured with sources, extractors,
// and a schema for validation.
func NewCurationWalker(cfg CurationWalkerConfig) *CurationWalker {
	r := NewRecord(cfg.RecordID)
	if cfg.InitialRecord != nil {
		r = *cfg.InitialRecord
	}

	return &CurationWalker{
		identity: framework.AgentIdentity{
			PersonaName: "curator",
			Alignment:   framework.AlignmentLight,
		},
		state:      framework.NewWalkerState(cfg.RecordID),
		schema:     cfg.Schema,
		sources:    cfg.Sources,
		extractors: cfg.Extractors,
		record:     r,
	}
}

func (w *CurationWalker) Identity() framework.AgentIdentity { return w.identity }
func (w *CurationWalker) State() *framework.WalkerState     { return w.state }

// Record returns the curated record after walking.
func (w *CurationWalker) Record() Record { return w.record }

// Promoted returns true if the record was promoted (all required fields present).
func (w *CurationWalker) Promoted() bool { return w.promoted }

// Handle processes each node in the curation pipeline, producing
// CurationArtifact outputs that the edge evaluators use for routing.
func (w *CurationWalker) Handle(ctx context.Context, node framework.Node, nc framework.NodeContext) (framework.Artifact, error) {
	switch node.Name() {
	case "fetch":
		return w.handleFetch(ctx)
	case "extract":
		return w.handleExtract(ctx, nc)
	case "validate":
		return w.handleValidate()
	case "enrich":
		return w.handleEnrich(ctx)
	case "promote":
		return w.handlePromote()
	default:
		return nil, fmt.Errorf("curate walker: unknown node %q", node.Name())
	}
}

func (w *CurationWalker) handleFetch(ctx context.Context) (framework.Artifact, error) {
	var lastRaw *RawEvidence
	for _, src := range w.sources {
		if !src.CanHandle(w.record.ID) {
			continue
		}
		raw, err := src.Fetch(ctx, w.record.ID)
		if err != nil {
			slog.Warn("source fetch failed",
				slog.String("source", src.Type()),
				slog.String("record", w.record.ID),
				slog.String("error", err.Error()),
			)
			continue
		}
		lastRaw = raw
		break
	}

	return &CurationArtifact{
		ArtifactType: "fetch",
		Rec:          &w.record,
		RawEvid:      lastRaw,
	}, nil
}

func (w *CurationWalker) handleExtract(ctx context.Context, nc framework.NodeContext) (framework.Artifact, error) {
	var raw *RawEvidence
	if prior, ok := nc.PriorArtifact.(*CurationArtifact); ok {
		raw = prior.RawEvid
	}
	if raw == nil {
		return &CurationArtifact{ArtifactType: "extract", Rec: &w.record}, nil
	}

	for _, ext := range w.extractors {
		fields, err := ext.Extract(ctx, raw)
		if err != nil {
			slog.Warn("extractor failed",
				slog.String("extractor", ext.Type()),
				slog.String("error", err.Error()),
			)
			continue
		}
		for _, f := range fields {
			w.record.Set(f)
		}
	}

	return &CurationArtifact{
		ArtifactType: "extract",
		Rec:          &w.record,
	}, nil
}

func (w *CurationWalker) handleValidate() (framework.Artifact, error) {
	cr := CheckCompleteness(w.record, w.schema)

	moreSources := false
	for _, src := range w.sources {
		if src.CanHandle(w.record.ID) {
			moreSources = true
			break
		}
	}

	return &CurationArtifact{
		ArtifactType: "validate",
		Rec:          &w.record,
		Complete:     cr.Promotable,
		MoreSources:  moreSources && !cr.Promotable,
		Conf:         cr.Score,
	}, nil
}

func (w *CurationWalker) handleEnrich(_ context.Context) (framework.Artifact, error) {
	cr := CheckCompleteness(w.record, w.schema)
	return &CurationArtifact{
		ArtifactType: "enrich",
		Rec:          &w.record,
		Complete:     cr.Promotable,
		Conf:         cr.Score,
	}, nil
}

func (w *CurationWalker) handlePromote() (framework.Artifact, error) {
	w.promoted = true
	slog.Info("record promoted",
		slog.String("record_id", w.record.ID),
		slog.Int("fields", len(w.record.Fields)),
	)
	return &CurationArtifact{
		ArtifactType: "promote",
		Rec:          &w.record,
		Complete:     true,
		Conf:         1.0,
	}, nil
}

// Verify compile-time interface compliance.
var _ framework.Walker = (*CurationWalker)(nil)
