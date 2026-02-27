package scenarios

import (
	"embed"
	"fmt"
	"sort"
	"strings"

	"asterisk/adapters/rca"

	"gopkg.in/yaml.v3"
)

//go:embed *.yaml
var scenarioFS embed.FS

// LoadScenario reads a scenario by name from the embedded YAML files.
func LoadScenario(name string) (*rca.Scenario, error) {
	data, err := scenarioFS.ReadFile(name + ".yaml")
	if err != nil {
		return nil, fmt.Errorf("scenario %q not found (available: %s): %w",
			name, strings.Join(ListScenarios(), ", "), err)
	}
	var s rca.Scenario
	if err := yaml.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parse scenario %q: %w", name, err)
	}
	return &s, nil
}

// ListScenarios returns the names of all embedded scenarios, sorted.
func ListScenarios() []string {
	entries, _ := scenarioFS.ReadDir(".")
	var names []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".yaml") {
			names = append(names, strings.TrimSuffix(e.Name(), ".yaml"))
		}
	}
	sort.Strings(names)
	return names
}
