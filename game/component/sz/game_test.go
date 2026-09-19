package sz

import (
	"framework/remote"
	"framework/stream"
	"game/component/internal/testutil"
	"game/component/proto"
	"testing"
	"time"
)

func TestGameVideoDataUsesReviewRecords(t *testing.T) {
	records := [][]*BureauReview{{{}}}
	frame := &GameFrame{ReviewRecord: records}
	data, ok := frame.GetGameVideoData().([][]*BureauReview)
	if !ok || len(data) != 1 || len(data[0]) != 1 || data[0][0] != records[0][0] {
		t.Fatalf("GetGameVideoData() = %#v, want review records", frame.GetGameVideoData())
	}
}

func TestSpectatorCannotMutateTrustState(t *testing.T) {
	room := testutil.NewPlayingRoom(3)
	game := &GameFrame{r: room, gameData: &GameData{ChairCount: 3, UserTrustArray: make([]bool, 3), TrustTmArray: make([]int, 3)}}
	spectator := &proto.RoomUser{ChairID: -1, UserInfo: &proto.UserInfo{Uid: "watcher"}}
	game.GameMessageHandle(spectator, &remote.Session{}, []byte(`{"type":315,"data":{"trust":true}}`))
	if len(room.Broadcasts) != 0 {
		t.Fatalf("spectator trust request was broadcast: %#v", room.Broadcasts)
	}
}

func TestSpectatorOfflineEventIsIgnored(t *testing.T) {
	room := testutil.NewPlayingRoom(3)
	game := &GameFrame{r: room, gameData: &GameData{ChairCount: 3, CurChairID: 9, UserTrustArray: make([]bool, 3), TrustTmArray: make([]int, 3)}}
	spectator := &proto.RoomUser{ChairID: 9, UserInfo: &proto.UserInfo{Uid: "watcher"}}
	game.OnEventUserOffLine(spectator, &remote.Session{})
	if len(room.Broadcasts) != 0 {
		t.Fatalf("spectator offline event produced pushes: %#v", room.Broadcasts)
	}
}

func TestTrustRequestRespectsRoomRule(t *testing.T) {
	room := testutil.NewPlayingRoom(3)
	game := &GameFrame{r: room, gameRule: proto.GameRule{CanTrust: false}, gameData: &GameData{ChairCount: 3, UserTrustArray: make([]bool, 3), TrustTmArray: make([]int, 3)}}
	game.GameMessageHandle(room.Users["a"], &remote.Session{}, []byte(`{"type":315,"data":{"trust":true}}`))
	if game.gameData.UserTrustArray[0] || len(room.Broadcasts) != 0 {
		t.Fatalf("disabled trust was accepted: state=%v pushes=%#v", game.gameData.UserTrustArray, room.Broadcasts)
	}
}

func TestEnterSnapshotRejectsMissingSessionOrUser(t *testing.T) {
	room := testutil.NewPlayingRoom(2)
	game := &GameFrame{r: room, gameData: initGameData(proto.GameRule{MaxPlayerCount: 2})}
	if data := game.GetEnterGameData(nil); data != nil {
		t.Fatalf("nil session snapshot = %#v, want nil", data)
	}
	if data := game.GetEnterGameData(remote.NewSession(nil, &stream.Msg{Uid: "missing"})); data != nil {
		t.Fatalf("unknown user snapshot = %#v, want nil", data)
	}
}

func TestSpectatorReconnectHidesAllCards(t *testing.T) {
	room := testutil.NewPlayingRoom(2)
	room.Users["watcher"] = &proto.RoomUser{ChairID: -1, UserInfo: &proto.UserInfo{Uid: "watcher"}}
	gameData := initGameData(proto.GameRule{MaxPlayerCount: 2})
	gameData.GameStatus = PourScore
	gameData.HandCards = [][]int{{1, 2, 3}, {4, 5, 6}}
	game := &GameFrame{r: room, gameRule: proto.GameRule{MaxPlayerCount: 2}, gameData: gameData, UserWinRecord: make(map[string]*UserWinRecord)}

	snapshot := game.GetEnterGameData(remote.NewSession(nil, &stream.Msg{Uid: "watcher"})).(GameData)
	if snapshot.SelfChairID != -1 {
		t.Fatalf("spectator snapshot self chair = %d, want -1", snapshot.SelfChairID)
	}
	for chairID, cards := range snapshot.HandCards {
		if len(cards) != 3 || cards[0] != 0 || cards[1] != 0 || cards[2] != 0 {
			t.Fatalf("spectator saw chair %d cards %v", chairID, cards)
		}
	}
}

func TestDelScheduleIDsStopsEveryDelayedTransition(t *testing.T) {
	game := &GameFrame{}
	timers := []*time.Timer{
		time.AfterFunc(time.Hour, func() {}),
		time.AfterFunc(time.Hour, func() {}),
		time.AfterFunc(time.Hour, func() {}),
		time.AfterFunc(time.Hour, func() {}),
		time.AfterFunc(time.Hour, func() {}),
	}
	game.startPourScoreID = timers[0]
	game.sendCardsScheduleID = timers[1]
	game.compareID = timers[2]
	game.endResultID = timers[3]
	game.abandonID = timers[4]

	game.delScheduleIDs()
	if game.startPourScoreID != nil || game.sendCardsScheduleID != nil || game.compareID != nil || game.endResultID != nil || game.abandonID != nil {
		t.Fatal("delScheduleIDs retained a delayed transition timer")
	}
	for index, timer := range timers {
		if timer.Stop() {
			t.Fatalf("timer %d was still active after delScheduleIDs", index)
		}
	}
}

func TestResetGameClearsResultAndStarterState(t *testing.T) {
	room := testutil.NewPlayingRoom(2)
	gameData := initGameData(proto.GameRule{MaxPlayerCount: 2})
	gameData.GameStarter = true
	gameData.GameStatus = Result
	gameData.Result = &GameResult{Winners: []int{1}}
	gameData.Winner = []int{1}
	game := &GameFrame{r: room, gameData: gameData}

	game.resetGame(&remote.Session{})
	if game.gameData.GameStarter || game.gameData.GameStatus != GameStatusNone || game.gameData.Result != nil || len(game.gameData.Winner) != 0 {
		t.Fatalf("reset retained prior bureau state: %#v", game.gameData)
	}
}

func TestResultSnapshotContainsAuthoritativeResult(t *testing.T) {
	room := testutil.NewPlayingRoom(2)
	gameData := initGameData(proto.GameRule{MaxPlayerCount: 2})
	result := &GameResult{Winners: []int{1}, WinScores: []int{-3, 3}, HandCards: [][]int{{1, 2, 3}, {4, 5, 6}}}
	gameData.GameStatus = Result
	gameData.Result = result
	gameData.HandCards = result.HandCards
	game := &GameFrame{r: room, gameRule: proto.GameRule{MaxPlayerCount: 2}, gameData: gameData, UserWinRecord: make(map[string]*UserWinRecord)}

	snapshot := game.GetEnterGameData(remote.NewSession(nil, &stream.Msg{Uid: "a"})).(GameData)
	if snapshot.SelfChairID != 0 || snapshot.Result != result || snapshot.HandCards[1][0] != 4 {
		t.Fatalf("result reconnect snapshot = %#v, want complete result and revealed hands", snapshot)
	}
}

func TestScoreTotalsUseStakeValues(t *testing.T) {
	pourScores := [][]int{{2, 5}, {3}, nil}
	if got := sumScores(pourScores[0]); got != 7 {
		t.Fatalf("sumScores() = %d, want 7", got)
	}
	if got := totalPourScores(pourScores); got != 10 {
		t.Fatalf("totalPourScores() = %d, want 10", got)
	}
}

func TestSettleScoresCreditsActualWinnerChair(t *testing.T) {
	pourScores := [][]int{{1, 2}, {4}, {5, 6}}
	got := settleScores(pourScores, []int{2}, 3)
	want := []int{-3, -4, 7}
	for chairID := range want {
		if got[chairID] != want[chairID] {
			t.Fatalf("settleScores() = %v, want %v", got, want)
		}
	}
}

func TestSettleScoresPreservesTotalWhenTiedPotHasRemainder(t *testing.T) {
	got := settleScores([][]int{{2, 3}, {1}, {1}}, []int{1, 2}, 3)
	want := []int{-5, 3, 2}
	total := 0
	for chairID := range want {
		total += got[chairID]
		if got[chairID] != want[chairID] {
			t.Fatalf("settleScores() = %v, want %v", got, want)
		}
	}
	if total != 0 {
		t.Fatalf("settleScores() total = %d, want zero-sum result", total)
	}
}
