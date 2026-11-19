package handler

import (
	"common"
	"common/config"
	"common/database"
	"common/jwts"
	"common/logs"
	"context"
	"core/models/entity"
	"core/repo"
	"encoding/json"
	"fmt"
	"framework/game"
	"framework/net"
	"os"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func TestEntryPersistsConnectorForBalancePushMongo(t *testing.T) {
	uri := os.Getenv("MSQP_RANK_TEST_MONGO_URI")
	if uri == "" {
		t.Skip("set MSQP_RANK_TEST_MONGO_URI to run entry regression")
	}
	oldConfig, oldGame := config.Conf, game.Conf
	config.Conf = &config.Config{Jwt: config.JwtConf{Secret: "entry-regression-only"}}
	game.Conf = &game.Config{GameConfig: map[string]game.GameConfigValue{}}
	logs.InitLog("entry-regression")
	t.Cleanup(func() { config.Conf, game.Conf = oldConfig, oldGame })
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Disconnect(context.Background())
	db := client.Database(fmt.Sprintf("msqp_entry_regression_%d", time.Now().UnixNano()))
	defer db.Drop(context.Background())
	if _, err := db.Collection("user").InsertOne(ctx, &entity.User{Uid: "balance-user", Gold: 100}); err != nil {
		t.Fatal(err)
	}
	handler := NewEntryHandler(&repo.Manager{Mongo: &database.MongoManager{Cli: client, Db: db}})
	token, err := jwts.GenToken(&jwts.CustomClaims{Uid: "balance-user"}, config.Conf.Jwt.Secret)
	if err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(map[string]any{"token": token, "userInfo": map[string]any{}})
	if err != nil {
		t.Fatal(err)
	}
	for _, serverID := range []string{"connector-first", "connector-reconnected"} {
		session := net.NewSession("balance-connection", &net.Manager{ServerId: serverID})
		response, err := handler.Entry(session, body)
		if err != nil {
			t.Fatal(err)
		}
		result := response.(common.Result)
		if result.Code != 0 {
			t.Fatalf("entry failed: %#v", result)
		}
		user := result.Msg.(map[string]any)["userInfo"].(*entity.User)
		if user.FrontendId != serverID || session.Uid != user.Uid {
			t.Fatalf("entry routing: %#v", user)
		}
		var stored entity.User
		if err := db.Collection("user").FindOne(ctx, bson.M{"uid": user.Uid}).Decode(&stored); err != nil {
			t.Fatal(err)
		}
		if stored.FrontendId != serverID || stored.Gold != 100 {
			t.Fatalf("stored routing and balance: %#v", stored)
		}
	}
}
