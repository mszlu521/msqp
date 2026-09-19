package dgn

import "testing"

func TestEvaluateSpecialTypes(t *testing.T) {
	config := map[string]bool{
		"ZHADANNIU": true,
		"WUXIAONIU": true,
	}
	if Evaluate([]int{0x03, 0x13, 0x23, 0x33, 0x04}, config, 1).Type != 10 {
		t.Fatal("expected bomb niu")
	}
	if Evaluate([]int{0x01, 0x11, 0x21, 0x31, 0x02}, map[string]bool{"WUXIAONIU": true}, 1).Type != 9 {
		t.Fatal("expected five-small niu")
	}
}

func TestCompareUsesFiveCards(t *testing.T) {
	if Compare([]int{0x01, 0x09, 0x1a, 0x2a, 0x3a}, []int{0x01, 0x08, 0x1a, 0x2a, 0x3a}, nil, 1) <= 0 {
		t.Fatal("higher normal niu should win")
	}
}
