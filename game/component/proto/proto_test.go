package proto

import (
	"core/models/enums"
	"encoding/json"
	"reflect"
	"testing"
)

func TestCardTypesAcceptLegacyClientValues(t *testing.T) {
	var rules GameRule
	if err := json.Unmarshal([]byte(`{"cardsType":{"MEINIU":1,"YOUNIU":2,"TONGHUANIU":null,"NIUNIU":false}}`), &rules); err != nil {
		t.Fatal(err)
	}
	if !rules.CardsType["MEINIU"] || !rules.CardsType["YOUNIU"] || rules.CardsType["TONGHUANIU"] || rules.CardsType["NIUNIU"] {
		t.Fatalf("unexpected normalized card types: %#v", rules.CardsType)
	}
}

func TestGameRulePreservesClientOptions(t *testing.T) {
	rule := GameRule{
		Hongtao10: true, Zhinengshunzi: true, XiaojuTrust: true,
		BeBankerScores: []int{50, 100, 200},
	}
	data, err := json.Marshal(rule)
	if err != nil {
		t.Fatal(err)
	}
	var decoded GameRule
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if !decoded.Hongtao10 || !decoded.Zhinengshunzi || !decoded.XiaojuTrust || len(decoded.BeBankerScores) != 3 {
		t.Fatalf("client options were not preserved: %#v", decoded)
	}
}

func TestGameRuleAcceptsClientPourOptions(t *testing.T) {
	var rule GameRule
	err := json.Unmarshal([]byte(`{"canPourScores":[1,3,5,10],"maxCanPourGold":20,"tuiScale":[6,20]}`), &rule)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(rule.CanPourScores, []int{1, 3, 5, 10}) || rule.MaxCanPourGold != 20 || !reflect.DeepEqual(rule.TuiScale, []int{6, 20}) {
		t.Fatalf("client pour options were not decoded: %#v", rule)
	}
}

func TestOneUserDiamondCountRejectsUnsupportedRules(t *testing.T) {
	if got := OneUserDiamondCount(999, enums.PDK); got != 0 {
		t.Fatalf("unsupported bureau cost = %d, want 0", got)
	}
	if got := OneUserDiamondCount(10, enums.GameType(999)); got != 0 {
		t.Fatalf("unsupported game cost = %d, want 0", got)
	}
}

func TestOneUserDiamondCountReturnsConfiguredCost(t *testing.T) {
	if got := OneUserDiamondCount(20, enums.PDK); got != 2 {
		t.Fatalf("pdk twenty-bureau cost = %d, want 2", got)
	}
}
