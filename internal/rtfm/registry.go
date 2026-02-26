// Package rtfm implements deterministic domain documentation lookup
// for the F1B_CONTEXT pipeline step. It resolves versioned Red Hat
// documentation URLs and reads pre-cached architecture notes so the
// F2_RESOLVE prompt can make informed repo selection decisions.
package rtfm

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"asterisk/internal/orchestrate"
)

const baseURL = "https://docs.redhat.com/en/documentation/openshift_container_platform"

// DocEntry maps a domain component to its documentation location.
type DocEntry struct {
	Component string   // component name or domain-wide label (e.g., "ptp")
	DocPath   string   // path suffix appended to the versioned base URL
	LocalPath string   // absolute path to cached markdown on disk
	Tags      []string // terms that trigger this entry (e.g., "linuxptp-daemon", "cloud-event-proxy")
}

// DocRegistry holds documentation entries and resolves them by component and version.
type DocRegistry struct {
	Entries []DocEntry
}

// NewRegistry creates a DocRegistry from the given entries.
func NewRegistry(entries []DocEntry) *DocRegistry {
	return &DocRegistry{Entries: entries}
}

// Lookup finds matching doc entries for a component (or any of its candidate repos),
// reads the cached documentation, and returns a ContextResult with the versioned URL.
// Returns nil (no error) when no entries match — the RTFM step is best-effort.
func (r *DocRegistry) Lookup(component string, candidateRepos []string, version string) *orchestrate.ContextResult {
	if len(r.Entries) == 0 || version == "" {
		return nil
	}

	terms := buildSearchTerms(component, candidateRepos)

	var matched []DocEntry
	for _, e := range r.Entries {
		if matchesAny(e, terms) {
			matched = append(matched, e)
		}
	}
	if len(matched) == 0 {
		return nil
	}

	result := &orchestrate.ContextResult{
		Version: version,
	}

	for _, m := range matched {
		if result.DocURL == "" && m.DocPath != "" {
			result.DocURL = fmt.Sprintf("%s/%s/%s", baseURL, version, m.DocPath)
		}
		if m.LocalPath != "" {
			content, err := os.ReadFile(m.LocalPath)
			if err == nil && len(content) > 0 {
				if result.Architecture != "" {
					result.Architecture += "\n\n"
				}
				result.Architecture += string(content)
			}
		}
	}

	return result
}

func buildSearchTerms(component string, candidateRepos []string) []string {
	seen := map[string]bool{}
	var terms []string
	add := func(s string) {
		lower := strings.ToLower(s)
		if lower != "" && !seen[lower] {
			seen[lower] = true
			terms = append(terms, lower)
		}
	}
	add(component)
	for _, r := range candidateRepos {
		add(r)
	}
	return terms
}

func matchesAny(entry DocEntry, terms []string) bool {
	for _, tag := range entry.Tags {
		tagLower := strings.ToLower(tag)
		for _, term := range terms {
			if tagLower == term {
				return true
			}
		}
	}
	compLower := strings.ToLower(entry.Component)
	for _, term := range terms {
		if compLower == term {
			return true
		}
	}
	return false
}

// ExtractVersion extracts an OCP version string from available context.
// Priority: explicit caseVersion > launch attributes > repo branch parse.
func ExtractVersion(caseVersion string, launchAttrs map[string]string, repoBranches []string) string {
	if caseVersion != "" {
		return normalizeVersion(caseVersion)
	}

	for _, key := range []string{"ocp_version", "OCP_VERSION", "version"} {
		if v, ok := launchAttrs[key]; ok && v != "" {
			return normalizeVersion(v)
		}
	}

	for _, branch := range repoBranches {
		if v := parseVersionFromBranch(branch); v != "" {
			return v
		}
	}

	return ""
}

var branchVersionRe = regexp.MustCompile(`release-(\d+\.\d+)`)

func parseVersionFromBranch(branch string) string {
	m := branchVersionRe.FindStringSubmatch(branch)
	if len(m) >= 2 {
		return m[1]
	}
	return ""
}

func normalizeVersion(v string) string {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "v")
	return v
}

// VersionedURL constructs the full Red Hat documentation URL for a given version.
func VersionedURL(version string) string {
	return fmt.Sprintf("%s/%s/", baseURL, version)
}
