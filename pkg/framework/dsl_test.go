package framework

import (
	"os"
	"testing"
)

func TestLoadPipeline_ValidYAML(t *testing.T) {
	data := []byte(`
pipeline: test-pipe
description: "A test pipeline"
nodes:
  - name: a
    element: fire
    family: start
  - name: b
    element: water
    family: finish
edges:
  - id: E1
    name: a-to-b
    from: a
    to: b
    condition: "always"
  - id: E2
    name: b-done
    from: b
    to: _done
    condition: "terminal"
start: a
done: _done
`)
	def, err := LoadPipeline(data)
	if err != nil {
		t.Fatalf("LoadPipeline: %v", err)
	}
	if def.Pipeline != "test-pipe" {
		t.Errorf("Pipeline = %q, want %q", def.Pipeline, "test-pipe")
	}
	if def.Description != "A test pipeline" {
		t.Errorf("Description = %q, want %q", def.Description, "A test pipeline")
	}
	if len(def.Nodes) != 2 {
		t.Errorf("len(Nodes) = %d, want 2", len(def.Nodes))
	}
	if len(def.Edges) != 2 {
		t.Errorf("len(Edges) = %d, want 2", len(def.Edges))
	}
	if def.Start != "a" {
		t.Errorf("Start = %q, want %q", def.Start, "a")
	}
	if def.Done != "_done" {
		t.Errorf("Done = %q, want %q", def.Done, "_done")
	}
}

func TestLoadPipeline_WithZones(t *testing.T) {
	data := []byte(`
pipeline: zoned
nodes:
  - name: x
    family: x
  - name: y
    family: y
zones:
  front:
    nodes: [x]
    element: fire
    stickiness: 2
  back:
    nodes: [y]
    element: water
edges:
  - id: E1
    name: x-to-y
    from: x
    to: y
  - id: E2
    name: y-done
    from: y
    to: _done
start: x
done: _done
`)
	def, err := LoadPipeline(data)
	if err != nil {
		t.Fatalf("LoadPipeline: %v", err)
	}
	if len(def.Zones) != 2 {
		t.Fatalf("len(Zones) = %d, want 2", len(def.Zones))
	}
	front := def.Zones["front"]
	if front.Stickiness != 2 {
		t.Errorf("front.Stickiness = %d, want 2", front.Stickiness)
	}
	if front.Element != "fire" {
		t.Errorf("front.Element = %q, want %q", front.Element, "fire")
	}
}

func TestLoadPipeline_InvalidYAML(t *testing.T) {
	data := []byte(`{invalid yaml: [`)
	_, err := LoadPipeline(data)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestValidate_Valid(t *testing.T) {
	def := &PipelineDef{
		Pipeline: "test",
		Nodes:    []NodeDef{{Name: "a"}, {Name: "b"}},
		Edges:    []EdgeDef{{ID: "E1", From: "a", To: "b"}, {ID: "E2", From: "b", To: "_done"}},
		Start:    "a",
		Done:     "_done",
	}
	if err := def.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}

func TestValidate_EmptyPipelineName(t *testing.T) {
	def := &PipelineDef{Nodes: []NodeDef{{Name: "a"}}, Edges: []EdgeDef{{ID: "E1", From: "a", To: "_done"}}, Start: "a", Done: "_done"}
	if err := def.Validate(); err == nil {
		t.Fatal("expected error for empty pipeline name")
	}
}

func TestValidate_MissingStartNode(t *testing.T) {
	def := &PipelineDef{
		Pipeline: "test",
		Nodes:    []NodeDef{{Name: "a"}},
		Edges:    []EdgeDef{{ID: "E1", From: "a", To: "_done"}},
		Start:    "nonexistent",
		Done:     "_done",
	}
	if err := def.Validate(); err == nil {
		t.Fatal("expected error for missing start node")
	}
}

func TestValidate_BrokenEdgeSource(t *testing.T) {
	def := &PipelineDef{
		Pipeline: "test",
		Nodes:    []NodeDef{{Name: "a"}},
		Edges:    []EdgeDef{{ID: "E1", From: "ghost", To: "a"}},
		Start:    "a",
		Done:     "_done",
	}
	err := def.Validate()
	if err == nil {
		t.Fatal("expected error for broken edge source")
	}
	if !contains(err.Error(), "ghost") {
		t.Errorf("error should name the invalid reference: %v", err)
	}
}

func TestValidate_BrokenEdgeTarget(t *testing.T) {
	def := &PipelineDef{
		Pipeline: "test",
		Nodes:    []NodeDef{{Name: "a"}},
		Edges:    []EdgeDef{{ID: "E1", From: "a", To: "ghost"}},
		Start:    "a",
		Done:     "_done",
	}
	err := def.Validate()
	if err == nil {
		t.Fatal("expected error for broken edge target")
	}
	if !contains(err.Error(), "ghost") {
		t.Errorf("error should name the invalid reference: %v", err)
	}
}

func TestValidate_BrokenZoneNode(t *testing.T) {
	def := &PipelineDef{
		Pipeline: "test",
		Nodes:    []NodeDef{{Name: "a"}},
		Edges:    []EdgeDef{{ID: "E1", From: "a", To: "_done"}},
		Zones:    map[string]ZoneDef{"z": {Nodes: []string{"ghost"}}},
		Start:    "a",
		Done:     "_done",
	}
	err := def.Validate()
	if err == nil {
		t.Fatal("expected error for broken zone node reference")
	}
	if !contains(err.Error(), "ghost") {
		t.Errorf("error should name the invalid reference: %v", err)
	}
}

func TestValidate_DuplicateNodeName(t *testing.T) {
	def := &PipelineDef{
		Pipeline: "test",
		Nodes:    []NodeDef{{Name: "a"}, {Name: "a"}},
		Edges:    []EdgeDef{{ID: "E1", From: "a", To: "_done"}},
		Start:    "a",
		Done:     "_done",
	}
	err := def.Validate()
	if err == nil {
		t.Fatal("expected error for duplicate node name")
	}
}

func TestValidate_DuplicateEdgeID(t *testing.T) {
	def := &PipelineDef{
		Pipeline: "test",
		Nodes:    []NodeDef{{Name: "a"}, {Name: "b"}},
		Edges:    []EdgeDef{{ID: "E1", From: "a", To: "b"}, {ID: "E1", From: "b", To: "_done"}},
		Start:    "a",
		Done:     "_done",
	}
	err := def.Validate()
	if err == nil {
		t.Fatal("expected error for duplicate edge id")
	}
}

func TestRoundTripFidelity(t *testing.T) {
	original := &PipelineDef{
		Pipeline:    "roundtrip",
		Description: "test round trip",
		Nodes:       []NodeDef{{Name: "a", Element: "fire", Family: "start"}, {Name: "b", Family: "end"}},
		Edges:       []EdgeDef{{ID: "E1", Name: "a-b", From: "a", To: "b"}, {ID: "E2", Name: "b-done", From: "b", To: "_done"}},
		Start:       "a",
		Done:        "_done",
	}

	data, err := original.MarshalYAML()
	if err != nil {
		t.Fatalf("MarshalYAML: %v", err)
	}

	restored, err := LoadPipeline(data)
	if err != nil {
		t.Fatalf("LoadPipeline round-trip: %v", err)
	}

	if restored.Pipeline != original.Pipeline {
		t.Errorf("Pipeline = %q, want %q", restored.Pipeline, original.Pipeline)
	}
	if len(restored.Nodes) != len(original.Nodes) {
		t.Errorf("len(Nodes) = %d, want %d", len(restored.Nodes), len(original.Nodes))
	}
	if len(restored.Edges) != len(original.Edges) {
		t.Errorf("len(Edges) = %d, want %d", len(restored.Edges), len(original.Edges))
	}
	if restored.Start != original.Start {
		t.Errorf("Start = %q, want %q", restored.Start, original.Start)
	}
	if restored.Done != original.Done {
		t.Errorf("Done = %q, want %q", restored.Done, original.Done)
	}
	for i, n := range restored.Nodes {
		if n.Name != original.Nodes[i].Name {
			t.Errorf("Node[%d].Name = %q, want %q", i, n.Name, original.Nodes[i].Name)
		}
	}
}

func TestLoadPipeline_RealF0F6(t *testing.T) {
	data, err := os.ReadFile("../../pipelines/rca-investigation.yaml")
	if err != nil {
		t.Fatalf("read rca-investigation.yaml: %v", err)
	}
	def, err := LoadPipeline(data)
	if err != nil {
		t.Fatalf("LoadPipeline: %v", err)
	}
	if err := def.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if def.Pipeline != "rca-investigation" {
		t.Errorf("Pipeline = %q, want %q", def.Pipeline, "rca-investigation")
	}
	if len(def.Nodes) != 7 {
		t.Errorf("len(Nodes) = %d, want 7", len(def.Nodes))
	}
	if len(def.Zones) != 3 {
		t.Errorf("len(Zones) = %d, want 3", len(def.Zones))
	}
}

func TestLoadPipeline_RealDefectCourt(t *testing.T) {
	data, err := os.ReadFile("../../pipelines/defect-court.yaml")
	if err != nil {
		t.Fatalf("read defect-court.yaml: %v", err)
	}
	def, err := LoadPipeline(data)
	if err != nil {
		t.Fatalf("LoadPipeline: %v", err)
	}
	if err := def.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if def.Pipeline != "defect-court" {
		t.Errorf("Pipeline = %q, want %q", def.Pipeline, "defect-court")
	}
	if len(def.Nodes) != 5 {
		t.Errorf("len(Nodes) = %d, want 5", len(def.Nodes))
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstr(s, substr))
}

func containsSubstr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
