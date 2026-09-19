package dgn

import (
	"core/models/enums"
	"framework/remote"
	"game/component/internal/testutil"
	"game/component/proto"
	"testing"
	"time"
)

func TestTimeoutContinuesAcrossAllPendingBankerChoices(t *testing.T) {
	game := NewGameFrame(proto.GameRule{MaxPlayerCount: 3}, testutil.NewPlayingRoom(3), nil)
	game.data.GameStarted = true
	game.data.GameStatus = StatusChooseBank
	game.timeout = time.Millisecond
	game.schedule(&remote.Session{})
	defer game.stop()

	deadline := time.Now().Add(250 * time.Millisecond)
	for time.Now().Before(deadline) {
		if game.data.GameStatus != StatusChooseBank {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("timeout automation stopped before every pending player was handled")
}

func TestGameStartPromptsBankerChoiceWithPushProtocol(t *testing.T) {
	room := testutil.NewPlayingRoom(3)
	game := NewGameFrame(proto.GameRule{MaxPlayerCount: 3}, room, nil)
	game.OnEventGameStart(nil, &remote.Session{})
	defer game.stop()

	foundPrompt := false
	for _, value := range room.Broadcasts {
		message, ok := value.(map[string]any)
		if !ok || message["type"] != ChooseBankPush {
			continue
		}
		data, ok := message["data"].(map[string]any)
		if ok && data["chairID"] == 0 {
			foundPrompt = true
		}
	}
	if !foundPrompt {
		t.Fatalf("game start pushes = %#v, want choose-bank prompt for chair 0", room.Broadcasts)
	}
	for _, value := range room.Direct {
		if message, ok := value.(map[string]any); ok && message["type"] == ChooseBankNotify {
			t.Fatalf("server sent client request protocol %d: %#v", ChooseBankNotify, value)
		}
	}
}

func TestGameRecordsReturnCardSnapshots(t *testing.T) {
	game := &GameFrame{dealt: [][]int{{1, 2, 3, 4, 5}}}

	video := game.GetGameVideoData().([][]int)
	video[0][0] = 99
	bureau := game.GetGameBureauData().([][]int)
	if bureau[0][0] != 1 {
		t.Fatalf("game records exposed internal cards: %v", game.dealt)
	}
}

func TestChooseBankRejectsOutOfTurnPlayer(t *testing.T) {
	room := testutil.NewPlayingRoom(3)
	game := NewGameFrame(proto.GameRule{MaxPlayerCount: 3}, room, nil)
	game.data.GameStarted = true
	game.data.GameStatus = StatusChooseBank
	game.choose(room.Users["b"], true, &remote.Session{})
	if game.choices[1] != -1 {
		t.Fatalf("out-of-turn choice was accepted: %#v", game.choices)
	}
}

func TestBankerChoiceSequenceSurvivesReconnect(t *testing.T) {
	room := testutil.NewPlayingRoom(3)
	game := NewGameFrame(proto.GameRule{MaxPlayerCount: 3}, room, nil)
	game.data.GameStarted = true
	game.data.GameStatus = StatusChooseBank
	game.promptBankerChoice(&remote.Session{})
	game.choose(room.Users["a"], false, &remote.Session{})
	defer game.stop()

	if game.data.NextBankerChairID != 1 {
		t.Fatalf("next banker chair = %d, want 1", game.data.NextBankerChairID)
	}
	snapshot := game.GetEnterGameData(nil).(*GameData)
	if snapshot.NextBankerChairID != 1 {
		t.Fatalf("reconnect next banker chair = %d, want 1", snapshot.NextBankerChairID)
	}
}

func TestSpectatorCannotMutateTrustButCanChat(t *testing.T) {
	room := testutil.NewPlayingRoom(3)
	game := NewGameFrame(proto.GameRule{MaxPlayerCount: 3}, room, nil)
	game.data.GameStarted = true
	spectator := &proto.RoomUser{ChairID: 9, UserInfo: &proto.UserInfo{Uid: "watcher"}}
	game.GameMessageHandle(spectator, &remote.Session{}, []byte(`{"type":311,"data":{"trust":true}}`))
	if len(room.Broadcasts) != 0 {
		t.Fatalf("spectator trust request was broadcast: %#v", room.Broadcasts)
	}
	game.GameMessageHandle(spectator, &remote.Session{}, []byte(`{"type":310,"data":{"type":1,"msg":"hello","recipientID":-1}}`))
	if len(room.Broadcasts) != 1 {
		t.Fatalf("spectator chat broadcasts = %d, want 1", len(room.Broadcasts))
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

func TestBankerPoolAndShemenSurviveReconnect(t *testing.T) {
	room := testutil.NewPlayingRoom(2)
	game := NewGameFrame(proto.GameRule{MaxPlayerCount: 2, Shouzhuang: 300}, room, nil)
	game.data.GameStarted = true
	game.data.GameStatus = StatusChooseBank
	game.choices[0] = 1
	game.choices[1] = 0
	game.selectBank(&remote.Session{})
	defer game.stop()

	if game.data.PoolScore != 300 || game.data.PourScores[0] != 300 {
		t.Fatalf("selected banker pool = %d, pours=%v, want 300 mirrored at chair 0", game.data.PoolScore, game.data.PourScores)
	}
	game.pour(room.Users["b"], 50, true, &remote.Session{})
	snapshot := game.GetEnterGameData(nil).(*GameData)
	if !snapshot.ShemenArray[1] || snapshot.PoolScore != 300 {
		t.Fatalf("reconnect lost shemen or pool state: %#v", snapshot)
	}
	if data := lastPushData(room.Broadcasts, PourPush); data == nil || data["shemen"] != true {
		t.Fatalf("shemen pour push missing: %#v", room.Broadcasts)
	}
}

func TestPourUsesConfiguredRatesAndAuthoritativePool(t *testing.T) {
	room := testutil.NewPlayingRoom(3)
	game := NewGameFrame(proto.GameRule{MaxPlayerCount: 3, Shouzhuang: 100, FirstBureauRate: 0.1, BureauRate: 0.05}, room, nil)
	game.data.GameStarted = true
	game.data.GameStatus = StatusPour
	game.data.BankerChairID = 0
	game.data.PoolScore = 90
	game.data.PourScores[0] = 999

	game.pour(room.Users["b"], 9, false, &remote.Session{})
	if game.data.PourScores[1] != 0 {
		t.Fatalf("bet below first-bureau minimum was accepted: %v", game.data.PourScores)
	}
	game.pour(room.Users["b"], 10, false, &remote.Session{})
	if game.data.PourScores[1] != 10 {
		t.Fatalf("configured first-bureau minimum was rejected: %v", game.data.PourScores)
	}
	game.data.BankerPourTurn = 2
	game.pour(room.Users["c"], 31, false, &remote.Session{})
	if game.data.PourScores[2] != 0 {
		t.Fatalf("bet above authoritative pool third was accepted: %v", game.data.PourScores)
	}
	game.pour(room.Users["c"], 5, false, &remote.Session{})
	defer game.stop()
	if game.data.PourScores[2] != 5 {
		t.Fatalf("configured later-bureau minimum was rejected: %v", game.data.PourScores)
	}
}

func TestResultReturnsUpdatedPoolInsteadOfBetSum(t *testing.T) {
	room := testutil.NewPlayingRoom(2)
	game := NewGameFrame(proto.GameRule{MaxPlayerCount: 2, BaseScore: 1, ScaleType: 1}, room, nil)
	game.data.GameStarted = true
	game.data.GameStatus = StatusShow
	game.data.BankerChairID = 0
	game.data.PoolScore = 100
	game.data.PourScores[0] = 100
	game.data.PourScores[1] = 7
	game.data.ShemenArray[1] = true
	game.dealt[0] = []int{2, 3, 4, 5, 7}
	game.dealt[1] = []int{0x0a, 0x1a, 0x05, 0x15, 0x2a}

	game.finish(&remote.Session{})
	result, ok := game.data.Result.(map[string]any)
	if !ok {
		t.Fatalf("result type = %T, want map", game.data.Result)
	}
	wins := result["winScores"].([]int)
	wantPool := 100 + wins[0]
	if result["poolScore"] != wantPool || game.data.PoolScore != wantPool || result["poolScore"] == sum([]int{100, 7}) {
		t.Fatalf("pool result=%v state=%d wins=%v, want updated pool %d instead of bet sum", result["poolScore"], game.data.PoolScore, wins, wantPool)
	}
	if game.data.PourScores[0] != wantPool || game.data.PourScores[1] != 0 {
		t.Fatalf("post-result pool mirror = %v, want [%d 0]", game.data.PourScores, wantPool)
	}
	if len(game.reviewRecord) != 1 || len(game.reviewRecord[0]) != 2 {
		t.Fatalf("review history = %#v, want one bureau with two players", game.reviewRecord)
	}
	banker, player := game.reviewRecord[0][0], game.reviewRecord[0][1]
	if !banker.IsBanker || banker.PourScore != 100 || player.PourScore != 7 || player.WinScore != wins[1] ||
		player.CardType != int(Evaluate(game.dealt[1], game.rule.CardsType, game.rule.ScaleType).Type) {
		t.Fatalf("review snapshot banker=%#v player=%#v", banker, player)
	}
	game.GameMessageHandle(room.Users["a"], &remote.Session{}, []byte(`{"type":313,"data":{}}`))
	message := room.Direct[len(room.Direct)-1].(map[string]any)
	data := message["data"].(map[string]any)
	if message["type"] != ReviewPush || len(data["list"].([][]*BureauReview)) != 1 {
		t.Fatalf("review push = %#v, want grouped history", message)
	}
}

func TestResultClearsPoolBelowDownBankerMinimum(t *testing.T) {
	room := testutil.NewPlayingRoom(2)
	game := NewGameFrame(proto.GameRule{MaxPlayerCount: 2, BaseScore: 1, ScaleType: 1, Xiazhuangfen: 50}, room, nil)
	game.data.GameStarted = true
	game.data.GameStatus = StatusShow
	game.data.BankerChairID = 0
	game.data.PoolScore = 40
	game.data.PourScores[0] = 40
	game.data.PourScores[1] = 1
	game.dealt[0] = []int{2, 3, 4, 5, 7}
	game.dealt[1] = []int{2, 3, 4, 5, 7}

	game.finish(&remote.Session{})
	result := game.data.Result.(map[string]any)
	if game.data.PoolScore != 0 || result["poolScore"] != 0 {
		t.Fatalf("pool was not cleared below down-banker minimum: state=%d result=%v", game.data.PoolScore, result["poolScore"])
	}
	if !hasPushType(room.Broadcasts, ClearPoolPush) {
		t.Fatalf("clear-pool push missing: %#v", room.Broadcasts)
	}
}

func TestSettlementReturnsToLegacyReadyStateAndPreservesBankerPool(t *testing.T) {
	room := testutil.NewPlayingRoom(2)
	room.CurBureau = 1
	game := NewGameFrame(proto.GameRule{MaxPlayerCount: 2, Bureau: 10, BaseScore: 1, ScaleType: 1}, room, nil)
	game.resultDelay = 0
	game.data.GameStarted = true
	game.data.GameStatus = StatusShow
	game.data.BankerChairID = 0
	game.data.BankerTurn = 2
	game.data.BankerPourTurn = 3
	game.data.PoolScore = 100
	game.data.PourScores[0] = 100
	game.data.PourScores[1] = 5
	game.data.ShowCards[0], game.data.ShowCards[1] = true, true
	game.data.ShemenArray[1] = true
	game.data.UserTrustArray[1] = true
	game.dealt[0] = []int{2, 3, 4, 5, 7}
	game.dealt[1] = []int{2, 3, 4, 5, 7}

	game.finish(&remote.Session{})

	if game.data.GameStatus != StatusNone || game.data.GameStarted || game.data.Result != nil {
		t.Fatalf("settled state = status %d, started %v, result %#v; want legacy ready state", game.data.GameStatus, game.data.GameStarted, game.data.Result)
	}
	if game.data.BankerChairID != 0 || game.data.BankerTurn != 2 || game.data.BankerPourTurn != 3 || game.data.PoolScore <= 0 || game.data.PourScores[0] != game.data.PoolScore {
		t.Fatalf("banker state was not preserved: %#v", game.data)
	}
	if !game.data.UserTrustArray[1] || game.data.ShowCards[1] || game.data.ShemenArray[1] || game.dealt[1] != nil {
		t.Fatalf("round fields were not reset while trust survived: %#v", game.data)
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
	game := NewGameFrame(proto.GameRule{MaxPlayerCount: 2, Bureau: 10, BaseScore: 1, ScaleType: 1}, room, nil)
	game.resultDelay = 0
	game.data.GameStarted = true
	game.data.GameStatus = StatusShow
	game.data.BankerChairID = 0
	game.data.PoolScore = 100
	game.data.PourScores[0] = 100
	game.data.PourScores[1] = 1
	game.dealt[0] = []int{2, 3, 4, 5, 7}
	game.dealt[1] = []int{2, 3, 4, 5, 7}
	defer game.stop()

	game.finish(&remote.Session{})
	time.Sleep(3 * roundResetRetryDelay)
	if game.data.GameStatus != StatusResult {
		t.Fatalf("settlement reset during dismissal: status=%d", game.data.GameStatus)
	}

	room.SetDismissing(false)
	deadline := time.Now().Add(250 * time.Millisecond)
	for time.Now().Before(deadline) && game.data.GameStatus != StatusNone {
		time.Sleep(time.Millisecond)
	}
	if game.data.GameStatus != StatusNone {
		t.Fatalf("settlement did not recover after dismissal rejection: status=%d", game.data.GameStatus)
	}
}

func TestFinalSettlementDismissesWithBureauFinishedAfterResult(t *testing.T) {
	room := testutil.NewPlayingRoom(2)
	room.CurBureau = room.MaxBureau
	game := NewGameFrame(proto.GameRule{MaxPlayerCount: 2, Bureau: room.MaxBureau, BaseScore: 1, ScaleType: 1}, room, nil)
	game.resultDelay = 0
	game.data.GameStarted = true
	game.data.GameStatus = StatusShow
	game.data.BankerChairID = 0
	game.data.PoolScore = 100
	game.data.PourScores[0] = 100
	game.data.PourScores[1] = 1
	game.dealt[0] = []int{2, 3, 4, 5, 7}
	game.dealt[1] = []int{2, 3, 4, 5, 7}

	game.finish(&remote.Session{})

	if len(room.Dismissals) != 1 || room.Dismissals[0] != enums.BureauFinished {
		t.Fatalf("dismiss reasons = %#v, want BureauFinished", room.Dismissals)
	}
	if game.data.GameStatus != StatusResult || game.data.Result == nil {
		t.Fatalf("final result was reset before room dismissal: status=%d result=%#v", game.data.GameStatus, game.data.Result)
	}
	last := room.Broadcasts[len(room.Broadcasts)-1].(map[string]any)
	if last["type"] != ResultPush {
		t.Fatalf("last game push = %#v, want result before dismissal", last)
	}
}

func TestShengzhuangCarriesWinningBankerUntilConfiguredLimit(t *testing.T) {
	room := testutil.NewPlayingRoom(2)
	game := NewGameFrame(proto.GameRule{MaxPlayerCount: 2, Shouzhuang: 100, LianzhuangType: 1, LianzhuangCount: 2}, room, nil)
	game.data.BankerChairID = 0
	game.data.BankerTurn = 1
	game.data.PoolScore = 100
	if banker, turn := game.nextBureauBanker(0, 10, false); banker != 0 || turn != 2 {
		t.Fatalf("winning banker was not carried: banker=%d turn=%d", banker, turn)
	}
	game.data.BankerTurn = 2
	if banker, turn := game.nextBureauBanker(0, 10, false); banker != -1 || turn != 0 {
		t.Fatalf("banker exceeded configured consecutive turns: banker=%d turn=%d", banker, turn)
	}
}

func TestLunzhuangAdvancesToNextValidChair(t *testing.T) {
	room := testutil.NewPlayingRoom(3)
	delete(room.Users, "b")
	game := NewGameFrame(proto.GameRule{MaxPlayerCount: 3, LianzhuangType: 2, LianzhuangCount: 2}, room, nil)
	if banker, turn := game.nextBureauBanker(0, 10, false); banker != 2 || turn != 1 {
		t.Fatalf("round banker did not skip empty chair: banker=%d turn=%d", banker, turn)
	}
	if banker, turn := game.nextBureauBanker(2, 10, false); banker != 0 || turn != 1 {
		t.Fatalf("round banker did not wrap: banker=%d turn=%d", banker, turn)
	}
}

func TestClearedPoolCancelsPendingBanker(t *testing.T) {
	room := testutil.NewPlayingRoom(2)
	game := NewGameFrame(proto.GameRule{MaxPlayerCount: 2, LianzhuangType: 1, LianzhuangCount: 2}, room, nil)
	game.pendingBanker = 0
	game.pendingBankerTurn = 2
	if banker, turn := game.nextBureauBanker(0, 10, true); banker != -1 || turn != 0 {
		t.Fatalf("cleared pool retained banker: banker=%d turn=%d", banker, turn)
	}
}

func TestAutomaticBankerChangePreservesPool(t *testing.T) {
	room := testutil.NewPlayingRoom(2)
	game := NewGameFrame(proto.GameRule{MaxPlayerCount: 2, Shouzhuang: 100}, room, nil)
	game.data.PoolScore = 137
	game.startWithBanker(1, 1, true, &remote.Session{})
	defer game.stop()
	if game.data.PoolScore != 137 {
		t.Fatalf("automatic banker change reset pool to %d, want 137", game.data.PoolScore)
	}
}

func hasPushType(pushes []any, wanted int) bool {
	for _, item := range pushes {
		message, ok := item.(map[string]any)
		if ok && message["type"] == wanted {
			return true
		}
	}
	return false
}

func lastPushData(pushes []any, typ int) map[string]any {
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
