package handler

import "testing"

func TestCanViewUnionRank(t *testing.T) {
	tests := []struct {
		name      string
		ownerUid  string
		viewerUid string
		visible   bool
		want      bool
	}{
		{name: "owner while hidden", ownerUid: "owner", viewerUid: "owner", visible: false, want: true},
		{name: "owner while visible", ownerUid: "owner", viewerUid: "owner", visible: true, want: true},
		{name: "member while visible", ownerUid: "owner", viewerUid: "member", visible: true, want: true},
		{name: "member while hidden", ownerUid: "owner", viewerUid: "member", visible: false, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := canViewUnionRank(tt.ownerUid, tt.viewerUid, tt.visible); got != tt.want {
				t.Fatalf("canViewUnionRank() = %v, want %v", got, tt.want)
			}
		})
	}
}
