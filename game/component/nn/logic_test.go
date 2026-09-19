package nn

import "testing"

func TestNiuNumberAndBasicTypes(t *testing.T) {
	if NiuNumber([]int{0x01, 0x09, 0x1a, 0x2a, 0x3a}) != 10 {
		t.Fatal("A9101010 should be 牛牛")
	}
	if got := Evaluate([]int{0x0a, 0x1a, 0x05, 0x15, 0x2a}, Rule{}); got.Type != NiuNiu {
		t.Fatalf("got type %v, want 牛牛", got.Type)
	}
	if got := Evaluate([]int{0x03, 0x14, 0x25, 0x36, 0x07}, Rule{ShunZiNiu: true}); got.Type != ShunZiNiu {
		t.Fatalf("got type %v, want 顺子牛", got.Type)
	}
}

func TestSpecialTypesAndCompare(t *testing.T) {
	rule := Rule{TongHuaShun: true, ZhaDanNiu: true, WuXiaoNiu: true}
	if got := Evaluate([]int{0x09, 0x0a, 0x0b, 0x0c, 0x0d}, rule); got.Type != TongHuaShun {
		t.Fatalf("got type %v, want 同花顺", got.Type)
	}
	if got := Evaluate([]int{0x03, 0x13, 0x23, 0x33, 0x04}, rule); got.Type != ZhaDanNiu {
		t.Fatalf("got type %v, want 炸弹牛", got.Type)
	}
	if Compare([]int{0x03, 0x13, 0x23, 0x33, 0x04}, []int{0x04, 0x14, 0x24, 0x34, 0x05}, rule) >= 0 {
		t.Fatal("the higher bomb should win")
	}
}
