package handler

import (
	"common"
	"common/database"
	"context"
	"core/dao"
	"core/repo"
	"encoding/json"
	"fmt"
	"framework/remote"
	"framework/stream"
	"os"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func TestUnionRankEmptyResponse(t *testing.T) {
	for _, pages := range [][]unionRankPage{nil, {{}}} {
		data, err := json.Marshal(unionRankPayload(pages))
		if err != nil || string(data) != `{"recordArr":[],"totalCount":0}` {
			t.Fatalf("empty rank response: %s, %v", data, err)
		}
	}
}

// Run against a separate, disposable database on the local test Mongo server.
// No application accounts, unions or game records are touched.
func TestUnionRankMongo(t *testing.T) {
	uri := os.Getenv("MSQP_RANK_TEST_MONGO_URI")
	if uri == "" {
		t.Skip("set MSQP_RANK_TEST_MONGO_URI to run the Mongo aggregation regression")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Disconnect(context.Background())
	db := client.Database(fmt.Sprintf("msqp_rank_regression_%d", time.Now().UnixNano()))
	defer func() {
		cleanup, done := context.WithTimeout(context.Background(), 10*time.Second)
		defer done()
		if err := db.Drop(cleanup); err != nil {
			t.Errorf("remove disposable regression database: %v", err)
		}
	}()

	query := func(collection string, pipeline mongo.Pipeline) map[string]any {
		t.Helper()
		cursor, err := db.Collection(collection).Aggregate(ctx, pipeline)
		if err != nil {
			t.Fatal(err)
		}
		defer cursor.Close(ctx)
		var pages []unionRankPage
		if err := cursor.All(ctx, &pages); err != nil {
			t.Fatal(err)
		}
		return unionRankPayload(pages)
	}
	_, err = db.Collection("user").InsertMany(ctx, []any{
		bson.M{"uid": "alice", "nickname": "Alice", "unionInfo": []any{bson.M{"unionID": 1, "score": 12, "yesterdayWin": -5, "todayDraw": 2, "weekDraw": 3, "totalDraw": 4}, bson.M{"unionID": 2, "score": 999}}},
		bson.M{"uid": "bob", "unionInfo": []any{bson.M{"unionID": 1, "score": 25, "yesterdayWin": 10}}},
		bson.M{"uid": "other", "unionInfo": []any{bson.M{"unionID": 2, "score": 1000}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		field string
		uid   string
	}{{"score", "bob"}, {"yesterdayWin", "bob"}, {"totalDraw", "alice"}, {"todayDraw", "alice"}, {"weekDraw", "alice"}} {
		page := query("user", unionRankPipeline(1, nil, bson.M{"unionInfo." + tt.field: -1}, 0, 1))
		rows := page["recordArr"].([]unionRankRecord)
		if page["totalCount"] != int64(2) || len(rows) != 1 || rows[0].Uid != tt.uid {
			t.Fatalf("%s rank: %#v", tt.field, page)
		}
	}
	page := query("user", unionRankPipeline(1, nil, bson.M{"unionInfo.score": -1}, 1, 1))
	row := page["recordArr"].([]unionRankRecord)[0]
	if row.Score != 12 || row.YesterdayWin != -5 || row.TotalDraw != 4 || row.Uid != "alice" {
		t.Fatalf("manager fields lost: %#v", row)
	}
	if page := query("user", unionRankPipeline(9, nil, nil, 0, 10)); page["totalCount"] != int64(0) || len(page["recordArr"].([]unionRankRecord)) != 0 {
		t.Fatalf("empty rank: %#v", page)
	}

	record := func(union, game, created int, alice, bob int) bson.M {
		return bson.M{"unionID": union, "gameType": game, "createTime": created, "userList": []any{
			bson.M{"uid": "alice", "nickname": "Alice", "avatar": "Common/head_icon_default", "score": alice},
			bson.M{"uid": "bob", "nickname": "Bob", "score": bob},
		}}
	}
	_, err = db.Collection("userGameRecord").InsertMany(ctx, []any{
		record(1, 3, 100, 10, -10), record(1, 3, 101, -20, 20),
		record(2, 3, 101, 999, -999), record(1, 4, 101, 888, -888), record(1, 3, 99, 777, -777),
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		direction float64 // encoding/json decodes the client's sort direction as float64.
		uid       string
		score     int64
	}{{-1, "bob", 20}, {1, "alice", -20}} {
		page := query("userGameRecord", unionSingleRankPipeline(1, 100, bson.M{"gameType": 3}, bson.M{"score": tt.direction}, 0, 1))
		rows := page["recordArr"].([]unionRankRecord)
		if page["totalCount"] != int64(2) || len(rows) != 1 || rows[0].Uid != tt.uid || rows[0].Score != tt.score || rows[0].Nickname == "" {
			t.Fatalf("single draw rank %v: %#v", tt.direction, page)
		}
	}
	_, err = db.Collection("union").InsertOne(ctx, bson.M{"unionID": 1, "ownerUid": "alice", "showRank": true, "showSingleRank": true})
	if err != nil {
		t.Fatal(err)
	}
	manager := &repo.Manager{Mongo: &database.MongoManager{Cli: client, Db: db}}
	handler := &UnionHandler{unionDao: dao.NewUnionDao(manager), commonDao: dao.NewCommonDao(manager)}
	session := remote.NewSession(nil, &stream.Msg{Uid: "alice"})
	result := handler.GetRank(session, []byte(`{"unionID":1,"startIndex":0,"count":10,"sortData":{"unionInfo.score":-1}}`)).(common.Result)
	if result.Code != 0 || result.Msg.(map[string]any)["totalCount"] != int64(2) {
		t.Fatalf("GetRank handler: %#v", result)
	}
	rows := result.Msg.(map[string]any)["recordArr"].([]unionRankRecord)
	if len(rows) != 2 || rows[0].Score != 25 || rows[0].YesterdayWin != 10 {
		t.Fatalf("GetRank handler fields: %#v", rows)
	}
	_, err = db.Collection("userGameRecord").InsertOne(ctx, bson.M{"unionID": 1, "gameType": 9, "createTime": time.Now().UnixMilli(), "userList": []any{bson.M{"uid": "alice", "score": -7, "nickname": "Alice"}}})
	if err != nil {
		t.Fatal(err)
	}
	result = handler.GetRankSingleDraw(session, []byte(`{"unionID":1,"startIndex":0,"count":20,"matchData":{"gameType":9},"sortData":{"score":1}}`)).(common.Result)
	if result.Code != 0 || result.Msg.(map[string]any)["totalCount"] != int64(1) {
		t.Fatalf("GetRankSingleDraw handler: %#v", result)
	}
	rows = result.Msg.(map[string]any)["recordArr"].([]unionRankRecord)
	if len(rows) != 1 || rows[0].Uid != "alice" || rows[0].Score != -7 {
		t.Fatalf("GetRankSingleDraw handler fields: %#v", rows)
	}
}
