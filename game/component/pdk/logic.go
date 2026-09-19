package pdk

import "sort"

type CardType int

const (
	Error CardType = iota
	Single
	Double
	SingleLine
	DoubleLine
	ThreeLine
	ThreeLineTakeOne
	ThreeLineTakeTwo
	FourLineTakeX
	Bomb
)

// Rule contains the optional card-shape rules used by the client.
type Rule struct {
	ThreeABomb    bool
	FourTakeTwo   bool
	FourTakeThree bool
}

func CardValue(card int) int { return card & 0x0f }

func CardLogicValue(card int) int {
	value := CardValue(card)
	if value <= 0 || value > 0x0f {
		return 0
	}
	if value <= 2 {
		return value + 13
	}
	return value
}

func SortCards(cards []int) []int {
	result := append([]int(nil), cards...)
	sort.Slice(result, func(i, j int) bool {
		left, right := CardLogicValue(result[i]), CardLogicValue(result[j])
		if left != right {
			return left > right
		}
		return result[i] > result[j]
	})
	return result
}

func counts(cards []int) (map[int]int, bool) {
	result := make(map[int]int)
	for _, card := range cards {
		value := CardLogicValue(card)
		if value == 0 {
			return nil, false
		}
		result[value]++
	}
	return result, true
}

func consecutiveGroups(counts map[int]int, required int) (int, int, bool) {
	return consecutiveGroupsThrough(counts, required, 14)
}

func consecutiveGroupsThrough(counts map[int]int, required, maximum int) (int, int, bool) {
	bestCount, bestHigh := 0, 0
	for high := maximum; high >= 3; high-- {
		if counts[high] < required {
			continue
		}
		length := 1
		for high-length >= 3 && counts[high-length] >= required {
			length++
		}
		if length > bestCount {
			bestCount, bestHigh = length, high
		}
	}
	return bestCount, bestHigh, bestCount > 0
}

func Type(cards []int, rule Rule) CardType {
	if len(cards) == 0 {
		return Error
	}
	ordered := SortCards(cards)
	cardCounts, valid := counts(ordered)
	if !valid {
		return Error
	}
	if len(ordered) == 1 {
		return Single
	}
	if len(ordered) == 2 {
		if ordered[0]&0x0f == ordered[1]&0x0f {
			return Double
		}
		return Error
	}

	fourCount := cardCountsWithCount(cardCounts, 4)
	if fourCount > 0 {
		if fourCount == 1 && len(ordered) == 4 {
			return Bomb
		}
		if fourCount == 1 && len(ordered) == 6 && rule.FourTakeTwo {
			return FourLineTakeX
		}
		if fourCount == 1 && len(ordered) == 7 && rule.FourTakeThree {
			return FourLineTakeX
		}
		return Error
	}
	if len(ordered) == 3 && CardValue(ordered[0]) == 1 && rule.ThreeABomb {
		return Bomb
	}

	if tripleCount := cardCountsWithCount(cardCounts, 3); tripleCount > 0 {
		lineLength, _, _ := consecutiveGroups(cardCounts, 3)
		if lineLength*3 == len(ordered) {
			return ThreeLine
		}
		if lineLength*4 == len(ordered) {
			return ThreeLineTakeOne
		}
		if lineLength*5 == len(ordered) {
			return ThreeLineTakeTwo
		}
	}

	if doubleCount := cardCountsWithCount(cardCounts, 2); doubleCount >= 2 {
		lineLength, high, _ := consecutiveGroups(cardCounts, 2)
		if high < 3 {
			return Error
		}
		if lineLength*2 == len(ordered) {
			return DoubleLine
		}
	}

	if len(ordered) >= 5 {
		lineLength, _, _ := consecutiveGroups(cardCounts, 1)
		if lineLength == len(ordered) {
			return SingleLine
		}
	}

	return Error
}

func RestType(cards []int, rule Rule) CardType {
	if len(cards) <= 2 {
		return Type(cards, rule)
	}
	cardCounts, valid := counts(SortCards(cards))
	if !valid {
		return Error
	}
	fourCount := cardCountsWithCount(cardCounts, 4)
	if fourCount > 0 {
		if fourCount == 1 && len(cards) == 4 {
			return Bomb
		}
		if fourCount == 1 && len(cards) == 6 && rule.FourTakeTwo {
			return FourLineTakeX
		}
		if fourCount == 1 && len(cards) == 7 && rule.FourTakeThree {
			return FourLineTakeX
		}
		return Error
	}
	if cardCountsWithCount(cardCounts, 3) > 0 {
		lineLength, _, _ := consecutiveGroupsThrough(cardCounts, 3, 15)
		switch {
		case lineLength*3 == len(cards):
			if lineLength == 1 && cardCounts[14] == 3 && rule.ThreeABomb {
				return Bomb
			}
			return ThreeLine
		case lineLength*4 == len(cards):
			return ThreeLineTakeOne
		case lineLength*5 == len(cards):
			return ThreeLineTakeTwo
		case len(cards) >= lineLength*3 && len(cards) <= lineLength*5:
			return ThreeLineTakeTwo
		default:
			return Error
		}
	}
	return Type(cards, rule)
}

func firstRequiredCard(cards []int) (int, bool) {
	for rank := 3; rank <= 12; rank++ {
		for suit := 3; suit >= 0; suit-- {
			card := suit<<4 | rank
			if containsCard(cards, card) {
				return card, true
			}
		}
	}
	return 0, false
}

func cardCountsWithCount(counts map[int]int, wanted int) int {
	result := 0
	for _, count := range counts {
		if count == wanted {
			result++
		}
	}
	return result
}

func Compare(previous, next []int, rule Rule, lastTurn, baiwei bool) bool {
	previousType := Type(previous, rule)
	nextType := Type(next, rule)
	if previousType == Error || nextType == Error {
		return false
	}
	if previousType != Bomb && nextType == Bomb {
		return true
	}
	if previousType == Bomb && nextType != Bomb {
		return false
	}
	tripleType := func(value CardType) bool {
		return value == ThreeLine || value == ThreeLineTakeOne || value == ThreeLineTakeTwo
	}
	if !(tripleType(previousType) && tripleType(nextType) && lastTurn && !baiwei) {
		if previousType != nextType || len(previous) != len(next) {
			return false
		}
	}

	previousOrdered := SortCards(previous)
	nextOrdered := SortCards(next)
	previousCounts, _ := counts(previousOrdered)
	nextCounts, _ := counts(nextOrdered)
	previousValue := previousOrdered[0]
	nextValue := nextOrdered[0]
	if nextType == ThreeLine || nextType == ThreeLineTakeOne || nextType == ThreeLineTakeTwo {
		_, previousHigh, _ := consecutiveGroups(previousCounts, 3)
		_, nextHigh, _ := consecutiveGroups(nextCounts, 3)
		previousValue, nextValue = previousHigh, nextHigh
	} else if nextType == FourLineTakeX {
		for value, count := range previousCounts {
			if count == 4 {
				previousValue = value
			}
		}
		for value, count := range nextCounts {
			if count == 4 {
				nextValue = value
			}
		}
	} else {
		previousValue = CardLogicValue(previousOrdered[0])
		nextValue = CardLogicValue(nextOrdered[0])
	}
	return nextValue > previousValue
}
