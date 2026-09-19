package handler

import (
	"core/models/entity"
	"testing"
)

func TestHasUnionAccess(t *testing.T) {
	member := &entity.User{UnionInfo: []*entity.UnionInfo{{UnionID: 42}}}
	tests := []struct {
		name    string
		user    *entity.User
		unionID int64
		want    bool
	}{
		{name: "public union", user: &entity.User{}, unionID: 1, want: true},
		{name: "member union", user: member, unionID: 42, want: true},
		{name: "other union", user: member, unionID: 43, want: false},
		{name: "missing user", user: nil, unionID: 42, want: false},
		{name: "invalid union", user: member, unionID: 0, want: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := hasUnionAccess(tc.user, tc.unionID); got != tc.want {
				t.Fatalf("hasUnionAccess(%v, %d) = %v, want %v", tc.user != nil, tc.unionID, got, tc.want)
			}
		})
	}
}
