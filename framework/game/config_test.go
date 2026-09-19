package game

import "testing"

func TestGetFrontGameConfigIgnoresOnlyExplicitBackend(t *testing.T) {
	config := &Config{GameConfig: map[string]GameConfigValue{
		"visible": {"value": 1},
		"hidden":  {"value": 2, "backend": true},
		"invalid": {"value": 3, "backend": "true"},
	}}
	got := config.GetFrontGameConfig()
	if _, ok := got["hidden"]; ok {
		t.Fatal("backend config was exposed")
	}
	if got["visible"] != 1 || got["invalid"] != 3 {
		t.Fatalf("unexpected front config: %#v", got)
	}
}

func TestConfigQueriesAreNilSafe(t *testing.T) {
	var config *Config
	if config.GetConnector("connector001") != nil {
		t.Fatal("nil config returned a connector")
	}
	if config.GetConnectorByServerType("connector") != nil {
		t.Fatal("nil config returned a connector by type")
	}
	if got := config.GetFrontGameConfig(); got == nil || len(got) != 0 {
		t.Fatalf("nil config front config = %#v, want empty map", got)
	}
	config = &Config{ServersConf: ServersConf{Connector: []*ConnectorConfig{nil, {ID: "connector001", ServerType: "connector"}}}}
	if config.GetConnector("connector001") == nil || config.GetConnectorByServerType("unknown") != nil {
		t.Fatal("connector queries mishandled nil entries")
	}
}
