package entity

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
	"time"
)

//账号  用户密码登录 生成一个账号 某个账号下 还会有多个角色

//简单一点 直接使用账号登录 账号->对应一个用户的角色  1对1

type Account struct {
	Id           primitive.ObjectID `bson:"_id,omitempty"`
	Uid          string             `bson:"uid"`
	Account      string             `bson:"account" `
	Password     string             `bson:"password"`
	PhoneAccount string             `bson:"phoneAccount"`
	WxAccount    string             `bson:"wxAccount"`
	CreateTime   time.Time          `bson:"createTime"`
}
