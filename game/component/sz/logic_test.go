package sz

import "testing"

func TestGetCardsDealsUpToThreeCardsWithoutPadding(t *testing.T) {
	logic := NewLogic()
	logic.cards = []int{0x01, 0x02}

	cards := logic.getCards()
	if len(cards) != 2 {
		t.Fatalf("getCards returned %d cards, want 2", len(cards))
	}
	for _, card := range cards {
		if card == 0 {
			t.Fatal("getCards must not pad a short deck with an invalid card")
		}
	}
}

func TestCompareCardsRejectsIncompleteHands(t *testing.T) {
	logic := NewLogic()
	if got := logic.CompareCards([]int{0x01, 0x02}, []int{0x03, 0x04, 0x05}); got != 0 {
		t.Fatalf("CompareCards returned %d for an incomplete hand, want 0", got)
	}
}

func TestWashCardsIsSafeBeforeConcurrentDeals(t *testing.T) {
	logic := NewLogic()
	logic.washCards()
	if got := len(logic.cards); got != 52 {
		t.Fatalf("washCards created %d cards, want 52", got)
	}
	if got := len(logic.getCards()); got != 3 {
		t.Fatalf("getCards returned %d cards after wash, want 3", got)
	}
}
