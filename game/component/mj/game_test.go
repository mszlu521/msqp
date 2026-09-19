package mj

import (
	"fmt"
	"framework/remote"
	"framework/stream"
	"game/component/internal/testutil"
	"game/component/mj/mp"
	"game/component/proto"
	"testing"
)

func TestDel(t *testing.T) {
	fmt.Println(1 & 1)
}

func TestQiduiWithAndWithoutHongZhong(t *testing.T) {
	logic := NewLogic(HongZhong4, true)
	sevenPairs := []mp.CardID{
		mp.Wan1, mp.Wan1, mp.Wan2, mp.Wan2, mp.Wan3, mp.Wan3,
		mp.Wan4, mp.Wan4, mp.Wan5, mp.Wan5, mp.Wan6, mp.Wan6, mp.Wan7,
	}
	if !logic.canHuQidui(sevenPairs, mp.Wan7) {
		t.Fatal("ordinary seven pairs should win")
	}
	withHongZhong := append([]mp.CardID{}, sevenPairs...)
	withHongZhong[len(withHongZhong)-1] = mp.Zhong
	if !logic.canHuQidui(withHongZhong, mp.Wan7) {
		t.Fatal("Hong Zhong should complete the missing pair")
	}
	withoutEnoughWild := []mp.CardID{
		mp.Wan1, mp.Wan1, mp.Wan2, mp.Wan2, mp.Wan3, mp.Wan3,
		mp.Wan4, mp.Wan4, mp.Wan5, mp.Wan5, mp.Wan6, mp.Wan7, mp.Wan8,
	}
	if logic.canHuQidui(withoutEnoughWild, mp.Wan9) {
		t.Fatal("two unmatched tiles without Hong Zhong should not win")
	}
}

func TestSpectatorCannotMutateTrustState(t *testing.T) {
	room := testutil.NewPlayingRoom(3)
	game := &GameFrame{
		r:              room,
		handCards:      make([][]mp.CardID, 3),
		userTrustArray: make([]bool, 3),
		trustTmArray:   make([]int, 3),
	}
	spectator := &proto.RoomUser{ChairID: 9, UserInfo: &proto.UserInfo{Uid: "watcher"}}
	game.GameMessageHandle(spectator, &remote.Session{}, []byte(`{"type":312,"data":{"trust":true}}`))
	if len(room.Direct) != 0 {
		t.Fatalf("spectator trust request was sent: %#v", room.Direct)
	}
}

func TestSpectatorOfflineEventIsIgnored(t *testing.T) {
	room := testutil.NewPlayingRoom(3)
	game := &GameFrame{r: room, curChairID: 9, handCards: make([][]mp.CardID, 3), userTrustArray: make([]bool, 3), trustTmArray: make([]int, 3)}
	spectator := &proto.RoomUser{ChairID: 9, UserInfo: &proto.UserInfo{Uid: "watcher"}}
	game.OnEventUserOffLine(spectator, &remote.Session{})
	if len(room.Direct) != 0 || len(room.Broadcasts) != 0 {
		t.Fatalf("spectator offline event produced pushes: direct=%#v broadcasts=%#v", room.Direct, room.Broadcasts)
	}
}

func TestTrustRequestRespectsRoomRule(t *testing.T) {
	room := testutil.NewPlayingRoom(3)
	game := &GameFrame{
		r:              room,
		gameRule:       proto.GameRule{CanTrust: false},
		handCards:      make([][]mp.CardID, 3),
		userTrustArray: make([]bool, 3),
		trustTmArray:   make([]int, 3),
	}
	game.GameMessageHandle(room.Users["a"], &remote.Session{}, []byte(`{"type":312,"data":{"trust":true}}`))
	if game.userTrustArray[0] || len(room.Direct) != 0 {
		t.Fatalf("disabled trust was accepted: state=%v sends=%#v", game.userTrustArray, room.Direct)
	}
}

func TestGameFramesKeepIndependentChairCounts(t *testing.T) {
	twoPlayer := NewGameFrame(proto.GameRule{MaxPlayerCount: 2}, testutil.NewPlayingRoom(2), &remote.Session{})
	fourPlayer := NewGameFrame(proto.GameRule{MaxPlayerCount: 4}, testutil.NewPlayingRoom(4), &remote.Session{})
	if twoPlayer.getChairCount() != 2 || len(twoPlayer.handCards) != 2 {
		t.Fatalf("two-player frame changed size: chairs=%d hands=%d", twoPlayer.getChairCount(), len(twoPlayer.handCards))
	}
	if fourPlayer.getChairCount() != 4 || len(fourPlayer.handCards) != 4 {
		t.Fatalf("four-player frame size: chairs=%d hands=%d", fourPlayer.getChairCount(), len(fourPlayer.handCards))
	}
}

func TestSpectatorReconnectHidesHandsAndDoesNotChangeTurn(t *testing.T) {
	room := testutil.NewPlayingRoom(2)
	room.Users["watcher"] = &proto.RoomUser{ChairID: -1, UserInfo: &proto.UserInfo{Uid: "watcher"}}
	game := NewGameFrame(proto.GameRule{MaxPlayerCount: 2}, room, &remote.Session{})
	game.curChairID = -1
	game.handCards = [][]mp.CardID{{1, 2}, {3, 4}}
	session := remote.NewSession(nil, &stream.Msg{Uid: "watcher"})

	snapshot := game.GetEnterGameData(session).(*GameData)
	if game.curChairID != -1 {
		t.Fatalf("spectator reconnect changed current chair to %d", game.curChairID)
	}
	if snapshot.ChairCount != 2 || len(snapshot.HandCards) != 2 {
		t.Fatalf("spectator snapshot has wrong chair count: %#v", snapshot)
	}
	for chairID, cards := range snapshot.HandCards {
		for _, card := range cards {
			if card != 36 {
				t.Fatalf("spectator saw chair %d card %d in %v", chairID, card, cards)
			}
		}
	}
}

var cur int

func TestAppend(t *testing.T) {
	old := make([][]mp.CardID, 4)
	old[0] = []mp.CardID{1, 2, 3, 6, 7, 8, 9, 0, 10, 11, 11, 12, 13}
	old[1] = []mp.CardID{4, 2, 5, 6, 11, 8, 9, 0, 10, 23, 34, 56, 78}
	cur = 0
	old[cur] = append(old[cur], 9)
	fmt.Println(old)
}

func TestRestCardsSnapshotCannotMutateDeck(t *testing.T) {
	logic := NewLogic(HongZhong4, false)
	logic.washCards()
	before := logic.getRestCardsCount()
	snapshot := logic.getRestCards()
	if len(snapshot) != before || len(snapshot) == 0 {
		t.Fatalf("rest-card snapshot length = %d, want %d", len(snapshot), before)
	}
	snapshot[0] = 99
	if logic.getRestCards()[0] == 99 {
		t.Fatal("rest-card snapshot mutated the deck")
	}
}

func TestGameVideoDataFiltersIncompleteBureauWithoutMutatingHistory(t *testing.T) {
	completed := &ReviewRecord{Result: &GameResult{Scores: []int{1, -1}}}
	incomplete := &ReviewRecord{}
	game := &GameFrame{reviewRecord: []*ReviewRecord{completed, incomplete}}

	data, ok := game.GetGameVideoData().([]*ReviewRecord)
	if !ok || len(data) != 1 || data[0] != completed {
		t.Fatalf("video data = %#v, want only completed bureau", data)
	}
	if len(game.reviewRecord) != 2 || game.reviewRecord[1] != incomplete {
		t.Fatalf("incomplete bureau was removed from history: %#v", game.reviewRecord)
	}
}

func TestGameBureauDataSkipsNilHistoryEntries(t *testing.T) {
	game := &GameFrame{reviewRecord: []*ReviewRecord{nil, {UserArray: []proto.UserRoomData{{Uid: "u1", Score: 2}}}}}

	data, ok := game.GetGameBureauData().([][]*BureauReview)
	if !ok || len(data) != 1 || len(data[0]) != 1 || data[0][0].Uid != "u1" {
		t.Fatalf("bureau data = %#v, want nil entries skipped", data)
	}
}
