package dao

import (
	"common/database"
	"context"
	"core/repo"
	"testing"
	"time"
)

func TestRedisDaoSmsAndDeleteHandleMissingClient(t *testing.T) {
	dao := NewRedisDao(&repo.Manager{Redis: &database.RedisManager{}})
	if dao.CheckSmsCode("13800000000", "123456") {
		t.Fatal("missing Redis client accepted an SMS code")
	}
	if err := dao.Register("13800000000", "123456", time.Minute); err == nil {
		t.Fatal("Register reported success without a Redis client")
	}
	if err := dao.Delete(context.Background(), "room"); err == nil {
		t.Fatal("Delete reported success without a Redis client")
	}
	if err := dao.Store(context.Background(), "room", "server"); err == nil {
		t.Fatal("Store reported success without a Redis client")
	}
	if _, err := dao.Get(context.Background(), "room"); err == nil {
		t.Fatal("Get reported success without a Redis client")
	}
	if _, err := dao.NextAccountId(); err == nil {
		t.Fatal("NextAccountId reported success without a Redis client")
	}
}
