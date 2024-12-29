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
