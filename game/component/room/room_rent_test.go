package room

import (
	"common/config"
	"common/database"
	"common/logs"
	"context"
	"core/models/entity"
	"core/models/enums"
	"core/repo"
	"core/service"
	"fmt"
	"game/component/proto"
	"os"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type rentTestUnion struct{}

func (rentTestUnion) DestroyRoom(string)  {}
func (rentTestUnion) GetOwnerUid() string { return "owner" }
func (rentTestUnion) IsOpening() bool     { return true }

// Use real balance updates in a disposable database, never application users.
func TestRoomRentMongo(t *testing.T) {
	uri := os.Getenv("MSQP_RANK_TEST_MONGO_URI")
	if uri == "" {
		t.Skip("set MSQP_RANK_TEST_MONGO_URI to run balance regressions")
	}
	originalConfig := config.Conf
	config.Conf = &config.Config{}
	logs.InitLog("rent-regression")
	t.Cleanup(func() { config.Conf = originalConfig })
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Disconnect(context.Background())
	db := client.Database(fmt.Sprintf("msqp_room_rent_regression_%d", time.Now().UnixNano()))
	defer db.Drop(context.Background())
	userService := service.NewUserService(&repo.Manager{Mongo: &database.MongoManager{Cli: client, Db: db}})
	seed := func(t *testing.T) {
		t.Helper()
		if _, err := db.Collection("user").DeleteMany(ctx, bson.M{}); err != nil {
			t.Fatal(err)
		}
		for _, uid := range []string{"owner", "alice", "bob", "viewer"} {
			if _, err := db.Collection("user").InsertOne(ctx, &entity.User{Uid: uid, Gold: 100}); err != nil {
				t.Fatal(err)
			}
		}
	}
	gold := func(t *testing.T, uid string) int64 {
		t.Helper()
		var user entity.User
		if err := db.Collection("user").FindOne(ctx, bson.M{"uid": uid}).Decode(&user); err != nil {
			t.Fatal(err)
		}
		return user.Gold
	}
	newRoom := func(creator enums.CreatorType, pay enums.RoomPayType) *Room {
		return &Room{RoomCreator: &proto.RoomCreator{Uid: "alice", CreatorType: creator},
			GameRule:   proto.GameRule{GameType: enums.NN, Bureau: 10, PayType: pay, PayDiamond: 2},
			chairCount: 2, UserService: userService, union: rentTestUnion{},
			users: map[string]*proto.RoomUser{
				"alice":  {ChairID: 0, UserInfo: &proto.UserInfo{Uid: "alice", Gold: 100}},
				"bob":    {ChairID: 1, UserInfo: &proto.UserInfo{Uid: "bob", Gold: 100}},
				"viewer": {ChairID: 2, UserInfo: &proto.UserInfo{Uid: "viewer", Gold: 100}},
			},
		}
	}
	t.Run("union owner pays actual seated users once regardless of private payType", func(t *testing.T) {
		for _, pay := range []enums.RoomPayType{0, enums.AAZhiFu, enums.MyPay} {
			seed(t)
			r := newRoom(enums.UnionCreatorType, pay)
			if err := r.collectionRoomRentWhenStart(nil); err != nil {
				t.Fatal(err)
			}
			if got := gold(t, "owner"); got != 98 {
				t.Fatalf("owner gold = %d, want 98", got)
			}
			if got := gold(t, "alice"); got != 100 {
				t.Fatalf("member gold = %d, want 100", got)
			}
			r.curBureau = 1
			if err := r.collectionRoomRentWhenStart(nil); err != nil {
				t.Fatal(err)
			}
			if got := gold(t, "owner"); got != 98 {
				t.Fatalf("repeat charge, owner gold = %d", got)
			}
			r.chairCount = 3
			if err := r.collectionRoomRentWhenStart(nil); err != nil {
				t.Fatal(err)
			}
			if got := gold(t, "owner"); got != 97 {
				t.Fatalf("late seated user, owner gold = %d, want 97", got)
			}
		}
	})
	t.Run("private AA excludes spectators and does not charge each small round", func(t *testing.T) {
		seed(t)
		r := newRoom(enums.UserCreatorType, enums.AAZhiFu)
		if err := r.collectionRoomRentWhenStart(nil); err != nil {
			t.Fatal(err)
		}
		if got := gold(t, "viewer"); got != 100 {
			t.Fatalf("spectator gold = %d, want 100", got)
		}
		if got := gold(t, "alice"); got != 98 {
			t.Fatalf("player gold = %d, want 98", got)
		}
		r.curBureau = 1
		if err := r.collectionRoomRentWhenStart(nil); err != nil {
			t.Fatal(err)
		}
		if got := gold(t, "alice"); got != 98 {
			t.Fatalf("repeat charge, player gold = %d", got)
		}
	})
	t.Run("private owner pays once", func(t *testing.T) {
		seed(t)
		r := newRoom(enums.UserCreatorType, enums.MyPay)
		for i := 0; i < 3; i++ {
			r.curBureau = i
			if err := r.collectionRoomRentWhenStart(nil); err != nil {
				t.Fatal(err)
			}
		}
		if got := gold(t, "alice"); got != 98 {
			t.Fatalf("owner gold = %d, want 98", got)
		}
		if got := gold(t, "bob"); got != 100 {
			t.Fatalf("guest gold = %d, want 100", got)
		}
	})
	t.Run("insufficient union balance rejects start", func(t *testing.T) {
		seed(t)
		if _, err := db.Collection("user").UpdateOne(ctx, bson.M{"uid": "owner"}, bson.M{"$set": bson.M{"gold": 1}}); err != nil {
			t.Fatal(err)
		}
		r := newRoom(enums.UnionCreatorType, enums.AAZhiFu)
		if err := r.collectionRoomRentWhenStart(nil); err == nil {
			t.Fatal("insufficient owner balance accepted")
		}
		if got := gold(t, "owner"); got != 1 {
			t.Fatalf("failed charge changed balance to %d", got)
		}
	})
	t.Run("failed AA start restores all charged balances", func(t *testing.T) {
		seed(t)
		if _, err := db.Collection("user").UpdateOne(ctx, bson.M{"uid": "bob"}, bson.M{"$set": bson.M{"gold": 1}}); err != nil {
			t.Fatal(err)
		}
		r := newRoom(enums.UserCreatorType, enums.AAZhiFu)
		if err := r.collectionRoomRentWhenStart(nil); err == nil {
			t.Fatal("insufficient AA balance accepted")
		}
		if got := gold(t, "alice"); got != 100 {
			t.Fatalf("partial AA charge left alice with %d", got)
		}
		if got := gold(t, "bob"); got != 1 {
			t.Fatalf("failed AA charge changed bob to %d", got)
		}
		if len(r.rentPaidUserUIDs) != 0 {
			t.Fatal("failed charge marked users as paid")
		}
	})
	t.Run("a new match in a reused union room is charged again", func(t *testing.T) {
		seed(t)
		r := newRoom(enums.UnionCreatorType, enums.AAZhiFu)
		if err := r.collectionRoomRentWhenStart(nil); err != nil {
			t.Fatal(err)
		}
		seatedUsers := r.users
		r.users = map[string]*proto.RoomUser{}
		r.GameRule.CanPourScores = []int{1, 2}
		r.alreadyCostUserUidArr = []string{"alice", "bob"}
		r.userJoinGameBureau = map[string]int{"alice": 10}
		if err := r.resetRoom(nil); err != nil {
			t.Fatal(err)
		}
		if len(r.alreadyCostUserUidArr) != 0 || len(r.userJoinGameBureau) != 0 {
			t.Fatal("new match retained previous participants")
		}
		r.users = seatedUsers
		if err := r.collectionRoomRentWhenStart(nil); err != nil {
			t.Fatal(err)
		}
		if got := gold(t, "owner"); got != 96 {
			t.Fatalf("second match owner gold = %d, want 96", got)
		}
	})
	t.Run("all six games and all payment modes", func(t *testing.T) {
		for _, game := range []struct {
			kind   enums.GameType
			rounds []int
			costs  []int
		}{
			{enums.SZ, []int{6, 10, 12, 15, 20}, []int{1, 2, 2, 3, 4}},
			{enums.NN, []int{10, 20, 30}, []int{1, 2, 3}},
			{enums.PDK, []int{10, 20}, []int{1, 2}},
			{enums.SG, []int{10, 20, 30}, []int{1, 2, 3}},
			{enums.ZNMJ, []int{8, 16}, []int{1, 2}},
			{enums.DGN, []int{10, 20, 30}, []int{1, 2, 3}},
		} {
			for index, bureau := range game.rounds {
				for _, pay := range []enums.RoomPayType{enums.AAZhiFu, enums.MyPay, enums.WinZhiFu} {
					t.Run(fmt.Sprintf("game-%d/bureau-%d/pay-%d", game.kind, bureau, pay), func(t *testing.T) {
						seed(t)
						r := newRoom(enums.UserCreatorType, pay)
						r.GameRule.GameType, r.GameRule.Bureau = game.kind, bureau
						r.GameRule.PayDiamond = game.costs[index]
						if err := r.collectionRoomRentWhenStart(nil); err != nil {
							t.Fatal(err)
						}
						r.curBureau = 1
						if err := r.collectionRoomRentWhenStart(nil); err != nil {
							t.Fatal(err)
						}
						if pay == enums.WinZhiFu && (gold(t, "alice") != 100 || gold(t, "bob") != 100) {
							t.Fatal("winner charged before settlement")
						}
						r.hasFinishedOneBureau = true
						r.alreadyCostUserUidArr = []string{"alice", "bob"}
						r.users["alice"].WinScore = -10
						r.users["bob"].WinScore = 10
						for call := 0; call < 2; call++ {
							if err := r.collectionRoomRentWhenEnd(nil); err != nil {
								t.Fatal(err)
							}
						}
						alice, bob := int64(100), int64(100)
						if pay == enums.AAZhiFu || pay == enums.MyPay {
							alice -= int64(game.costs[index])
						}
						if pay == enums.AAZhiFu || pay == enums.WinZhiFu {
							bob -= int64(game.costs[index])
						}
						if gold(t, "alice") != alice || gold(t, "bob") != bob || gold(t, "viewer") != 100 || gold(t, "owner") != 100 {
							t.Fatal("incorrect payer or repeated charge")
						}
						if r.users["alice"].UserInfo.Gold != alice || r.users["bob"].UserInfo.Gold != bob {
							t.Fatal("room balance differs from stored balance")
						}
					})
				}
				seed(t)
				r := newRoom(enums.UnionCreatorType, enums.WinZhiFu)
				r.GameRule.GameType, r.GameRule.Bureau = game.kind, bureau
				if err := r.collectionRoomRentWhenStart(nil); err != nil {
					t.Fatal(err)
				}
				if got, want := gold(t, "owner"), 100-int64(2*game.costs[index]); got != want {
					t.Fatalf("union owner gold=%d, want %d", got, want)
				}
			}
		}
	})
	t.Run("tied winners split exactly one fee once", func(t *testing.T) {
		for _, tc := range []struct {
			name               string
			fee, score         int
			wantAlice, wantBob int64
		}{
			{"equal shares", 4, 10, 98, 98},
			{"remainder goes to lowest UID", 3, 10, 98, 99},
			{"fee smaller than winner count", 1, 10, 99, 100},
			{"all players draw", 3, 0, 98, 99},
		} {
			t.Run(tc.name, func(t *testing.T) {
				seed(t)
				r := newRoom(enums.UserCreatorType, enums.WinZhiFu)
				r.GameRule.PayDiamond = tc.fee
				r.hasFinishedOneBureau = true
				r.alreadyCostUserUidArr = []string{"bob", "alice"}
				r.users["alice"].WinScore, r.users["bob"].WinScore = tc.score, tc.score
				for call := 0; call < 2; call++ {
					if err := r.collectionRoomRentWhenEnd(nil); err != nil {
						t.Fatal(err)
					}
				}
				if gold(t, "alice") != tc.wantAlice || gold(t, "bob") != tc.wantBob || gold(t, "viewer") != 100 || !r.creatorRentPaid {
					t.Fatal("incorrect split, duplicate charge, or spectator charged")
				}
				if r.users["alice"].UserInfo.Gold != tc.wantAlice || r.users["bob"].UserInfo.Gold != tc.wantBob {
					t.Fatal("room balance differs from stored split")
				}
			})
		}
	})
	t.Run("tied winner insufficient funds leaves all balances unchanged", func(t *testing.T) {
		seed(t)
		if _, err := db.Collection("user").UpdateOne(ctx, bson.M{"uid": "bob"}, bson.M{"$set": bson.M{"gold": 0}}); err != nil {
			t.Fatal(err)
		}
		r := newRoom(enums.UserCreatorType, enums.WinZhiFu)
		r.GameRule.PayDiamond = 3
		r.hasFinishedOneBureau = true
		r.alreadyCostUserUidArr = []string{"alice", "bob"}
		if err := r.collectionRoomRentWhenEnd(nil); err == nil {
			t.Fatal("insufficient shared payment accepted")
		}
		if gold(t, "alice") != 100 || gold(t, "bob") != 0 || r.creatorRentPaid {
			t.Fatal("partial shared payment was retained")
		}
	})
	t.Run("three tied winners include a participant who left", func(t *testing.T) {
		seed(t)
		r := newRoom(enums.UserCreatorType, enums.WinZhiFu)
		r.GameRule.PayDiamond = 4
		r.hasFinishedOneBureau = true
		r.alreadyCostUserUidArr = []string{"owner", "bob", "alice"}
		r.clearUserArr = map[string]*entity.GameUser{"owner": {Uid: "owner", Score: 0}}
		if err := r.collectionRoomRentWhenEnd(nil); err != nil {
			t.Fatal(err)
		}
		if gold(t, "alice") != 98 || gold(t, "bob") != 99 || gold(t, "owner") != 99 || gold(t, "viewer") != 100 {
			t.Fatal("three-way split excluded a participant or exceeded one fee")
		}
	})
	t.Run("unplayed winner-pay room is free", func(t *testing.T) {
		seed(t)
		r := newRoom(enums.UserCreatorType, enums.WinZhiFu)
		if err := r.collectionRoomRentWhenEnd(nil); err != nil {
			t.Fatal(err)
		}
		if gold(t, "alice") != 100 || gold(t, "bob") != 100 {
			t.Fatal("unplayed room was charged")
		}
	})
}
