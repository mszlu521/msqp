package entity

import "go.mongodb.org/mongo-driver/bson/primitive"

// ScoreModifyRecord 改分记录
type ScoreModifyRecord struct {
	Id  primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	Uid string             `bson:"uid" json:"uid"`
	// 改分昵称
	Nickname string `bson:"nickname" json:"nickname"`
	// 改分头像
	Avatar string `bson:"avatar" json:"avatar"`
	// 被改分者ID
	GainUid string `bson:"gainUid" json:"gainUid"`
	// 被改分者昵称
	GainNickname string `bson:"gainNickname" json:"gainNickname"`
	// 联盟ID
	UnionID int64 `bson:"unionID" json:"unionID"`
	// 数量
	Count      int32 `bson:"count" json:"count"`
	CreateTime int64 `bson:"createTime" json:"createTime"`
}

// ScoreGiveRecord 赠送积分记录
type ScoreGiveRecord struct {
	Id           primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	Uid          string             `bson:"uid" json:"uid"`
	Nickname     string             `bson:"nickname" json:"nickname"`
	GainUid      string             `bson:"gainUid" json:"gainUid"`
	GainNickname string             `bson:"gainNickname" json:"gainNickname"`
	UnionID      string             `bson:"unionID" json:"unionID"`
	Count        int32              `bson:"count" json:"count"`
	CreateTime   int64              `bson:"createTime" json:"createTime"`
}
