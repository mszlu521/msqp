package sg

import "testing"

func TestEvaluateAndCompare(t *testing.T) {
	if Evaluate([]int{0x01, 0x11, 0x21}, true).Type != BaoZi {
		t.Fatal("expected baozi")
	}
	if Evaluate([]int{0x0b, 0x1c, 0x2d}, true).Type != SanGong {
		t.Fatal("expected san gong")
	}
	if Compare([]int{0x09, 0x0a, 0x1a}, []int{0x08, 0x0a, 0x1a}) <= 0 {
		t.Fatal("nine points should beat eight")
	}
}
