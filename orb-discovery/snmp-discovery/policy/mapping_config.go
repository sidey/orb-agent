package policy

import (
	"bytes"
	"embed"
	"fmt"
	"io"
	"os"
	"regexp"

	"go.yaml.in/yaml/v3"

	"github.com/netboxlabs/orb-agent/orb-discovery/snmp-discovery/config"
)

//go:embed mapping.yaml
var embeddedMapping embed.FS

type managerOptions struct {
	mappingFile string
}

// ManagerOption configures the policy manager before it starts any policies.
type ManagerOption func(*managerOptions)

// WithMappingFile replaces the embedded mapping with a YAML file read once at
// manager startup. A configured file must be valid; errors never fall back to
// the embedded mapping, which could query a different set of OIDs.
func WithMappingFile(filename string) ManagerOption {
	return func(options *managerOptions) {
		options.mappingFile = filename
	}
}

var mappingOIDPattern = regexp.MustCompile(`^\.[0-2](\.[0-9]+)+$`)

func loadMappingConfig(filename string) (config.Mapping, error) {
	var contents []byte
	var err error
	source := "embedded mapping.yaml"
	if filename == "" {
		contents, err = embeddedMapping.ReadFile("mapping.yaml")
	} else {
		source = filename
		contents, err = os.ReadFile(filename)
	}
	if err != nil {
		return config.Mapping{}, fmt.Errorf("failed to read mapping config %q: %w", source, err)
	}

	var result config.Mapping
	decoder := yaml.NewDecoder(bytes.NewReader(contents))
	decoder.KnownFields(true)
	if err := decoder.Decode(&result); err != nil {
		return config.Mapping{}, fmt.Errorf("failed to decode mapping config %q: %w", source, err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err != nil {
			return config.Mapping{}, fmt.Errorf("failed to decode mapping config %q: %w", source, err)
		}
		return config.Mapping{}, fmt.Errorf("mapping config %q must contain exactly one YAML document", source)
	}
	if len(result.Entries) == 0 {
		return config.Mapping{}, fmt.Errorf("mapping config %q must contain at least one entry", source)
	}
	if err := validateMappingEntries(result.Entries, "entries", make(map[string]string)); err != nil {
		return config.Mapping{}, fmt.Errorf("invalid mapping config %q: %w", source, err)
	}
	return result, nil
}

func validateMappingEntries(entries []config.MappingEntry, parent string, seen map[string]string) error {
	for i, entry := range entries {
		location := fmt.Sprintf("%s[%d]", parent, i)
		if !mappingOIDPattern.MatchString(entry.OID) {
			return fmt.Errorf("%s: oid must be a numeric OID with a leading dot, got %q", location, entry.OID)
		}
		if previous, ok := seen[entry.OID]; ok {
			return fmt.Errorf("%s: duplicate oid %q (already declared at %s)", location, entry.OID, previous)
		}
		seen[entry.OID] = location
		if entry.Entity == "" || entry.Field == "" {
			return fmt.Errorf("%s: entity and field are required", location)
		}
		if entry.IdentifierSize < 0 {
			return fmt.Errorf("%s: identifier_size must not be negative", location)
		}
		if err := validateMappingEntries(entry.MappingEntries, location+".mapping_entries", seen); err != nil {
			return err
		}
	}
	return nil
}
