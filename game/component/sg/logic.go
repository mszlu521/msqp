package sg

import "sort"

const (
	Normal  CardType = 1
	SanGong CardType = 4
	BaoZi   CardType = 5
)

type CardType int

type Result struct {
	Type      CardType `json:"type"`
	Point     int      `json:"point"`
	GongCount int      `json:"gongCount"`
	Scale     int      `json:"scale"`
}

func Number(card int) int { return card & 0x0f }
func Value(card int) int {
	n := Number(card)
	if n >= 10 {
		return 0
	}
	return n
}
func Suit(card int) int { return card >> 4 }

func Evaluate(cards []int, bigScale bool) Result {
	if len(cards) != 3 {
		return Result{Type: Normal, Scale: 1}
	}
	numbers := []int{Number(cards[0]), Number(cards[1]), Number(cards[2])}
	same := numbers[0] == numbers[1] && numbers[1] == numbers[2]
	gong := 0
	for _, n := range numbers {
		if n > 10 {
			gong++
		}
	}
	point := (Value(cards[0]) + Value(cards[1]) + Value(cards[2])) % 10
	r := Result{Type: Normal, Point: point, GongCount: gong, Scale: 1}
	if same {
		r.Type = BaoZi
		if bigScale {
			r.Scale = 5
		}
		return r
	}
	if gong == 3 {
		r.Type = SanGong
		if bigScale {
			r.Scale = 4
		}
		return r
	}
	if bigScale && point == 9 {
		r.Scale = 3
	} else if bigScale && point == 8 {
		r.Scale = 2
	}
	return r
}

func Compare(left, right []int) int {
	a, b := Evaluate(left, false), Evaluate(right, false)
	if a.Type != b.Type {
		return int(a.Type - b.Type)
	}
	if a.Type == Normal && a.Point != b.Point {
		return a.Point - b.Point
	}
	if a.GongCount != b.GongCount {
		return a.GongCount - b.GongCount
	}
	la, lb := append([]int(nil), left...), append([]int(nil), right...)
	sort.Slice(la, func(i, j int) bool {
		if Number(la[i]) != Number(la[j]) {
			return Number(la[i]) > Number(la[j])
		}
		return Suit(la[i]) > Suit(la[j])
	})
	sort.Slice(lb, func(i, j int) bool {
		if Number(lb[i]) != Number(lb[j]) {
			return Number(lb[i]) > Number(lb[j])
		}
		return Suit(lb[i]) > Suit(lb[j])
	})
	for i := range la {
		if Number(la[i]) != Number(lb[i]) {
			return Number(la[i]) - Number(lb[i])
		}
		if Suit(la[i]) != Suit(lb[i]) {
			return Suit(la[i]) - Suit(lb[i])
		}
	}
	return 0
}

func Deck() []int {
	cards := make([]int, 0, 52)
	for suit := 0; suit < 4; suit++ {
		for n := 1; n <= 13; n++ {
			cards = append(cards, suit<<4|n)
		}
	}
	return cards
}
