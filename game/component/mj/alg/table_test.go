package alg

import (
	"fmt"
	"game/component/mj/mp"
	"testing"
)

func TestGen(t *testing.T) {
	table := NewTable()
	table.gen()
	//321110033
	//1A1A1A 2A2A 3A4A5A 8A8A8A 9A9A9A  3*n+2  =  3*n + 3*m + 2
	//301221020
	//1A1A1A 3A4A5A 4A5A6A    8A8A   B
}
func TestCheckHu(t *testing.T) {
	h := NewHuLogic()
	cards := []mp.CardID{
		mp.Wan1, mp.Wan1, mp.Wan1, mp.Wan2, mp.Wan3, mp.Wan5, mp.Wan5, mp.Wan5,
		mp.Tong1, mp.Tong1, mp.Tong1, mp.Zhong, mp.Tong4,
	}
	checkHu := h.CheckHu(cards, []mp.CardID{mp.Zhong}, mp.Tong2)
	fmt.Println(checkHu)
}
