package pdk

import (
	"core/models/enums"
	"framework/remote"
	"game/component/internal/testutil"
	"game/component/proto"
	"testing"
	"time"
)

func TestGameStartPushIncludesAllPlayerCardCounts(t *testing.T) {
	counts := [][]int{{1, 1}, {1, 1, 1}}
	push, ok := gameStartPush(0, 1, []int{1, 2}, counts).(map[string]any)
	if !ok {
		t.Fatalf("game start push type = %T", gameStartPush(0, 1, nil, counts))
	}
	data, ok := push["data"].(map[string]any)
	if !ok {
		t.Fatalf("game start push data type = %T", push["data"])
	}
	got, ok := data["allUserCardCountArr"].([][]int)
	if !ok || len(got) != 2 || len(got[1]) != 3 {
		t.Fatalf("card counts = %#v, want two player rows with lengths 2 and 3", data["allUserCardCountArr"])
	}
}

func TestGameBureauDataContainsClientResultRecords(t *testing.T) {
	record := map[string]any{
		"nicknameArr":  []any{"a", "b", nil},
		"allCardArr":   [][]int{{1}, {2}, nil},
		"allHandCards": [][][]int{{{1}}, {{2}}, nil},
		"winArr":       []int{1, -1, 0},
	}
	frame := &GameFrame{bureauRecords: []any{record}}

	data, ok := frame.GetGameBureauData().([]any)
	if !ok || len(data) != 1 {
		t.Fatalf("GetGameBureauData() = %#v, want one result record", frame.GetGameBureauData())
	}
	if _, ok := data[0].(map[string]any)["allHandCards"]; !ok {
		t.Fatalf("result record does not contain client result fields: %#v", data[0])
	}
}

func TestGameVideoDataUsesRecordedBureaus(t *testing.T) {
	record := map[string]any{"curBureau": 1, "winChairID": 0}
	frame := &GameFrame{bureauRecords: []any{record}}
	data, ok := frame.GetGameVideoData().([]any)
	if !ok || len(data) != 1 || data[0].(map[string]any)["curBureau"] != 1 {
		t.Fatalf("GetGameVideoData() = %#v, want recorded bureau data", frame.GetGameVideoData())
	}
}

func TestRoomDismissPushIncludesAggregateResult(t *testing.T) {
	room := testutil.NewPlayingRoom(2)
	room.Creator = proto.RoomCreator{Uid: "a", CreatorType: enums.UserCreatorType}
	room.Users["a"].UserInfo.Nickname = "Alice"
	room.Users["a"].UserInfo.Avatar = "alice.png"
	room.Users["a"].WinScore = 5
	room.Users["b"].UserInfo.Nickname = "Bob"
	room.Users["b"].WinScore = -5
	frame := NewGameFrame(proto.GameRule{MaxPlayerCount: 2}, room, nil)
	frame.bureauRecords = []any{
		map[string]any{"winArr": []int{2, -2}, "bombArr": []int{1, 0}},
		map[string]any{"winArr": []int{3, -3}, "bombArr": []int{2, 1}},
	}

	frame.OnEventRoomDismiss(enums.BureauFinished, &remote.Session{})
	if len(room.Broadcasts) != 1 {
		t.Fatalf("dismiss broadcasts = %d, want 1", len(room.Broadcasts))
	}
	push := room.Broadcasts[0].(map[string]any)
	if push["type"] != GameDismissPush || push["pushRouter"] != "GameMessagePush" {
		t.Fatalf("dismiss push envelope = %#v", push)
	}
	data := push["data"].(map[string]any)
	users := data["userArray"].([]*dismissUser)
	if len(users) != 2 || users[0].Uid != "a" || users[0].WinScore != 5 || users[0].SingleMaxScore != 3 || users[0].BoomCount != 3 || users[0].WinCount != 2 || users[0].LoseCount != 0 {
		t.Fatalf("aggregate dismiss users = %#v", users)
	}
	creator := data["creator"].(*dismissCreator)
	reason, ok := data["reason"].(enums.RoomDismissReason)
	if !ok || creator.Uid != "a" || creator.Nickname != "Alice" || reason != enums.BureauFinished {
		t.Fatalf("dismiss creator/data = %#v / %#v", creator, data)
	}
}

func TestFinishKeepsOriginalCardsForResultAndVideo(t *testing.T) {
	room := testutil.NewPlayingRoom(2)
	game := NewGameFrame(proto.GameRule{MaxPlayerCount: 2, BaseScore: 1}, room, nil)
	game.data.GameStarted = true
	game.data.GameStatus = GameStatusOutCard
	game.data.CurChairID = 0
	game.hands = [][]int{{}, {0x04, 0x05}}
	game.allCards = [][]int{{0x03, 0x33}, {0x04, 0x05, 0x06}}
	game.played = [][]int{{0x03, 0x33}, {0x06}}

	game.finish(room.Users["a"], []int{0x03, 0x33}, &remote.Session{})
	if len(game.bureauRecords) != 1 {
		t.Fatalf("finish recorded %d bureaus, want 1", len(game.bureauRecords))
	}
	record, ok := game.bureauRecords[0].(map[string]any)
	if !ok {
		t.Fatalf("result record has type %T, want map", game.bureauRecords[0])
	}
	allCards, ok := record["allCardArr"].([][]int)
	if !ok || len(allCards[0]) != 2 || len(allCards[1]) != 3 {
		t.Fatalf("result lost played cards: %#v", record["allCardArr"])
	}
}

func TestManualPassDoesNotAutoPlayAtStartOfTurn(t *testing.T) {
	if canAutoPassOrPlay(false) {
		t.Fatal("manual pass must not auto-play without a previous turn")
	}
	if !canAutoPassOrPlay(true) {
		t.Fatal("timeout/offline handling must auto-play")
	}
}

func TestPreviousTurnPushUsesLegacyPlayerRecordShape(t *testing.T) {
	room := testutil.NewPlayingRoom(3)
	room.Users["a"].UserInfo.Nickname, room.Users["a"].UserInfo.Avatar = "Alice", "alice.png"
	room.Users["b"].UserInfo.Nickname, room.Users["b"].UserInfo.Avatar = "Bob", "bob.png"
	room.Users["c"].UserInfo.Nickname, room.Users["c"].UserInfo.Avatar = "Carol", "carol.png"
	game := NewGameFrame(proto.GameRule{MaxPlayerCount: 3}, room, nil)
	game.data.GameStarted = true
	game.data.GameStatus = GameStatusOutCard
	game.data.CurChairID = 0
	game.data.TurnWinerChairID = 0
	game.hands = [][]int{{0x03, 0x04}, {0x05}, {0x06}}
	game.played = make([][]int, 3)
	game.data.IsFirstTurnArray = []bool{true, true, true}
	game.turnTimeout = time.Hour
	defer func() {
		if game.turnTimer != nil {
			game.turnTimer.Stop()
		}
	}()

	game.out(room.Users["a"], []int{0x03}, nil)
	game.pass(room.Users["b"], nil, false)
	game.pass(room.Users["c"], nil, false)
	directBeforeReview := len(room.Direct)
	game.GameMessageHandle(room.Users["a"], nil, []byte(`{"type":313,"data":{}}`))

	if len(room.Direct) != directBeforeReview+1 {
		t.Fatalf("previous-turn request added %d direct pushes, want 1", len(room.Direct)-directBeforeReview)
	}
	push := room.Direct[len(room.Direct)-1].(map[string]any)
	data := push["data"].(map[string]any)
	records := data["list"].([]preTurnRecord)
	if len(records) != 3 {
		t.Fatalf("previous-turn records = %#v, want play plus two passes", records)
	}
	if records[0].Name != "Alice" || records[0].Avatar != "alice.png" || len(records[0].Cards) != 1 || records[0].Cards[0] != 0x03 {
		t.Fatalf("play record = %#v", records[0])
	}
	if records[1].Name != "Bob" || len(records[1].Cards) != 0 || records[2].Name != "Carol" || len(records[2].Cards) != 0 {
		t.Fatalf("pass records = %#v", records[1:])
	}
}

func TestSpectatorCannotMutateTrustState(t *testing.T) {
	room := testutil.NewPlayingRoom(3)
	game := NewGameFrame(proto.GameRule{MaxPlayerCount: 3}, room, nil)
	game.data.GameStarted = true
	spectator := &proto.RoomUser{ChairID: 9, UserInfo: &proto.UserInfo{Uid: "watcher"}}
	game.GameMessageHandle(spectator, &remote.Session{}, []byte(`{"type":312,"data":{"trust":true}}`))
	if len(room.Broadcasts) != 0 {
		t.Fatalf("spectator trust request was broadcast: %#v", room.Broadcasts)
	}
}

func TestSpectatorOfflineEventIsIgnored(t *testing.T) {
	room := testutil.NewPlayingRoom(3)
	game := NewGameFrame(proto.GameRule{MaxPlayerCount: 3}, room, nil)
	game.data.CurChairID = 9
	spectator := &proto.RoomUser{ChairID: 9, UserInfo: &proto.UserInfo{Uid: "watcher"}}
	game.OnEventUserOffLine(spectator, &remote.Session{})
	if len(room.Broadcasts) != 0 {
		t.Fatalf("spectator offline event produced pushes: %#v", room.Broadcasts)
	}
}

func TestPlayingUsersIgnoresMalformedRoomEntries(t *testing.T) {
	room := testutil.NewPlayingRoom(2)
	room.Users["nil"] = nil
	room.Users["negative"] = &proto.RoomUser{ChairID: -1, UserInfo: &proto.UserInfo{Uid: "negative"}, UserStatus: enums.Playing}
	room.Users["missing-info"] = &proto.RoomUser{ChairID: 1, UserStatus: enums.Playing}
	game := NewGameFrame(proto.GameRule{MaxPlayerCount: 2}, room, nil)
	if got := game.playingUsers(); len(got) != 2 {
		t.Fatalf("playingUsers() returned %d entries, want 2: %#v", len(got), got)
	}
}

func TestTrustRequestRespectsRoomRule(t *testing.T) {
	room := testutil.NewPlayingRoom(3)
	game := NewGameFrame(proto.GameRule{MaxPlayerCount: 3, CanTrust: false}, room, nil)
	game.data.GameStarted = true
	game.GameMessageHandle(room.Users["a"], &remote.Session{}, []byte(`{"type":312,"data":{"trust":true}}`))
	if game.data.UserTrustArray[0] || len(room.Broadcasts) != 0 {
		t.Fatalf("disabled trust was accepted: state=%v pushes=%#v", game.data.UserTrustArray, room.Broadcasts)
	}
}

func TestForcedTurnPlaysLegalCardWhenPassIsForbidden(t *testing.T) {
	room := testutil.NewPlayingRoom(2)
	game := NewGameFrame(proto.GameRule{MaxPlayerCount: 2, Bichu: true}, room, nil)
	game.data.GameStarted = true
	game.data.GameStatus = GameStatusOutCard
	game.data.CurChairID = 1
	game.data.TurnWinerChairID = 0
	game.hands[0] = []int{0x03, 0x09}
	game.hands[1] = []int{0x04, 0x08}
	game.played = make([][]int, 2)
	game.lastCards = []int{0x03}

	game.pass(room.Users["b"], &remote.Session{}, true)
	defer game.stopTurnTimer()
	if len(game.hands[1]) != 1 || len(game.played[1]) != 1 || game.played[1][0] != 0x04 {
		t.Fatalf("forced response hand=%v played=%v, want legal single 4", game.hands[1], game.played[1])
	}
	if data := lastPDKPushData(room.Direct, 402); data == nil || data["chairID"] != 1 {
		t.Fatalf("forced response push missing: %#v", room.Direct)
	}
}

func TestForcedTurnCanUseThreeAceBomb(t *testing.T) {
	room := testutil.NewPlayingRoom(2)
	game := NewGameFrame(proto.GameRule{MaxPlayerCount: 2, Bichu: true, ThreeABomb: true}, room, nil)
	game.data.GameStarted = true
	game.data.GameStatus = GameStatusOutCard
	game.data.CurChairID = 1
	game.data.TurnWinerChairID = 0
	game.hands[0] = []int{0x02, 0x0d}
	game.hands[1] = []int{0x01, 0x11, 0x21, 0x03}
	game.played = make([][]int, 2)
	game.lastCards = []int{0x02}

	game.pass(room.Users["b"], &remote.Session{}, true)
	defer game.stopTurnTimer()
	if len(game.hands[1]) != 1 || len(game.played[1]) != 3 || Type(game.played[1], Rule{ThreeABomb: true}) != Bomb {
		t.Fatalf("forced response hand=%v played=%v, want three-ace bomb", game.hands[1], game.played[1])
	}
}

func TestForcedOpeningTurnHonorsSpadeThreeRule(t *testing.T) {
	room := testutil.NewPlayingRoom(2)
	game := NewGameFrame(proto.GameRule{MaxPlayerCount: 2, Heitao3: true}, room, nil)
	game.data.GameStarted = true
	game.data.GameStatus = GameStatusOutCard
	game.data.CurBureau = 1
	game.data.CurChairID = 0
	game.data.FirstChairID = 0
	game.data.TurnWinerChairID = 0
	game.hands[0] = []int{0x0d, 0x33}
	game.hands[1] = []int{0x04, 0x05}
	game.played = make([][]int, 2)

	game.pass(room.Users["a"], &remote.Session{}, true)
	defer game.stopTurnTimer()
	if len(game.played[0]) != 1 || game.played[0][0] != 0x33 || containsCard(game.hands[0], 0x33) {
		t.Fatalf("forced opening played=%v hand=%v, want spade 3", game.played[0], game.hands[0])
	}
}

func TestForcedOpeningTurnUsesLegacyFallbackRequiredCard(t *testing.T) {
	room := testutil.NewPlayingRoom(2)
	game := NewGameFrame(proto.GameRule{MaxPlayerCount: 2, Heitao3: true}, room, nil)
	game.data.GameStarted = true
	game.data.GameStatus = GameStatusOutCard
	game.data.CurBureau = 1
	game.data.CurChairID = 0
	game.data.FirstChairID = 0
	game.data.TurnWinerChairID = 0
	game.hands[0] = []int{0x0d, 0x13}
	game.hands[1] = []int{0x04, 0x05}
	game.played = make([][]int, 2)

	game.pass(room.Users["a"], &remote.Session{}, true)
	defer game.stopTurnTimer()
	if len(game.played[0]) != 1 || game.played[0][0] != 0x13 || containsCard(game.hands[0], 0x13) {
		t.Fatalf("forced opening played=%v hand=%v, want club 3 fallback", game.played[0], game.hands[0])
	}
}

func TestFinalHandAcceptsLegacyShortAttachments(t *testing.T) {
	room := testutil.NewPlayingRoom(2)
	game := NewGameFrame(proto.GameRule{MaxPlayerCount: 2, Baiwei: false}, room, nil)
	cards := []int{0x03, 0x13, 0x23, 0x04, 0x14, 0x24, 0x05}
	game.data.GameStarted = true
	game.data.GameStatus = GameStatusOutCard
	game.data.CurBureau = 2
	game.data.CurChairID = 0
	game.data.TurnWinerChairID = 0
	game.hands[0] = append([]int(nil), cards...)
	game.hands[1] = []int{0x06}
	game.allCards[0] = append([]int(nil), cards...)
	game.allCards[1] = []int{0x06}
	game.played = make([][]int, 2)

	game.out(room.Users["a"], cards, &remote.Session{})
	if len(game.hands[0]) != 0 || game.data.GameStarted {
		t.Fatalf("legacy final hand was rejected: hand=%v started=%v", game.hands[0], game.data.GameStarted)
	}
}

func TestSpadeThreeRuleOnlyAppliesToFirstPlay(t *testing.T) {
	room := testutil.NewPlayingRoom(2)
	game := NewGameFrame(proto.GameRule{MaxPlayerCount: 2, Heitao3: true}, room, nil)
	game.data.GameStarted = true
	game.data.GameStatus = GameStatusOutCard
	game.data.CurBureau = 1
	game.data.CurChairID = 0
	game.data.FirstChairID = 0
	game.data.IsFirstTurnArray[0] = true
	game.hands[0] = []int{0x33, 0x04}
	game.hands[1] = []int{0x05}
	game.played = make([][]int, 2)

	game.out(room.Users["a"], []int{0x04}, &remote.Session{})
	if len(game.played[0]) != 0 {
		t.Fatal("first play without spade three must be rejected")
	}
	game.out(room.Users["a"], []int{0x33}, &remote.Session{})
	if len(game.played[0]) != 1 || game.data.IsFirstTurnArray[0] {
		t.Fatalf("first play did not consume spade three: played=%v first=%v", game.played[0], game.data.IsFirstTurnArray[0])
	}

	game.data.CurChairID = 0
	game.hands[0] = []int{0x33, 0x04}
	game.played[0] = nil
	game.data.IsFirstTurnArray[0] = false
	game.out(room.Users["a"], []int{0x04}, &remote.Session{})
	if len(game.played[0]) != 1 || game.played[0][0] != 0x04 {
		t.Fatalf("later play was incorrectly forced to include spade three: %v", game.played[0])
	}
}

func TestNewBureauClearsTurnResidueAndSmallBureauTrust(t *testing.T) {
	room := testutil.NewPlayingRoom(2)
	game := NewGameFrame(proto.GameRule{MaxPlayerCount: 2, XiaojuTrust: true}, room, nil)
	game.lastCards = []int{0x0d}
	game.passCount = 1
	game.data.UserBombTimes[0] = 2
	game.data.UserTrustArray[0] = true
	game.data.GameStatus = GameStatusEnd

	game.OnEventGameStart(nil, &remote.Session{})
	defer game.stopTurnTimer()
	if len(game.lastCards) != 0 || game.passCount != 0 || game.data.UserBombTimes[0] != 0 {
		t.Fatalf("new bureau retained turn residue: last=%v passes=%d bombs=%v", game.lastCards, game.passCount, game.data.UserBombTimes)
	}
	if game.data.UserTrustArray[0] {
		t.Fatal("small-bureau trust survived into the next bureau")
	}
	if data := lastPDKPushData(room.Broadcasts, GameTrustPush); data == nil || data["chairID"] != 0 || data["trust"] != false {
		t.Fatalf("trust cancellation push missing: %#v", room.Broadcasts)
	}
}

func TestReconnectSnapshotRecomputesEnablePass(t *testing.T) {
	room := testutil.NewPlayingRoom(2)
	game := NewGameFrame(proto.GameRule{MaxPlayerCount: 2, Bichu: true}, room, nil)
	game.data.GameStarted = true
	game.data.CurChairID = 1
	game.lastCards = []int{0x0d}
	game.hands[1] = []int{0x03, 0x04}

	snapshot := game.GetEnterGameData(nil).(*GameData)
	if !snapshot.EnablePass {
		t.Fatal("reconnect snapshot forbids pass when the player cannot beat the turn")
	}
	game.lastCards = []int{0x03}
	snapshot = game.GetEnterGameData(nil).(*GameData)
	if snapshot.EnablePass {
		t.Fatal("reconnect snapshot allows pass despite a mandatory legal response")
	}
}

func TestTurnTimerContinuesAfterForcedMandatoryPlay(t *testing.T) {
	room := testutil.NewPlayingRoom(2)
	game := NewGameFrame(proto.GameRule{MaxPlayerCount: 2, Bichu: true}, room, nil)
	game.data.GameStarted = true
	game.data.GameStatus = GameStatusOutCard
	game.data.CurChairID = 1
	game.data.TurnWinerChairID = 0
	game.hands[0] = []int{0x03, 0x09}
	game.hands[1] = []int{0x04, 0x08}
	game.played = make([][]int, 2)
	game.lastCards = []int{0x03}
	game.turnTimeout = time.Millisecond
	game.scheduleTurn(&remote.Session{})
	defer game.stopTurnTimer()

	deadline := time.Now().Add(250 * time.Millisecond)
	for time.Now().Before(deadline) {
		if len(game.played[1]) > 0 && game.data.CurChairID != 1 {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("mandatory timeout did not advance: chair=%d hand=%v played=%v", game.data.CurChairID, game.hands[1], game.played[1])
}

func TestTurnTimeoutMatchesClientFirstTurnClock(t *testing.T) {
	game := NewGameFrame(proto.GameRule{MaxPlayerCount: 2}, testutil.NewPlayingRoom(2), nil)
	game.data.IsFirstTurnArray[0] = true
	if got := game.timeoutForChair(0); got != 30*time.Second {
		t.Fatalf("first turn timeout = %v, want 30s", got)
	}
	game.data.IsFirstTurnArray[0] = false
	if got := game.timeoutForChair(0); got != 15*time.Second {
		t.Fatalf("later turn timeout = %v, want 15s", got)
	}
}

func TestTrustPersistsAcrossBureausWithoutSmallBureauOption(t *testing.T) {
	room := testutil.NewPlayingRoom(2)
	game := NewGameFrame(proto.GameRule{MaxPlayerCount: 2, XiaojuTrust: false}, room, nil)
	game.data.UserTrustArray[0] = true
	game.OnEventGameStart(nil, &remote.Session{})
	defer game.stopTurnTimer()
	if !game.data.UserTrustArray[0] {
		t.Fatal("persistent trust was cleared despite xiaojuTrust being disabled")
	}
	if data := lastPDKPushData(room.Broadcasts, GameTrustPush); data != nil {
		t.Fatalf("unexpected trust cancellation push: %#v", data)
	}
}

func (g *GameFrame) stopTurnTimer() {
	if g.turnTimer != nil {
		g.turnTimer.Stop()
		g.turnTimer = nil
	}
}

func lastPDKPushData(pushes []any, typ int) map[string]any {
	for index := len(pushes) - 1; index >= 0; index-- {
		message, ok := pushes[index].(map[string]any)
		if !ok || message["type"] != typ {
			continue
		}
		data, _ := message["data"].(map[string]any)
		return data
	}
	return nil
}
