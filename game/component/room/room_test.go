package room

import (
	"math"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"core/models/entity"
	"core/models/enums"
	"framework/msError"
	"game/component/proto"
)

func TestGetEmptyChairIDSkipsAllOccupiedChairs(t *testing.T) {
	r := &Room{
		chairCount: 4,
		users: map[string]*proto.RoomUser{
			"u0": {ChairID: 0},
			"u1": {ChairID: 1},
			"u3": {ChairID: 3},
		},
	}
	if got := r.getEmptyChairID("", false); got != 2 {
		t.Fatalf("empty chair = %d, want 2", got)
	}
}

func TestIsStartGameCountsReadyBitWithOtherStatusFlags(t *testing.T) {
	r := &Room{
		chairCount: 2,
		GameRule:   proto.GameRule{GameType: enums.NN, MinPlayerCount: 2, MaxPlayerCount: 2},
		users: map[string]*proto.RoomUser{
			"u0": {ChairID: 0, UserStatus: enums.Ready | enums.Dismiss},
			"u1": {ChairID: 1, UserStatus: enums.Ready | enums.Dismiss},
		},
	}
	if !r.IsStartGame() {
		t.Fatal("ready players with additional status flags were not counted")
	}
}

func TestNewGameFrameRejectsInvalidPourRules(t *testing.T) {
	tests := []struct {
		name string
		rule proto.GameRule
		want string
	}{
		{name: "nn score", rule: proto.GameRule{GameType: enums.NN, CanPourScores: []int{0, 2}}, want: "canPourScores"},
		{name: "sz missing add score", rule: proto.GameRule{GameType: enums.SZ}, want: "addScores"},
		{name: "sz invalid add score", rule: proto.GameRule{GameType: enums.SZ, AddScores: []int{1, 0}}, want: "addScores"},
		{name: "nn push scale", rule: proto.GameRule{GameType: enums.NN, CanPourScores: []int{1, 2}, TuiScale: []int{-1}}, want: "tuiScale"},
		{name: "sg maximum", rule: proto.GameRule{GameType: enums.SG, CanPourScores: []int{5, 20}, MaxCanPourGold: 4}, want: "maxCanPourGold"},
		{name: "dgn initial pool", rule: validDGNRule(func(rule *proto.GameRule) { rule.Shouzhuang = 0 }), want: "shouzhuang"},
		{name: "dgn first rate", rule: validDGNRule(func(rule *proto.GameRule) { rule.FirstBureauRate = 0 }), want: "firstBureauRate"},
		{name: "dgn later rate", rule: validDGNRule(func(rule *proto.GameRule) { rule.BureauRate = -0.1 }), want: "bureauRate"},
		{name: "dgn non-finite rate", rule: validDGNRule(func(rule *proto.GameRule) { rule.FirstBureauRate = math.NaN() }), want: "firstBureauRate"},
		{name: "dgn first minimum", rule: validDGNRule(func(rule *proto.GameRule) { rule.FirstBureauRate = 0.34 }), want: "firstBureauRate"},
		{name: "dgn later minimum", rule: validDGNRule(func(rule *proto.GameRule) { rule.BureauRate = 0.5 }), want: "bureauRate"},
		{name: "dgn leave score", rule: validDGNRule(func(rule *proto.GameRule) { rule.Xiazhuangfen = 0 }), want: "xiazhuangfen"},
		{name: "dgn banker mode", rule: validDGNRule(func(rule *proto.GameRule) { rule.LianzhuangType = 3 }), want: "lianzhuangType"},
		{name: "dgn banker count", rule: validDGNRule(func(rule *proto.GameRule) { rule.LianzhuangCount = 0 }), want: "lianzhuangCount"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewGameFrame(tc.rule, nil, nil)
			if err == nil {
				t.Fatalf("invalid pour rule was accepted: %#v", tc.rule)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %q, want field %q", err, tc.want)
			}
		})
	}
}

func TestValidateRoomConfigRejectsMalformedRoom(t *testing.T) {
	creator := &proto.RoomCreator{}
	valid := proto.GameRule{MinPlayerCount: 1, MaxPlayerCount: 2}
	tests := []struct {
		name    string
		roomID  string
		creator *proto.RoomCreator
		rule    proto.GameRule
		want    string
	}{
		{name: "missing room id", roomID: "", creator: creator, rule: valid, want: "room id"},
		{name: "missing creator", roomID: "room-1", creator: nil, rule: valid, want: "creator"},
		{name: "zero max players", roomID: "room-1", creator: creator, rule: proto.GameRule{MinPlayerCount: 1}, want: "maxPlayerCount"},
		{name: "zero min players", roomID: "room-1", creator: creator, rule: proto.GameRule{MaxPlayerCount: 2}, want: "minPlayerCount"},
		{name: "min exceeds max", roomID: "room-1", creator: creator, rule: proto.GameRule{MinPlayerCount: 3, MaxPlayerCount: 2}, want: "minPlayerCount"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := validateRoomConfig(tc.roomID, tc.creator, tc.rule); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("validateRoomConfig error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestNewGameFrameAcceptsValidDGNRule(t *testing.T) {
	rule := validDGNRule(nil)
	if _, err := NewGameFrame(rule, nil, nil); err != nil {
		t.Fatalf("valid DGN rule was rejected: %v", err)
	}
	rule.FirstBureauRate = 1.0 / 3.0
	rule.BureauRate = 1.0 / 3.0
	if _, err := NewGameFrame(rule, nil, nil); err != nil {
		t.Fatalf("boundary DGN rule was rejected: %v", err)
	}
}

func TestNewGameFrameRejectsUnknownGameType(t *testing.T) {
	if frame, err := NewGameFrame(proto.GameRule{GameType: enums.GameType(999)}, nil, nil); err == nil || frame != nil {
		t.Fatalf("unknown game type was accepted: frame=%T err=%v", frame, err)
	}
}

func TestNewGameFrameRejectsUnsupportedPlayerCounts(t *testing.T) {
	tests := []struct {
		name string
		rule proto.GameRule
		want string
	}{
		{name: "pdk too many", rule: proto.GameRule{GameType: enums.PDK, MaxPlayerCount: 4}, want: "maxPlayerCount"},
		{name: "mahjong too many", rule: proto.GameRule{GameType: enums.ZNMJ, MaxPlayerCount: 5}, want: "maxPlayerCount"},
		{name: "nn too few", rule: proto.GameRule{GameType: enums.NN, MaxPlayerCount: 1}, want: "maxPlayerCount"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := NewGameFrame(tc.rule, nil, nil); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestNewGameFrameRejectsUnsupportedMinimumPlayerCounts(t *testing.T) {
	tests := []proto.GameRule{
		{GameType: enums.PDK, MinPlayerCount: 1, MaxPlayerCount: 3},
		{GameType: enums.ZNMJ, MinPlayerCount: 1, MaxPlayerCount: 4},
		{GameType: enums.SZ, MinPlayerCount: 1, MaxPlayerCount: 6},
	}
	for _, rule := range tests {
		if _, err := NewGameFrame(rule, nil, nil); err == nil || !strings.Contains(err.Error(), "minPlayerCount") {
			t.Fatalf("rule %#v error = %v, want minPlayerCount validation", rule, err)
		}
	}
}

func TestNewGameFrameRejectsUnsupportedGameModes(t *testing.T) {
	tests := []proto.GameRule{
		{GameType: enums.NN, GameFrameType: 2},
		{GameType: enums.NN, GameFrameType: 4},
		{GameType: enums.SG, GameFrameType: 3},
		{GameType: enums.PDK, GameFrameType: 4},
		{GameType: enums.ZNMJ, GameFrameType: 3},
	}
	for _, rule := range tests {
		if _, err := NewGameFrame(rule, nil, nil); err == nil || !strings.Contains(err.Error(), "gameFrameType") {
			t.Fatalf("rule %#v error = %v, want gameFrameType validation", rule, err)
		}
	}
}

func TestNewGameFrameAcceptsNNMingPaiQiangZhuang(t *testing.T) {
	frame, err := NewGameFrame(proto.GameRule{GameType: enums.NN, GameFrameType: 6}, nil, nil)
	if err != nil || frame == nil {
		t.Fatalf("NN ming pai qiang zhuang was rejected: frame=%T err=%v", frame, err)
	}
}

func validDGNRule(change func(*proto.GameRule)) proto.GameRule {
	rule := proto.GameRule{
		GameType:        enums.DGN,
		Shouzhuang:      300,
		FirstBureauRate: 0.2,
		BureauRate:      0.1,
		Xiazhuangfen:    50,
		LianzhuangType:  1,
		LianzhuangCount: 1,
	}
	if change != nil {
		change(&rule)
	}
	return rule
}

func TestGetEmptyChairIDReturnsMinusOneWhenFull(t *testing.T) {
	r := &Room{
		chairCount: 2,
		users: map[string]*proto.RoomUser{
			"u0": {ChairID: 0},
			"u1": {ChairID: 1},
		},
	}
	if got := r.getEmptyChairID("", false); got != -1 {
		t.Fatalf("full room empty chair = %d, want -1", got)
	}
}

func TestGetEmptyChairIDAllocatesWatcherSlots(t *testing.T) {
	r := &Room{
		chairCount: 2,
		users: map[string]*proto.RoomUser{
			"u0": {ChairID: 2},
			"u1": {ChairID: 3},
		},
	}
	if got := r.getEmptyChairID("", true); got != 4 {
		t.Fatalf("empty watcher chair = %d, want 4", got)
	}
}

func TestHasEmptyChairIgnoresWatchers(t *testing.T) {
	r := &Room{
		chairCount: 4,
		users: map[string]*proto.RoomUser{
			"player":   {ChairID: 0},
			"watcher1": {ChairID: 4},
			"watcher2": {ChairID: 5},
		},
	}
	if !r.HasEmptyChair() {
		t.Fatal("watchers must not consume formal seats")
	}
}

func TestUserEntryRoomDoesNotReenterRoomLockWhenFull(t *testing.T) {
	r := &Room{
		chairCount: 1,
		users: map[string]*proto.RoomUser{
			"existing": {ChairID: 0, UserInfo: &proto.UserInfo{Uid: "existing"}},
		},
		GameRule: proto.GameRule{MinPlayerCount: 1, MaxPlayerCount: 1},
	}
	done := make(chan *msError.Error, 1)
	go func() {
		done <- r.UserEntryRoom(nil, &entity.User{Uid: "new-user"})
	}()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("full room accepted a new user")
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("full-room entry appears to be deadlocked")
	}
}

func TestGetUsersReturnsMapSnapshot(t *testing.T) {
	room := &Room{
		users: map[string]*proto.RoomUser{
			"u0": {ChairID: 0},
		},
	}
	snapshot := room.GetUsers()
	delete(snapshot, "u0")
	snapshot["u1"] = &proto.RoomUser{ChairID: 1}

	room.RLock()
	defer room.RUnlock()
	if len(room.users) != 1 || room.users["u0"] == nil || room.users["u1"] != nil {
		t.Fatalf("mutating users snapshot changed room map: %#v", room.users)
	}
}

func TestGetUsersFiltersInvalidEntries(t *testing.T) {
	room := &Room{
		users: map[string]*proto.RoomUser{
			"nil-user":   nil,
			"nil-info":   {},
			"valid-user": {UserInfo: &proto.UserInfo{Uid: "valid-user"}},
		},
	}
	users := room.GetUsers()
	if len(users) != 1 || users["valid-user"] == nil {
		t.Fatalf("filtered users = %#v, want only valid-user", users)
	}
}

func TestAllUsersReturnsStableUserIDs(t *testing.T) {
	room := &Room{
		users: map[string]*proto.RoomUser{
			"u0":  {UserInfo: &proto.UserInfo{Uid: "u0"}},
			"nil": nil,
		},
	}
	users := room.AllUsers()
	if len(users) != 1 || users[0] != "u0" {
		t.Fatalf("all users = %#v, want [u0]", users)
	}
}

func TestAllUsersLockedCanBeCalledWhileHoldingRoomLock(t *testing.T) {
	room := &Room{users: map[string]*proto.RoomUser{
		"u0": {UserInfo: &proto.UserInfo{Uid: "u0"}},
		"u1": {UserInfo: &proto.UserInfo{Uid: "u1"}},
	}}
	room.Lock()
	users := room.allUsersLocked()
	room.Unlock()
	if len(users) != 2 {
		t.Fatalf("users = %#v, want two users", users)
	}
}

func TestAskForDismissIgnoresMissingUser(t *testing.T) {
	r := &Room{users: map[string]*proto.RoomUser{}}
	r.askForDismiss(nil, "missing", true)
}

func TestDismissUserArraysUseChairCount(t *testing.T) {
	r := &Room{
		chairCount: 4,
		users: map[string]*proto.RoomUser{
			"u2": {
				ChairID:    2,
				UserStatus: enums.Dismiss,
				UserInfo:   &proto.UserInfo{Nickname: "player", Avatar: "avatar"},
			},
			"watcher": {ChairID: 4, UserStatus: enums.Dismiss, UserInfo: &proto.UserInfo{Nickname: "watcher"}},
			"nil":     nil,
		},
	}
	names, avatars, online := dismissUserArrays(r.users, r.chairCount)
	if len(names) != 4 || len(avatars) != 4 || len(online) != 4 {
		t.Fatalf("dismiss arrays lengths = %d/%d/%d, want 4", len(names), len(avatars), len(online))
	}
	if names[2] != "player" || avatars[2] != "avatar" || !online[2] {
		t.Fatalf("chair 2 dismiss data = %q/%q/%v", names[2], avatars[2], online[2])
	}
}

func TestCancelKickScheduleStopsTimer(t *testing.T) {
	var fired atomic.Bool
	r := &Room{kickSchedules: map[string]*time.Timer{}}
	r.kickSchedules["user"] = time.AfterFunc(20*time.Millisecond, func() {
		fired.Store(true)
	})
	r.cancelKickScheduleLocked("user")
	time.Sleep(50 * time.Millisecond)
	if fired.Load() {
		t.Fatal("cancelled kick timer still fired")
	}
	if len(r.kickSchedules) != 0 {
		t.Fatalf("kick schedules length = %d, want 0", len(r.kickSchedules))
	}
}

func TestRollbackNewUserEntryRestoresCapacity(t *testing.T) {
	r := &Room{
		users:            map[string]*proto.RoomUser{"new": {UserInfo: &proto.UserInfo{Uid: "new"}}},
		currentUserCount: 1,
	}
	r.rollbackNewUserEntryLocked("new")
	if len(r.users) != 0 || r.currentUserCount != 0 {
		t.Fatalf("rolled back room state = users:%d count:%d", len(r.users), r.currentUserCount)
	}
}

func TestAllPlayedUsersBuildsWritableSettlementMap(t *testing.T) {
	r := &Room{
		users: map[string]*proto.RoomUser{
			"active": {UserInfo: &proto.UserInfo{Uid: "active"}},
		},
		clearUserArr: map[string]*entity.GameUser{
			"left": {Uid: "left", Nickname: "left-user", Score: 12},
		},
		alreadyCostUserUidArr: []string{"active", "left"},
	}
	played := r.allPlayedUsersLocked()
	if len(played) != 2 || played["active"] == nil || played["left"] == nil {
		t.Fatalf("played users = %#v, want active and left", played)
	}
}

func TestRunGameActionSerializesCallbacks(t *testing.T) {
	r := &Room{}
	var active atomic.Int32
	var maximum atomic.Int32
	var wait sync.WaitGroup
	for i := 0; i < 20; i++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			r.RunGameAction(func() {
				current := active.Add(1)
				if current > maximum.Load() {
					maximum.Store(current)
				}
				time.Sleep(time.Millisecond)
				active.Add(-1)
			})
		}()
	}
	wait.Wait()
	if maximum.Load() != 1 {
		t.Fatalf("maximum concurrent game actions = %d, want 1", maximum.Load())
	}
}

func TestGetEmptyChairIDWaitsForWriter(t *testing.T) {
	r := &Room{chairCount: 1, users: map[string]*proto.RoomUser{}}
	r.Lock()
	result := make(chan int, 1)
	go func() { result <- r.getEmptyChairID("", false) }()
	select {
	case <-result:
		r.Unlock()
		t.Fatal("seat lookup bypassed the room write lock")
	case <-time.After(20 * time.Millisecond):
	}
	r.Unlock()
	select {
	case got := <-result:
		if got != 0 {
			t.Fatalf("empty chair = %d, want 0", got)
		}
	case <-time.After(time.Second):
		t.Fatal("seat lookup did not resume after unlock")
	}
}

func TestConcurrentDismissVoteIsRecordedOnce(t *testing.T) {
	r := &Room{
		chairCount: 2,
		users: map[string]*proto.RoomUser{
			"u0": {ChairID: 0, UserStatus: enums.Dismiss, UserInfo: &proto.UserInfo{Uid: "u0"}},
			"u1": {ChairID: 1, UserStatus: enums.Dismiss, UserInfo: &proto.UserInfo{Uid: "u1"}},
		},
	}
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r.askForDismiss(nil, "u0", true)
		}()
	}
	wg.Wait()
	r.Lock()
	defer r.Unlock()
	if len(r.askDismiss) != 2 || r.askDismiss[0] != true || r.askDismiss[1] != nil {
		t.Fatalf("dismiss votes = %#v, want [true nil]", r.askDismiss)
	}
	if r.answerExitSchedule == nil {
		t.Fatal("dismiss timer was not started")
	}
	r.stopAnswerScheduleLocked()
}

func TestDismissRejectionStopsVoteTimer(t *testing.T) {
	r := &Room{
		chairCount: 2,
		users: map[string]*proto.RoomUser{
			"u0": {ChairID: 0, UserStatus: enums.Dismiss, UserInfo: &proto.UserInfo{Uid: "u0"}},
			"u1": {ChairID: 1, UserStatus: enums.Dismiss, UserInfo: &proto.UserInfo{Uid: "u1"}},
		},
	}
	r.askForDismiss(nil, "u0", true)
	r.askForDismiss(nil, "u1", false)
	r.Lock()
	defer r.Unlock()
	if r.askDismiss != nil {
		t.Fatalf("dismiss votes = %#v, want nil after rejection", r.askDismiss)
	}
	if r.answerExitSchedule != nil {
		t.Fatal("dismiss timer was not stopped after rejection")
	}
}

func TestBeginDismissAllowsOnlyOneCallerPerGeneration(t *testing.T) {
	r := &Room{}
	generation := r.roomGeneration.Load()
	var winners atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r.Lock()
			if r.beginDismissLocked(generation) {
				winners.Add(1)
			}
			r.Unlock()
		}()
	}
	wg.Wait()
	if got := winners.Load(); got != 1 {
		t.Fatalf("dismiss winners = %d, want 1", got)
	}
	r.Lock()
	r.roomDismissed = false
	r.roomGeneration.Add(1)
	if r.beginDismissLocked(generation) {
		r.Unlock()
		t.Fatal("stale generation was allowed to dismiss a reset room")
	}
	r.Unlock()
}

func TestKickUserIgnoresStaleUserPointer(t *testing.T) {
	current := &proto.RoomUser{ChairID: 0, UserInfo: &proto.UserInfo{Uid: "user"}}
	stale := &proto.RoomUser{ChairID: 0, UserInfo: &proto.UserInfo{Uid: "user"}}
	r := &Room{
		users:            map[string]*proto.RoomUser{"user": current},
		currentUserCount: 1,
		kickSchedules:    map[string]*time.Timer{},
	}
	r.kickUserLocked(stale, nil)
	if r.users["user"] != current || r.currentUserCount != 1 {
		t.Fatal("stale user pointer removed the current room user")
	}
}
