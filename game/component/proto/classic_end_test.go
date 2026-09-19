package proto

import "testing"

func TestClassicGameEndPushDataMatchesLegacyResultContract(t *testing.T) {
	users := map[string]*RoomUser{
		"b": {ChairID: 1, WinScore: -6, UserInfo: &UserInfo{Uid: "b", Nickname: "B", Avatar: "b.png"}},
		"a": {ChairID: 0, WinScore: 6, UserInfo: &UserInfo{Uid: "a", Nickname: "A", Avatar: "a.png"}},
	}
	push := ClassicGameEndPushData(409, users, &RoomCreator{Uid: "b"}, []int{1, -1}, true).(map[string]any)
	if push["type"] != 409 || push["pushRouter"] != "GameMessagePush" {
		t.Fatalf("unexpected envelope: %#v", push)
	}
	data := push["data"].(map[string]any)
	result := data["result"].([]*ClassicEndResult)
	if len(result) != 2 || result[0].Uid != "a" || result[0].Score != 6 || result[1].Uid != "b" || result[1].Score != -6 {
		t.Fatalf("unexpected result rows: %#v", result)
	}
	creator := data["creater"].(*ClassicEndCreator)
	if creator.Uid != "b" || data["winMost"] != "a" || data["loseMost"] != "b" {
		t.Fatalf("unexpected summary metadata: %#v", data)
	}
}

func TestClassicGameEndPushDataIsEmptyBeforeFirstCompletedBureau(t *testing.T) {
	users := map[string]*RoomUser{
		"a": {ChairID: 0, UserInfo: &UserInfo{Uid: "a", Nickname: "A"}},
	}
	push := ClassicGameEndPushData(408, users, &RoomCreator{Uid: "a"}, nil, false).(map[string]any)
	data := push["data"].(map[string]any)
	if result := data["result"].([]*ClassicEndResult); len(result) != 0 {
		t.Fatalf("result before a completed bureau = %#v, want empty", result)
	}
	if data["creater"] != (*ClassicEndCreator)(nil) || data["winMost"] != nil || data["loseMost"] != nil {
		t.Fatalf("metadata before a completed bureau = %#v, want empty", data)
	}
}
