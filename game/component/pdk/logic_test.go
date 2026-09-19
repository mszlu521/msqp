package pdk

import (
	"game/component/proto"
	"testing"
)

func TestType(t *testing.T) {
	tests := []struct {
		name  string
		cards []int
		rule  Rule
		want  CardType
	}{
		{"single", []int{0x03}, Rule{}, Single},
		{"double", []int{0x03, 0x13}, Rule{}, Double},
		{"straight", []int{0x03, 0x04, 0x05, 0x06, 0x07}, Rule{}, SingleLine},
		{"straight cannot contain two", []int{0x0d, 0x02, 0x03, 0x04, 0x05}, Rule{}, Error},
		{"double straight", []int{0x03, 0x13, 0x04, 0x14}, Rule{}, DoubleLine},
		{"triple with one", []int{0x03, 0x13, 0x23, 0x04}, Rule{}, ThreeLineTakeOne},
		{"bomb", []int{0x03, 0x13, 0x23, 0x33}, Rule{}, Bomb},
		{"four take two", []int{0x03, 0x13, 0x23, 0x33, 0x04, 0x14}, Rule{FourTakeTwo: true}, FourLineTakeX},
		{"four take two singles", []int{0x03, 0x13, 0x23, 0x33, 0x04, 0x05}, Rule{FourTakeTwo: true}, FourLineTakeX},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := Type(test.cards, test.rule); got != test.want {
				t.Fatalf("Type(%v) = %v, want %v", test.cards, got, test.want)
			}
		})
	}
}

func TestRestTypeMatchesLegacyShortAttachments(t *testing.T) {
	cards := []int{0x03, 0x13, 0x23, 0x04, 0x14, 0x24, 0x05}
	if Type(cards, Rule{}) != Error {
		t.Fatal("ordinary type must still reject an incomplete airplane")
	}
	if RestType(cards, Rule{}) != ThreeLineTakeTwo {
		t.Fatalf("RestType(%v) = %v, want %v", cards, RestType(cards, Rule{}), ThreeLineTakeTwo)
	}
	if RestType([]int{0x03, 0x13, 0x23, 0x04}, Rule{}) != ThreeLineTakeOne {
		t.Fatal("complete triple-with-one changed under remaining-hand rules")
	}
}

func TestFirstRequiredCardMatchesLegacySearchOrder(t *testing.T) {
	if card, ok := firstRequiredCard([]int{0x33, 0x23, 0x13, 0x03}); !ok || card != 0x33 {
		t.Fatalf("firstRequiredCard preferred %#x, %v", card, ok)
	}
	if card, ok := firstRequiredCard([]int{0x04, 0x24, 0x13}); !ok || card != 0x13 {
		t.Fatalf("firstRequiredCard fallback = %#x, %v", card, ok)
	}
}

func TestCompare(t *testing.T) {
	if !Compare([]int{0x03}, []int{0x04}, Rule{}, false, false) {
		t.Fatal("higher single should win")
	}
	if !Compare([]int{0x03, 0x13}, []int{0x04, 0x14}, Rule{}, false, false) {
		t.Fatal("higher pair should win")
	}
	if !Compare([]int{0x03}, []int{0x04, 0x14, 0x24, 0x34}, Rule{}, false, false) {
		t.Fatal("bomb should beat a non-bomb")
	}
	if Compare([]int{0x03, 0x13}, []int{0x04}, Rule{}, false, false) {
		t.Fatal("different non-bomb types should not compare")
	}
}

func TestDeckSupportsPlayerModes(t *testing.T) {
	for _, test := range []struct {
		frame int
		want  int
	}{
		{frame: 1, want: 48},
		{frame: 2, want: 45},
	} {
		cards := deck(test.frame)
		if len(cards) != test.want {
			t.Fatalf("deck(%d) has %d cards, want %d", test.frame, len(cards), test.want)
		}
		seen := make(map[int]bool, len(cards))
		for _, card := range cards {
			if seen[card] {
				t.Fatalf("deck(%d) contains duplicate card %#x", test.frame, card)
			}
			seen[card] = true
		}
	}
}

func TestDeckOrderCanBeShuffled(t *testing.T) {
	first := deck(1)
	second := deck(1)
	for i := len(second) - 1; i > 0; i-- {
		j := (i * 17) % (i + 1)
		second[i], second[j] = second[j], second[i]
	}
	same := true
	for i := range first {
		if first[i] != second[i] {
			same = false
			break
		}
	}
	if same {
		t.Fatal("a shuffled deck should not retain its original order")
	}
}

func TestBichuCanBeatWithBomb(t *testing.T) {
	frame := &GameFrame{rule: proto.GameRule{Baiwei: false}}
	if !frame.canBeat([]int{0x03, 0x13, 0x23, 0x33}, []int{0x04}, Rule{}) {
		t.Fatal("a bomb should count as a playable response to a single")
	}
}

func TestFirstCardMustContainBlackThree(t *testing.T) {
	if !containsCards([]int{0x33, 0x04}, []int{0x33}) {
		t.Fatal("black three should be found in the first hand")
	}
	if containsCards([]int{0x04, 0x05}, []int{0x33}) {
		t.Fatal("a hand without black three must not satisfy the rule")
	}
}

func TestCardGroups(t *testing.T) {
	groups := cardGroups([]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11})
	if len(groups) != 2 || len(groups[0]) != 10 || len(groups[1]) != 1 {
		t.Fatalf("unexpected card groups: %#v", groups)
	}
}

func TestResultPushUsesClientShape(t *testing.T) {
	result := gameResultPush([][]int{{1}, {}}, 0, []string{"a", ""}, [][][]int{{{1}}, {}}, []int{2, -2}, []int{1, 0}, true)
	data := result.(map[string]any)["data"].(map[string]any)
	if data["winArr"].([]int)[0] != 2 || data["hasChuntian"] != true {
		t.Fatalf("unexpected result data: %#v", data)
	}
	ids := data["idArr"].([]any)
	if ids[1] != nil {
		t.Fatalf("empty seat must be JSON null, got %#v", ids[1])
	}
}
