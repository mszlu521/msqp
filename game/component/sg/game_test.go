package sg

import (
	"core/models/enums"
	"framework/remote"
	"game/component/internal/testutil"
	"game/component/proto"
	"testing"
	"time"
)

func TestTimeoutContinuesAcrossAllPendingRobbers(t *testing.T) {
	game := NewGameFrame(proto.GameRule{MaxPlayerCount: 3}, testutil.NewPlayingRoom(3), nil)
	game.data.GameStarted = true
	game.data.GameStatus = StatusRob
	game.timeout = time.Millisecond
	game.schedule(&remote.Session{})
	defer game.stop()

	deadline := time.Now().Add(250 * time.Millisecond)
	for time.Now().Before(deadline) {
		if game.data.GameStatus != StatusRob {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("timeout automation stopped before every pending player was handled")
}

func TestGameVideoDataReturnsCardSnapshot(t *testing.T) {
	game := &GameFrame{dealt: [][]int{{1, 2, 3}}}

	video := game.GetGameVideoData().([][]int)
	video[0][0] = 99
	if game.dealt[0][0] != 1 {
		t.Fatalf("video data exposed internal cards: %v", game.dealt)
	}
}

func TestSpectatorCannotMutateTrustState(t *testing.T) {
	room := testutil.NewPlayingRoom(3)
	game := NewGameFrame(proto.GameRule{MaxPlayerCount: 3}, room, nil)
	game.data.GameStarted = true
	spectator := &proto.RoomUser{ChairID: 9, UserInfo: &proto.UserInfo{Uid: "watcher"}}
	game.GameMessageHandle(spectator, &remote.Session{}, []byte(`{"type":311,"data":{"trust":true}}`))
	if len(room.Broadcasts) != 0 {
		t.Fatalf("spectator trust request was broadcast: %#v", room.Broadcasts)
	}
}

func TestSpectatorOfflineEventIsIgnored(t *testing.T) {
	room := testutil.NewPlayingRoom(3)
	game := NewGameFrame(proto.GameRule{MaxPlayerCount: 3}, room, nil)
	spectator := &proto.RoomUser{ChairID: 9, UserInfo: &proto.UserInfo{Uid: "watcher"}}
	game.OnEventUserOffLine(spectator, &remote.Session{})
	if len(room.Broadcasts) != 0 {
		t.Fatalf("spectator offline event produced pushes: %#v", room.Broadcasts)
	}
}

func TestTrustRequestRespectsRoomRule(t *testing.T) {
	room := testutil.NewPlayingRoom(3)
	game := NewGameFrame(proto.GameRule{MaxPlayerCount: 3, CanTrust: false}, room, nil)
	game.data.GameStarted = true
	game.GameMessageHandle(room.Users["a"], &remote.Session{}, []byte(`{"type":311,"data":{"trust":true}}`))
	if game.data.UserTrustArray[0] || len(room.Broadcasts) != 0 {
		t.Fatalf("disabled trust was accepted: state=%v pushes=%#v", game.data.UserTrustArray, room.Broadcasts)
	}
}

func TestPourAcceptsOnlyConfiguredSliderRange(t *testing.T) {
	room := testutil.NewPlayingRoom(2)
	game := NewGameFrame(proto.GameRule{MaxPlayerCount: 2, BaseScore: 2, CanPourScores: []int{1, 3, 5, 10}, MaxCanPourGold: 12}, room, nil)
	game.data.GameStarted = true
	game.data.GameStatus = StatusPour
	game.data.BankerChairID = 0
	game.pour(room.Users["b"], 1, &remote.Session{})
	game.pour(room.Users["b"], 25, &remote.Session{})
	if game.data.PourScores[1] != 0 || len(room.Broadcasts) != 0 {
		t.Fatalf("out-of-range pour score was accepted: scores=%v pushes=%#v", game.data.PourScores, room.Broadcasts)
	}
	game.pour(room.Users["b"], 7, &remote.Session{})
	if game.data.PourScores[1] != 7 {
		t.Fatalf("slider score inside configured range was rejected: %v", game.data.PourScores)
	}
}

func TestUnionPourRespectsWorstCaseBalance(t *testing.T) {
	room := testutil.NewPlayingRoom(2)
	room.Creator.CreatorType = enums.UnionCreatorType
	room.Users["b"].UserInfo.Score = 19
	game := NewGameFrame(proto.GameRule{MaxPlayerCount: 2, ScaleType: 1, CanPourScores: []int{4}, MaxCanPourGold: 4}, room, nil)
	game.data.GameStarted = true
	game.data.GameStatus = StatusPour
	game.data.BankerChairID = 0
	game.data.RobBankScales[0] = 1
	game.pour(room.Users["b"], 4, &remote.Session{})
	if game.data.PourScores[1] != 0 {
		t.Fatalf("pour score exceeding worst-case balance was accepted: %v", game.data.PourScores)
	}
}

func TestTrustedPourUsesConfiguredMinimumAndAdvances(t *testing.T) {
	room := testutil.NewPlayingRoom(2)
	game := NewGameFrame(proto.GameRule{MaxPlayerCount: 2, BaseScore: 2, CanPourScores: []int{2, 4}, MaxCanPourGold: 4}, room, nil)
	game.data.GameStarted = true
	game.data.BankerChairID = 0
	game.data.UserTrustArray[1] = true
	game.startPour(&remote.Session{})
	defer game.stop()
	if game.data.PourScores[1] != 4 || game.data.GameStatus != StatusShow {
		t.Fatalf("trusted pour did not use the configured minimum or advance: score=%d status=%d", game.data.PourScores[1], game.data.GameStatus)
	}
}

func TestSettlementDoesNotApplyBaseScoreTwice(t *testing.T) {
	room := testutil.NewPlayingRoom(2)
	game := NewGameFrame(proto.GameRule{MaxPlayerCount: 2, BaseScore: 3}, room, nil)
	game.data.GameStarted = true
	game.data.BankerChairID = 0
	game.data.RobBankScales[0] = 1
	game.data.PourScores[1] = 6
	game.dealt[0] = []int{0x0d, 0x0c, 0x0b}
	game.dealt[1] = []int{0x01, 0x12, 0x23}
	game.finish(&remote.Session{})
	wins := game.data.Result.(map[string]any)["winScores"].([]int)
	want := 6 * Evaluate(game.dealt[1], false).Scale
	if wins[0] < 0 {
		want = -want
	}
	if wins[0] != want {
		t.Fatalf("base score was applied more than once: got %d want %d", wins[0], want)
	}
	if len(game.reviewRecord) != 1 || len(game.reviewRecord[0]) != 2 {
		t.Fatalf("review history = %#v, want one bureau with two players", game.reviewRecord)
	}
	player := game.reviewRecord[0][1]
	if player.Uid != "b" || player.PourScore != 6 || player.WinScore != wins[1] ||
		player.Rob != -1 || player.IsBanker || player.CardType != int(Evaluate(game.dealt[1], false).Type) {
		t.Fatalf("player review = %#v, want authoritative settlement snapshot", player)
	}
	game.GameMessageHandle(room.Users["a"], &remote.Session{}, []byte(`{"type":312,"data":{}}`))
	message := room.Direct[len(room.Direct)-1].(map[string]any)
	data := message["data"].(map[string]any)
	if message["type"] != ReviewPush || len(data["list"].([][]*BureauReview)) != 1 {
		t.Fatalf("review push = %#v, want grouped history", message)
	}
}

func TestSettlementReturnsToLegacyReadyState(t *testing.T) {
	room := testutil.NewPlayingRoom(2)
	room.CurBureau = 1
	game := NewGameFrame(proto.GameRule{MaxPlayerCount: 2}, room, nil)
	game.resultDelay = 0
	game.data.GameStarted = true
	game.data.BankerChairID = 0
	game.data.RobBankScales[0] = 1
	game.data.PourScores[1] = 1
	game.dealt[0] = []int{0x0d, 0x0c, 0x0b}
	game.dealt[1] = []int{0x01, 0x12, 0x23}

	game.finish(&remote.Session{})

	if game.data.GameStatus != StatusNone || game.data.GameStarted || game.data.Result != nil {
		t.Fatalf("settled round state = status %d, started %v, result %#v; want legacy ready state", game.data.GameStatus, game.data.GameStarted, game.data.Result)
	}
	if !hasPushType(room.Broadcasts, StatusPush) {
		t.Fatalf("settled round did not push status none: %#v", room.Broadcasts)
	}
	last := room.Broadcasts[len(room.Broadcasts)-1].(map[string]any)
	data := last["data"].(map[string]any)
	if last["type"] != StatusPush || data["gameStatus"] != StatusNone {
		t.Fatalf("last push = %#v, want status none", last)
	}
}

func TestSettlementWaitsForDismissalRejectionBeforeReturningToReadyState(t *testing.T) {
	room := testutil.NewPlayingRoom(2)
	room.CurBureau = 1
	room.SetDismissing(true)
	game := NewGameFrame(proto.GameRule{MaxPlayerCount: 2}, room, nil)
	game.resultDelay = 0
	game.data.GameStarted = true
	game.data.BankerChairID = 0
	game.data.RobBankScales[0] = 1
	game.data.PourScores[1] = 1
	game.dealt[0] = []int{0x0d, 0x0c, 0x0b}
	game.dealt[1] = []int{0x01, 0x12, 0x23}
	defer game.stop()

	game.finish(&remote.Session{})
	time.Sleep(3 * roundResetRetryDelay)
	if game.data.GameStatus != StatusResult {
		t.Fatalf("settlement reset during dismissal: status=%d", game.data.GameStatus)
	}

	room.SetDismissing(false)
	waitForSettlementStatusPush(t, room, StatusNone)
	last := room.Broadcasts[len(room.Broadcasts)-1].(map[string]any)
	data := last["data"].(map[string]any)
	if last["type"] != StatusPush || data["gameStatus"] != StatusNone {
		t.Fatalf("last push = %#v, want status none after dismissal rejection", last)
	}
}

func waitForSettlementStatusPush(t *testing.T, room *testutil.Room, status int) {
	t.Helper()
	deadline := time.Now().Add(250 * time.Millisecond)
	for time.Now().Before(deadline) {
		if len(room.Broadcasts) > 0 {
			last, ok := room.Broadcasts[len(room.Broadcasts)-1].(map[string]any)
			data, dataOK := last["data"].(map[string]any)
			if ok && dataOK && last["type"] == StatusPush && data["gameStatus"] == status {
				return
			}
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("status %d push did not arrive: %#v", status, room.Broadcasts)
}

func TestBigEatSmallModePushesInitialBanker(t *testing.T) {
	room := testutil.NewPlayingRoom(2)
	game := NewGameFrame(proto.GameRule{MaxPlayerCount: 2, GameFrameType: 2, CanPourScores: []int{1}, MaxCanPourGold: 1}, room, nil)
	game.OnEventGameStart(nil, &remote.Session{})
	defer game.stop()
	if game.data.BankerChairID != 0 || !hasPushType(room.Broadcasts, BankerPush) {
		t.Fatalf("initial banker was not announced: banker=%d pushes=%#v", game.data.BankerChairID, room.Broadcasts)
	}
}

func TestRoundBankerAdvancesAcrossValidSeats(t *testing.T) {
	room := testutil.NewPlayingRoom(3)
	game := NewGameFrame(proto.GameRule{MaxPlayerCount: 3, GameFrameType: gameModeRoundBanker, CanPourScores: []int{1}, MaxCanPourGold: 1}, room, nil)
	users := game.players()
	if banker, _ := game.bankerForMode(users); banker != 0 {
		t.Fatalf("first round banker = %d, want 0", banker)
	}
	game.data.BankerChairID = 0
	if banker, _ := game.bankerForMode(users); banker != 1 {
		t.Fatalf("second round banker = %d, want 1", banker)
	}
	delete(room.Users, "b")
	users = game.players()
	if banker, _ := game.bankerForMode(users); banker != 2 {
		t.Fatalf("round banker did not skip an empty seat: got %d want 2", banker)
	}
	game.data.BankerChairID = 2
	if banker, _ := game.bankerForMode(users); banker != 0 {
		t.Fatalf("round banker did not wrap: got %d want 0", banker)
	}
}

func TestRoundBankerModePushesNextBanker(t *testing.T) {
	room := testutil.NewPlayingRoom(3)
	game := NewGameFrame(proto.GameRule{MaxPlayerCount: 3, GameFrameType: gameModeRoundBanker, CanPourScores: []int{1}, MaxCanPourGold: 1}, room, nil)
	game.data.BankerChairID = 0
	game.OnEventGameStart(nil, &remote.Session{})
	defer game.stop()
	if game.data.BankerChairID != 1 || !hasPushType(room.Broadcasts, BankerPush) {
		t.Fatalf("next round banker was not announced: banker=%d pushes=%#v", game.data.BankerChairID, room.Broadcasts)
	}
}

func TestRobBankerPushIncludesRobChairIDs(t *testing.T) {
	room := testutil.NewPlayingRoom(2)
	game := NewGameFrame(proto.GameRule{MaxPlayerCount: 2, CanPourScores: []int{1}, MaxCanPourGold: 1}, room, nil)
	game.data.GameStarted = true
	game.data.GameStatus = StatusRob
	game.rob(room.Users["a"], 2, &remote.Session{})
	game.rob(room.Users["b"], 2, &remote.Session{})
	found := false
	for _, item := range room.Broadcasts {
		message, ok := item.(map[string]any)
		if !ok || message["type"] != BankerPush {
			continue
		}
		data := message["data"].(map[string]any)
		chairs := data["robChairIDs"].([]int)
		found = len(chairs) == 2 && chairs[0] == 0 && chairs[1] == 1
	}
	if !found {
		t.Fatalf("banker push did not include tied robbers: %#v", room.Broadcasts)
	}
}

func hasPushType(pushes []any, wanted int) bool {
	for _, item := range pushes {
		if message, ok := item.(map[string]any); ok && message["type"] == wanted {
			return true
		}
	}
	return false
}
