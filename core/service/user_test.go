package service

import (
	"encoding/json"
	"framework/game"
	"testing"
)

func TestConfiguredStartGold(t *testing.T) {
	original := game.Conf
	t.Cleanup(func() { game.Conf = original })

	tests := []struct {
		name  string
		conf  *game.Config
		want  int64
		valid bool
	}{
		{name: "json number", conf: &game.Config{GameConfig: map[string]game.GameConfigValue{"startGold": {"value": float64(1000)}}}, want: 1000, valid: true},
		{name: "integer", conf: &game.Config{GameConfig: map[string]game.GameConfigValue{"startGold": {"value": int(42)}}}, want: 42, valid: true},
		{name: "json number type", conf: &game.Config{GameConfig: map[string]game.GameConfigValue{"startGold": {"value": json.Number("7")}}}, want: 7, valid: true},
		{name: "missing", conf: &game.Config{GameConfig: map[string]game.GameConfigValue{}}, valid: false},
		{name: "fraction", conf: &game.Config{GameConfig: map[string]game.GameConfigValue{"startGold": {"value": 1.5}}}, valid: false},
		{name: "negative", conf: &game.Config{GameConfig: map[string]game.GameConfigValue{"startGold": {"value": -1}}}, valid: false},
		{name: "int64 overflow", conf: &game.Config{GameConfig: map[string]game.GameConfigValue{"startGold": {"value": float64(uint64(1) << 63)}}}, valid: false},
		{name: "text", conf: &game.Config{GameConfig: map[string]game.GameConfigValue{"startGold": {"value": "100"}}}, valid: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			game.Conf = tt.conf
			got, err := configuredStartGold()
			if tt.valid {
				if err != nil || got != tt.want {
					t.Fatalf("configuredStartGold() = %d, %v; want %d, nil", got, err, tt.want)
				}
			} else if err == nil {
				t.Fatal("configuredStartGold() unexpectedly succeeded")
			}
		})
	}
	game.Conf = nil
	if _, err := configuredStartGold(); err == nil {
		t.Fatal("configuredStartGold() unexpectedly succeeded with nil config")
	}
}
