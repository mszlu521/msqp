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
		mp.Zhong, mp.Wan2, mp.Wan2, mp.Wan6, mp.Wan8, mp.Wan9, mp.Wan9, mp.Tiao8,
		mp.Tiao8, mp.Tiao8,
	}
	checkHu := h.CheckHu(cards, []mp.CardID{mp.Zhong}, mp.Wan2)
	fmt.Println(checkHu)
}

func TestCheckHuRejectsInvalidCardWithoutPanicking(t *testing.T) {
	h := NewHuLogic()
	if h.CheckHu([]mp.CardID{mp.Wan1, 99}, []mp.CardID{mp.Zhong}, mp.Wan2) {
		t.Fatal("invalid hand card must not win")
	}
	if h.CheckHu([]mp.CardID{mp.Wan1}, []mp.CardID{mp.Zhong}, 99) {
		t.Fatal("invalid incoming card must not win")
	}
	if h.CheckHu([]mp.CardID{mp.Wan1, 36}, []mp.CardID{mp.Zhong}, mp.Wan2) {
		t.Fatal("hidden-card marker must not be accepted by hu logic")
	}
	if h.CheckHu([]mp.CardID{mp.Wan1, mp.Wan1, mp.Wan1, mp.Wan1, mp.Wan1}, nil, mp.Wan2) {
		t.Fatal("a normal tile cannot appear more than four times")
	}
}
