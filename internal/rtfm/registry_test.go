package rtfm

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExtractVersion(t *testing.T) {
	tests := []struct {
		name         string
		caseVersion  string
		launchAttrs  map[string]string
		repoBranches []string
		want         string
	}{
		{
			name:        "case version takes priority",
			caseVersion: "4.21",
			launchAttrs: map[string]string{"ocp_version": "4.20"},
			want:        "4.21",
		},
		{
			name:        "strips v prefix",
			caseVersion: "v4.21",
			want:        "4.21",
		},
		{
			name:        "launch attrs fallback",
			launchAttrs: map[string]string{"ocp_version": "4.20"},
			want:        "4.20",
		},
		{
			name:        "launch attrs OCP_VERSION key",
			launchAttrs: map[string]string{"OCP_VERSION": "4.19"},
			want:        "4.19",
		},
		{
			name:         "branch parse fallback",
			repoBranches: []string{"master", "release-4.18", "main"},
			want:         "4.18",
		},
		{
			name:         "no version available",
			repoBranches: []string{"master", "main"},
			want:         "",
		},
		{
			name: "empty everything",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractVersion(tt.caseVersion, tt.launchAttrs, tt.repoBranches)
			if got != tt.want {
				t.Errorf("ExtractVersion() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestVersionedURL(t *testing.T) {
	got := VersionedURL("4.21")
	want := "https://docs.redhat.com/en/documentation/openshift_container_platform/4.21/"
	if got != want {
		t.Errorf("VersionedURL() = %q, want %q", got, want)
	}
}

func TestDocRegistry_Lookup(t *testing.T) {
	tmpDir := t.TempDir()
	docFile := filepath.Join(tmpDir, "ptp.md")
	if err := os.WriteFile(docFile, []byte("PTP architecture notes"), 0644); err != nil {
		t.Fatal(err)
	}

	reg := NewRegistry([]DocEntry{
		{
			Component: "ptp",
			DocPath:   "html/advanced_networking/using-ptp-hardware",
			LocalPath: docFile,
			Tags:      []string{"linuxptp-daemon", "cloud-event-proxy", "ptp-operator"},
		},
	})

	t.Run("matches by tag", func(t *testing.T) {
		result := reg.Lookup("cloud-event-proxy", nil, "4.21")
		if result == nil {
			t.Fatal("expected non-nil result")
		}
		if result.Version != "4.21" {
			t.Errorf("version = %q, want 4.21", result.Version)
		}
		wantURL := "https://docs.redhat.com/en/documentation/openshift_container_platform/4.21/html/advanced_networking/using-ptp-hardware"
		if result.DocURL != wantURL {
			t.Errorf("DocURL = %q, want %q", result.DocURL, wantURL)
		}
		if result.Architecture != "PTP architecture notes" {
			t.Errorf("Architecture = %q, want %q", result.Architecture, "PTP architecture notes")
		}
	})

	t.Run("matches by candidate repos", func(t *testing.T) {
		result := reg.Lookup("", []string{"ptp-operator", "cnf-gotests"}, "4.20")
		if result == nil {
			t.Fatal("expected non-nil result")
		}
		if result.Version != "4.20" {
			t.Errorf("version = %q, want 4.20", result.Version)
		}
	})

	t.Run("matches by component name", func(t *testing.T) {
		result := reg.Lookup("ptp", nil, "4.18")
		if result == nil {
			t.Fatal("expected non-nil result")
		}
	})

	t.Run("case insensitive matching", func(t *testing.T) {
		result := reg.Lookup("Cloud-Event-Proxy", nil, "4.21")
		if result == nil {
			t.Fatal("expected non-nil result for case-insensitive match")
		}
	})

	t.Run("no match returns nil", func(t *testing.T) {
		result := reg.Lookup("unknown-component", nil, "4.21")
		if result != nil {
			t.Errorf("expected nil for unknown component, got %+v", result)
		}
	})

	t.Run("empty version returns nil", func(t *testing.T) {
		result := reg.Lookup("ptp", nil, "")
		if result != nil {
			t.Errorf("expected nil for empty version, got %+v", result)
		}
	})

	t.Run("empty registry returns nil", func(t *testing.T) {
		empty := NewRegistry(nil)
		result := empty.Lookup("ptp", nil, "4.21")
		if result != nil {
			t.Errorf("expected nil for empty registry, got %+v", result)
		}
	})

	t.Run("missing local file is tolerated", func(t *testing.T) {
		reg := NewRegistry([]DocEntry{
			{
				Component: "ptp",
				DocPath:   "html/advanced_networking/using-ptp-hardware",
				LocalPath: "/nonexistent/file.md",
				Tags:      []string{"ptp-operator"},
			},
		})
		result := reg.Lookup("ptp-operator", nil, "4.21")
		if result == nil {
			t.Fatal("expected non-nil result even with missing file")
		}
		if result.Architecture != "" {
			t.Errorf("expected empty architecture for missing file, got %q", result.Architecture)
		}
	})
}

func TestParseVersionFromBranch(t *testing.T) {
	tests := []struct {
		branch string
		want   string
	}{
		{"release-4.18", "4.18"},
		{"release-4.21", "4.21"},
		{"master", ""},
		{"main", ""},
		{"v4.18", ""},
		{"release-4.18-rc1", "4.18"},
	}
	for _, tt := range tests {
		t.Run(tt.branch, func(t *testing.T) {
			got := parseVersionFromBranch(tt.branch)
			if got != tt.want {
				t.Errorf("parseVersionFromBranch(%q) = %q, want %q", tt.branch, got, tt.want)
			}
		})
	}
}
