package mj

import (
	"fmt"
	"game/component/mj/mp"
	"testing"
)

func TestDel(t *testing.T) {
	gf := &GameFrame{}
	cards := gf.delCardFromArray([]mp.CardID{1, 2, 11, 3, 4, 5, 6, 11, 7, 8, 9, 10}, 11, 2)
	fmt.Println(cards)
}

var cur int

func TestAppend(t *testing.T) {
	old := make([][]mp.CardID, 4)
	old[0] = []mp.CardID{1, 2, 3, 6, 7, 8, 9, 0, 10, 11, 11, 12, 13}
	old[1] = []mp.CardID{4, 2, 5, 6, 11, 8, 9, 0, 10, 23, 34, 56, 78}
	cur = 0
	old[cur] = append(old[cur], 9)
	fmt.Println(old)
}
