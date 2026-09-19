package nn

import "sort"

type CardType int

const (
	NoNiu CardType = iota + 1
	YouNiu
	NiuNiu
	ShunZiNiu
	YinNiu
	TongHuaNiu
	WuHuaNiu
	HuLuNiu
	WuXiaoNiu
	ZhaDanNiu
	YiTiaoLong
	TongHuaShun
)

type ScaleType int

const (
	ScaleBig ScaleType = iota + 1
	ScaleLittle
	ScaleDianZi
)

type Rule struct {
	ScaleType   ScaleType
	ShunZiNiu   bool
	YinNiu      bool
	TongHuaNiu  bool
	WuHuaNiu    bool
	HuLuNiu     bool
	WuXiaoNiu   bool
	ZhaDanNiu   bool
	YiTiaoLong  bool
	TongHuaShun bool
}

type Result struct {
	Type     CardType
	Niu      int
	Scale    int
	HighCard int
}

func CardNumber(card int) int { return card & 0x0f }

func CardValue(card int) int {
	value := CardNumber(card)
	if value > 10 {
		return 10
	}
	return value
}

func CardSuit(card int) int { return card >> 4 }

func NiuNumber(cards []int) int {
	if len(cards) != 5 {
		return 0
	}
	sum := 0
	for _, card := range cards {
		sum += CardValue(card)
	}
	for i := 0; i < len(cards); i++ {
		for j := i + 1; j < len(cards); j++ {
			rest := sum - CardValue(cards[i]) - CardValue(cards[j])
			if rest%10 == 0 {
				niu := (sum - rest) % 10
				if niu == 0 {
					return 10
				}
				return niu
			}
		}
	}
	return 0
}

func Evaluate(cards []int, rule Rule) Result {
	if len(cards) != 5 {
		return Result{Type: NoNiu, Scale: 1}
	}
	copyCards := append([]int(nil), cards...)
	numbers := make([]int, len(copyCards))
	sum := 0
	counts := make(map[int]int)
	sameSuit := true
	for i, card := range copyCards {
		numbers[i] = CardNumber(card)
		sum += CardValue(card)
		counts[numbers[i]]++
		if i > 0 && CardSuit(card) != CardSuit(copyCards[0]) {
			sameSuit = false
		}
	}
	sort.Ints(numbers)
	straight := isStraight(numbers)
	niu := NiuNumber(copyCards)
	high := highCard(copyCards)
	result := Result{Type: NoNiu, Niu: niu, Scale: 1, HighCard: high}

	if straight && sameSuit && rule.TongHuaShun {
		return withScale(result, TongHuaShun, niu, rule)
	}
	if straight && numbers[0] == 1 && numbers[1] == 2 && rule.YiTiaoLong {
		return withScale(result, YiTiaoLong, niu, rule)
	}
	if hasCount(counts, 4) && rule.ZhaDanNiu {
		return withScale(result, ZhaDanNiu, niu, rule)
	}
	if allSmall(numbers) && sum < 10 && rule.WuXiaoNiu {
		return withScale(result, WuXiaoNiu, niu, rule)
	}
	if hasCount(counts, 3) && hasCount(counts, 2) && rule.HuLuNiu {
		return withScale(result, HuLuNiu, niu, rule)
	}
	if allFace(numbers) && rule.WuHuaNiu {
		return withScale(result, WuHuaNiu, niu, rule)
	}
	if sameSuit && rule.TongHuaNiu {
		return withScale(result, TongHuaNiu, niu, rule)
	}
	if allAtLeastTen(numbers) && rule.YinNiu {
		return withScale(result, YinNiu, niu, rule)
	}
	if straight && rule.ShunZiNiu {
		return withScale(result, ShunZiNiu, niu, rule)
	}
	if niu == 10 {
		result.Type = NiuNiu
	} else if niu > 0 {
		result.Type = YouNiu
	}
	result.Scale = scaleFor(result.Type, niu, rule)
	return result
}

func Compare(left, right []int, rule Rule) int {
	a, b := Evaluate(left, rule), Evaluate(right, rule)
	if a.Type != b.Type {
		return int(a.Type - b.Type)
	}
	if a.Type == YouNiu && a.Niu != b.Niu {
		return a.Niu - b.Niu
	}
	if a.HighCard != b.HighCard {
		return a.HighCard - b.HighCard
	}
	return highSuit(left) - highSuit(right)
}

func withScale(result Result, typ CardType, niu int, rule Rule) Result {
	result.Type, result.Niu = typ, niu
	result.Scale = scaleFor(typ, niu, rule)
	return result
}

func scaleFor(typ CardType, niu int, rule Rule) int {
	if typ == YouNiu || typ == NiuNiu {
		switch rule.ScaleType {
		case ScaleBig:
			return []int{1, 1, 1, 1, 1, 1, 1, 2, 2, 3, 4}[niu]
		case ScaleLittle:
			return []int{1, 1, 1, 1, 1, 1, 1, 1, 2, 2, 3}[niu]
		case ScaleDianZi:
			return []int{1, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10}[niu]
		}
	}
	values := map[CardType]int{NoNiu: 1, YouNiu: 1, NiuNiu: 3, ShunZiNiu: 4, YinNiu: 4, TongHuaNiu: 6, WuHuaNiu: 6, HuLuNiu: 6, WuXiaoNiu: 7, ZhaDanNiu: 8, YiTiaoLong: 9, TongHuaShun: 10}
	if rule.ScaleType == ScaleDianZi {
		values[ShunZiNiu], values[YinNiu] = 11, 11
		values[TongHuaNiu], values[WuHuaNiu] = 11, 11
		values[HuLuNiu], values[WuXiaoNiu] = 12, 13
		values[ZhaDanNiu], values[YiTiaoLong] = 14, 15
		values[TongHuaShun] = 16
	}
	return values[typ]
}

func isStraight(numbers []int) bool {
	if len(numbers) != 5 {
		return false
	}
	if numbers[0] == 1 && numbers[1] == 10 && numbers[2] == 11 && numbers[3] == 12 && numbers[4] == 13 {
		return true
	}
	for i := 1; i < len(numbers); i++ {
		if numbers[i] != numbers[i-1]+1 {
			return false
		}
	}
	return true
}

func hasCount(counts map[int]int, wanted int) bool {
	for _, count := range counts {
		if count == wanted {
			return true
		}
	}
	return false
}
func allSmall(numbers []int) bool {
	for _, value := range numbers {
		if value >= 5 {
			return false
		}
	}
	return true
}
func allFace(numbers []int) bool {
	for _, value := range numbers {
		if value <= 10 {
			return false
		}
	}
	return true
}
func allAtLeastTen(numbers []int) bool {
	for _, value := range numbers {
		if value < 10 {
			return false
		}
	}
	return true
}
func highCard(cards []int) int {
	high := 0
	for _, card := range cards {
		if CardNumber(card) > CardNumber(high) || (CardNumber(card) == CardNumber(high) && CardSuit(card) > CardSuit(high)) {
			high = card
		}
	}
	return CardNumber(high)
}
func highSuit(cards []int) int {
	high := 0
	for _, card := range cards {
		if CardNumber(card) > CardNumber(high) || (CardNumber(card) == CardNumber(high) && CardSuit(card) > CardSuit(high)) {
			high = card
		}
	}
	return CardSuit(high)
}
