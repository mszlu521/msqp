package handler

import (
	"hall/models/request"
	"testing"
)

func TestModifyScoreRequestAllowsLegacyAddAndSubtract(t *testing.T) {
	tests := []struct {
		name  string
		count int
		want  bool
	}{
		{name: "add score", count: 10, want: true},
		{name: "subtract score", count: -10, want: true},
		{name: "zero score", count: 0, want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := request.ModifyScoreReq{UnionID: 42, MemberUid: "member-1", Count: test.count}
			if got := validModifyScoreRequest(req); got != test.want {
				t.Fatalf("validModifyScoreRequest(count=%d) = %v, want %v", test.count, got, test.want)
			}
		})
	}
}

func TestModifyScoreRequestRejectsMissingIdentity(t *testing.T) {
	if validModifyScoreRequest(request.ModifyScoreReq{MemberUid: "member-1", Count: 10}) {
		t.Fatal("request without union ID was accepted")
	}
	if validModifyScoreRequest(request.ModifyScoreReq{UnionID: 42, Count: 10}) {
		t.Fatal("request without member UID was accepted")
	}
}
