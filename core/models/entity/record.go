package entity

import (
	"core/models/enums"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type GameProfitRecord struct {
	Id  primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	Day string             `bson:"day" json:"day"`
	// 新增用户
	Register int `bson:"register" json:"register"`
	// 活跃用户
	Active    int       `bson:"active" json:"active"`
	DrawCount DrawCount `bson:"drawCount" json:"drawCount"`
	// 花费总金币数
	ExpendGold int `bson:"expendGold" json:"expendGold"`
}

type DrawCount struct {
	// 游戏类型
	GameType int `bson:"gameType" json:"gameType"`
	// 游戏局数
	Count int `bson:"count" json:"count"`
}

type RechargeOrderRecords struct {
	Id         primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	OrderId    string             `bson:"orderId" json:"orderId"`
	Uid        string             `bson:"uid" json:"uid"`
	ItemId     string             `bson:"itemId" json:"itemId"`
	CreateTime int64              `bson:"createTime" json:"createTime"`
}

type RechargeRecord struct {
	Id                    primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	Uid                   string             `bson:"uid" json:"uid"`
	Nickname              string             `bson:"nickname" json:"nickname"`
	SpreaderID            string             `bson:"spreaderID" json:"spreaderID"`
	RechargeMoney         int64              `bson:"rechargeMoney" json:"rechargeMoney"`
	GoldCount             int64              `bson:"goldCount" json:"goldCount"`
	UserOrderId           string             `bson:"userOrderId" json:"userOrderId"`
	PlatformReturnOrderID string             `bson:"platformReturnOrderID" json:"platformReturnOrderID"`
	Platform              string             `bson:"platform" json:"platform"`
	CreateTime            int64              `bson:"createTime" json:"createTime"`
}

type UserGameRecord struct {
	Id            primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	RoomID        string             `bson:"roomID" json:"roomID"`
	UnionID       string             `bson:"unionID" json:"unionID"`
	CreatorUid    string             `bson:"creatorUid" json:"creatorUid"`
	GameType      int                `bson:"gameType" json:"gameType"`
	UserList      []GameUser         `bson:"userList" json:"userList"`
	Detail        string             `bson:"detail" json:"detail"`
	VideoRecordID string             `bson:"videoRecordID" json:"videoRecordID"`
	CreateTime    int64              `bson:"createTime" json:"createTime"`
}

type GameUser struct {
	Uid        string `bson:"uid" json:"uid"`
	Score      int64  `bson:"score" json:"score"`
	Nickname   string `bson:"nickname" json:"nickname"`
	Avatar     string `bson:"avatar" json:"avatar"`
	SpreaderID string `bson:"spreaderID" json:"spreaderID"`
}

type GameVideoRecord struct {
	Id         primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	RoomID     string             `bson:"roomID" json:"roomID"`
	GmeType    int                `bson:"gmeType" json:"gmeType"`
	Detail     string             `bson:"detail" json:"detail"`
	CreateTime int64              `bson:"createTime" json:"createTime"`
}

// UserRebateRecord 玩家抽水记录
type UserRebateRecord struct {
	Id         primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	Uid        string             `bson:"uid" json:"uid"`
	RoomID     string             `bson:"roomID" json:"roomID"`
	GameType   int                `bson:"gameType" json:"gameType"`
	UnionID    int64              `bson:"unionID" json:"unionID"`
	PlayerUid  string             `bson:"playerUid" json:"playerUid"`
	TotalCount int                `bson:"totalCount" json:"totalCount"`
	GainCount  int                `bson:"gainCount" json:"gainCount"`
	Start      bool               `bson:"start" json:"start"`
	CreateTime int64              `bson:"createTime" json:"createTime"`
}

// UserScoreChangeRecord 玩家分数变化记录
type UserScoreChangeRecord struct {
	Id       primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	Uid      string             `bson:"uid" json:"uid"`
	Nickname string             `bson:"nickname" json:"nickname"`
	UnionID  int64              `bson:"unionID" json:"unionID"`
	// 分数变化
	ChangeCount int64 `bson:"changeCount" json:"changeCount"`
	// 剩余分数
	LeftCount int64 `bson:"leftCount" json:"leftCount"`
	// 剩余保险柜分数
	LeftSafeBoxCount int64 `bson:"leftSafeBoxCount" json:"leftSafeBoxCount"`
	// 改变类型
	ChangeType enums.ScoreChangeType `bson:"changeType" json:"changeType"`
	// 描述
	Describe   string `bson:"describe" json:"describe"`
	CreateTime int64  `bson:"createTime" json:"createTime"`
}
