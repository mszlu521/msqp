package entity

import "go.mongodb.org/mongo-driver/bson/primitive"

type SafeBoxRecord struct {
	Id primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	// 操作者ID
	Uid string `bson:"uid" json:"uid"`
	// 联盟ID
	UnionID int64 `bson:"unionID" json:"unionID"`
	// 操作数量
	Count int32 `bson:"count" json:"count"`
	// 赠送时间
	CreateTime int64 `bson:"createTime" json:"createTime"`
}
