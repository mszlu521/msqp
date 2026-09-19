package logic

import (
	"core/models/entity"
	"core/models/enums"
	"game/component/proto"
	"game/component/room"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"math"
	"reflect"
	"strconv"
	"sync"
	"testing"
	"time"
)

func TestCreateRoomIDIsSixDigitsAndConcurrentSafe(t *testing.T) {
	manager := NewUnionManager()
	const count = 200
	ids := make(chan string, count)
	var wait sync.WaitGroup
	for i := 0; i < count; i++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			ids <- manager.CreateRoomId()
		}()
	}
	wait.Wait()
	close(ids)
	seen := make(map[string]bool, count)
	for id := range ids {
		if len(id) != 6 {
			t.Fatalf("room id %q is not six digits", id)
		}
		if _, err := strconv.Atoi(id); err != nil {
			t.Fatalf("room id %q is not numeric", id)
		}
		if seen[id] {
			t.Fatalf("duplicate room id generated: %s", id)
		}
		seen[id] = true
	}
}

func TestCreateRoomIDUsesTemporaryReservation(t *testing.T) {
	manager := NewUnionManager()
	id := manager.CreateRoomId()
	manager.RLock()
	_, reserved := manager.reservedRoomIDs[id]
	manager.RUnlock()
	if !reserved {
		t.Fatalf("generated room id %s was not reserved", id)
	}
	manager.releaseRoomID(id)
	manager.RLock()
	_, reserved = manager.reservedRoomIDs[id]
	manager.RUnlock()
	if reserved {
		t.Fatalf("room id %s reservation was not released", id)
	}
}

func TestUnionIsShouldDeleteAfterIdleTimeout(t *testing.T) {
	u := &Union{
		RoomList:   map[string]*room.Room{},
		activeTime: time.Now().Add(-2 * time.Minute),
	}
	if !u.IsShouldDelete(int64(time.Minute / time.Millisecond)) {
		t.Fatal("idle union should be eligible for deletion")
	}
}

func TestUnionIsShouldDeleteKeepsActiveRoom(t *testing.T) {
	u := &Union{
		RoomList:   map[string]*room.Room{"room": nil},
		activeTime: time.Now().Add(-2 * time.Hour),
	}
	if u.IsShouldDelete(int64(time.Minute / time.Millisecond)) {
		t.Fatal("union with a room must not be deleted")
	}
}

func TestGetUnionInfoHandlesMissingUnionData(t *testing.T) {
	u := &Union{Id: 42}
	info := u.GetUnionInfo("u1")
	if info.UnionID != 42 {
		t.Fatalf("missing union info = %#v", info)
	}
}

func TestGetGameRuleRestoresRoomRuleMetadata(t *testing.T) {
	id := primitive.NewObjectID()
	u := &Union{unionData: &entity.Union{
		RoomRuleList: []*entity.RoomRule{{
			Id:       id,
			GameType: int(enums.PDK),
			RuleName: "legacy pdk",
			GameRule: `{"maxPlayerCount":3,"minPlayerCount":2}`,
		}},
	}}

	rule := u.GetGameRule(id.Hex())
	if rule.Id != id.Hex() {
		t.Fatalf("rule id = %q, want %q", rule.Id, id.Hex())
	}
	if rule.GameType != enums.PDK {
		t.Fatalf("rule game type = %d, want %d", rule.GameType, enums.PDK)
	}
	if rule.RuleName != "legacy pdk" {
		t.Fatalf("rule name = %q, want %q", rule.RuleName, "legacy pdk")
	}
}

func TestGetGameRuleRejectsMalformedRule(t *testing.T) {
	id := primitive.NewObjectID()
	u := &Union{unionData: &entity.Union{
		RoomRuleList: []*entity.RoomRule{{Id: id, GameRule: "{"}},
	}}

	if rule := u.GetGameRule(id.Hex()); !reflect.DeepEqual(rule, proto.GameRule{}) {
		t.Fatalf("malformed rule = %#v, want empty rule", rule)
	}
}

func TestGetGameRuleReturnsEmptyForUnknownID(t *testing.T) {
	u := &Union{unionData: &entity.Union{RoomRuleList: []*entity.RoomRule{nil}}}
	if rule := u.GetGameRule(primitive.NewObjectID().Hex()); !reflect.DeepEqual(rule, proto.GameRule{}) {
		t.Fatalf("unknown rule = %#v, want empty rule", rule)
	}
}

func TestRoomRuleWritesRejectUnserializableRules(t *testing.T) {
	u := &Union{}
	rule := proto.GameRule{FirstBureauRate: math.NaN()}

	if err := u.AddRoomRuleList(rule, "invalid", int(enums.DGN)); err == nil {
		t.Fatal("AddRoomRuleList accepted a rule with NaN")
	}
	if err := u.UpdateRoomRuleList(primitive.NewObjectID().Hex(), rule, "invalid", int(enums.DGN)); err == nil {
		t.Fatal("UpdateRoomRuleList accepted a rule with NaN")
	}
}

func TestRoomRulePullUpdateUsesEmbeddedObjectID(t *testing.T) {
	id := primitive.NewObjectID()
	update, err := roomRulePullUpdate(id.Hex())
	if err != nil {
		t.Fatal(err)
	}
	pull, ok := update["$pull"].(primitive.M)
	if !ok {
		t.Fatalf("pull update = %#v", update)
	}
	condition, ok := pull["roomRuleList"].(primitive.M)
	if !ok || condition["_id"] != id {
		t.Fatalf("room rule pull condition = %#v, want ObjectID %s", condition, id.Hex())
	}
	if _, err := roomRulePullUpdate("not-an-object-id"); err == nil {
		t.Fatal("roomRulePullUpdate accepted an invalid object id")
	}
}
