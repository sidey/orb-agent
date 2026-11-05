package policy

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gosnmp/gosnmp"
	"github.com/netboxlabs/diode-sdk-go/diode"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/netboxlabs/orb-agent/orb-discovery/snmp-discovery/config"
	"github.com/netboxlabs/orb-agent/orb-discovery/snmp-discovery/mapping"
	"github.com/netboxlabs/orb-agent/orb-discovery/snmp-discovery/snmp"
)

const externalInterfaceMapping = `entries:
  - oid: ".1.3.6.1.4.1.99999.1"
    entity: "interface"
    field: "_id"
    identifier_size: 1
    mapping_entries:
      - oid: ".1.3.6.1.4.1.99999.1.2"
        entity: "interface"
        field: "name"
`

func writeMappingFile(t *testing.T, contents string) string {
	t.Helper()
	filename := filepath.Join(t.TempDir(), "custom mapping.yaml")
	require.NoError(t, os.WriteFile(filename, []byte(contents), 0o600))
	return filename
}

func newMappingManager(t *testing.T, options ...ManagerOption) (*Manager, error) {
	t.Helper()
	return NewManager(context.Background(), slog.New(slog.DiscardHandler), nil, nil, options...)
}

func TestManagerMappingDefaultsToEmbedded(t *testing.T) {
	manager, err := newMappingManager(t)
	require.NoError(t, err)
	entries, err := mapping.NewConfig(manager.mappingConfig.Entries, manager.logger, nil, nil, nil, config.Options{})
	require.NoError(t, err)
	assert.Contains(t, entries.GenericObjectIDs(), ".1.3.6.1.2.1.2.2.1.2")
	assert.Contains(t, entries.GenericObjectIDs(), ".1.3.6.1.2.1.1.5.0")
}

func TestManagerMappingFileReplacesEmbeddedAndIsReadAtStartup(t *testing.T) {
	filename := writeMappingFile(t, externalInterfaceMapping)
	first, err := newMappingManager(t, WithMappingFile(filename))
	require.NoError(t, err)
	require.Len(t, first.mappingConfig.Entries, 1)
	assert.Equal(t, ".1.3.6.1.4.1.99999.1", first.mappingConfig.Entries[0].OID)

	updated := strings.ReplaceAll(externalInterfaceMapping, ".99999.1", ".99999.2")
	require.NoError(t, os.WriteFile(filename, []byte(updated), 0o600))
	second, err := newMappingManager(t, WithMappingFile(filename))
	require.NoError(t, err)
	assert.Equal(t, ".1.3.6.1.4.1.99999.2", second.mappingConfig.Entries[0].OID)
	assert.Equal(t, ".1.3.6.1.4.1.99999.1", first.mappingConfig.Entries[0].OID,
		"running managers keep a consistent mapping until restarted")
}

func TestManagerMappingFileErrorsDoNotFallBack(t *testing.T) {
	tests := []struct {
		name     string
		contents string
		want     string
	}{
		{"invalid YAML", "entries: [", "failed to decode"},
		{"unknown key", "entires: []", "field entires not found"},
		{"unknown nested key", strings.ReplaceAll(externalInterfaceMapping, "field: \"name\"", "feild: \"name\""), "field feild not found"},
		{"empty file", "", "failed to decode"},
		{"empty entries", "entries: []", "at least one entry"},
		{"multiple documents", externalInterfaceMapping + "---\nentries: []\n", "exactly one YAML document"},
		{"invalid OID", strings.ReplaceAll(externalInterfaceMapping, ".99999.1.2", ".99999.invalid"), "numeric OID"},
		{"missing entity", strings.ReplaceAll(externalInterfaceMapping, "entity: \"interface\"", "entity: \"\""), "entity and field are required"},
		{"negative index size", strings.ReplaceAll(externalInterfaceMapping, "identifier_size: 1", "identifier_size: -1"), "identifier_size must not be negative"},
		{"duplicate OID", externalInterfaceMapping + "  - oid: \".1.3.6.1.4.1.99999.1.2\"\n    entity: interface\n    field: name\n", "duplicate oid"},
		{"invalid index kind", strings.ReplaceAll(externalInterfaceMapping, "identifier_size: 1", "index_kind: misspelled"), "invalid index_kind"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			filename := writeMappingFile(t, test.contents)
			manager, err := newMappingManager(t, WithMappingFile(filename))
			require.ErrorContains(t, err, test.want)
			assert.Nil(t, manager)
		})
	}
	for _, filename := range []string{filepath.Join(t.TempDir(), "missing.yaml"), t.TempDir()} {
		manager, err := newMappingManager(t, WithMappingFile(filename))
		require.ErrorContains(t, err, "failed to read mapping config")
		assert.Nil(t, manager)
	}
}

type mappingFileWalker struct {
	requested []string
}

func (*mappingFileWalker) Connect() error { return nil }
func (*mappingFileWalker) Close() error   { return nil }

func (w *mappingFileWalker) Walk(_ context.Context, oid string, identifierSize int) (map[string]snmp.PDU, error) {
	w.requested = append(w.requested, oid)
	name := oid + ".5"
	return map[string]snmp.PDU{name: {
		Name: name, Type: gosnmp.OctetString, Value: []byte("Ethernet1/5"), IdentifierSize: identifierSize,
	}}, nil
}

func TestMappingFileControlsWalkAndEntityTranslation(t *testing.T) {
	manager, err := newMappingManager(t, WithMappingFile(writeMappingFile(t, externalInterfaceMapping)))
	require.NoError(t, err)
	entries, err := mapping.NewConfig(manager.mappingConfig.Entries, manager.logger, nil, nil, nil, config.Options{})
	require.NoError(t, err)
	walker := &mappingFileWalker{}
	factory := func(string, uint16, int, time.Duration, *config.Authentication, *slog.Logger) (snmp.Walker, error) {
		return walker, nil
	}
	host := snmp.NewHost("192.0.2.1", 161, 0, time.Second, nil, manager.logger, factory)
	oids, err := host.Walk(context.Background(), entries.GenericObjectIDs())
	require.NoError(t, err)
	assert.Equal(t, []string{".1.3.6.1.4.1.99999.1.2"}, walker.requested,
		"the external mapping is a replacement, not merged with built-in OIDs")
	mapper := mapping.NewObjectIDMapper(entries, manager.logger, nil, "192.0.2.1")
	entities := mapper.MapObjectIDsToEntity(oids)
	require.Len(t, entities, 1)
	iface, ok := entities[0].(*diode.Interface)
	require.True(t, ok)
	require.NotNil(t, iface.Name)
	assert.Equal(t, "Ethernet1/5", *iface.Name)
}
