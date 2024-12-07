package entity

import "go.mongodb.org/mongo-driver/bson/primitive"

type Union struct {
	Id primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	//盟主Uid
	OwnerUid string `bson:"ownerUid" json:"ownerUid"`
	// 盟主昵称
	OwnerNickname string `bson:"ownerNickname" json:"ownerNickname"`
	// 盟主头像
	OwnerAvatar string `bson:"ownerAvatar" json:"ownerAvatar"`
	// 联盟名字
	UnionName string `bson:"unionName" json:"unionName"`
	// 当前成员数量
	CurMember int32 `bson:"curMember" json:"curMember"`
	// 在线成员数量
	OnlineMember int32 `bson:"onlineMember" json:"onlineMember"`
	//房间规则
	RoomRuleList []RoomRule `bson:"roomRuleList" json:"roomRuleList"`
	// 是否允许创建房间
	AllowCreateRoom bool `bson:"allowCreateRoom" json:"allowCreateRoom"`
	// 最大房间数量
	MaxRoomCount int32 `bson:"maxRoomCount" json:"maxRoomCount"`
	// 公告
	Notice string `bson:"notice" json:"notice"`
	// 公告开关
	NoticeSwitch bool `bson:"noticeSwitch" json:"noticeSwitch"`
	// 允许合并
	AllowMerge bool `bson:"allowMerge" json:"allowMerge"`
	// 是否正在营业
	Opening bool `bson:"opening" json:"opening"`
	// 加入请求列表
	JoinRequestList []JoinRequest `bson:"joinRequestList" json:"joinRequestList"`
	// 是否允许显示排行榜
	ShowRank bool `bson:"showRank" json:"showRank"`
	// 是否允许显示单局排行榜
	ShowSingleRank bool `bson:"showSingleRank" json:"showSingleRank"`
	// 是否允许显示联盟活动
	ShowUnionActive bool `bson:"showUnionActive" json:"showUnionActive"`
	// 是否禁止邀请
	ForbidInvite bool `bson:"forbidInvite" json:"forbidInvite"`
	// 是否禁止赠送分数
	ForbidGive  bool        `bson:"forbidGive" json:"forbidGive"`
	HongBaoInfo HongBaoInfo `bson:"hongBaoInfo" json:"hongBaoInfo"`
	// 红包金额列表
	HongBaoScoreList  []int32           `bson:"hongBaoScoreList" json:"hongBaoScoreList"`
	ResultLotteryInfo ResultLotteryInfo `bson:"resultLotteryInfo" json:"resultLotteryInfo"`
	// 红包领取用户列表
	HongBaoUidList []string `bson:"hongBaoUidList" json:"hongBaoUidList"`
	// 创建时间
	CreateTime int64 `bson:"createTime" json:"createTime"`
}
type ResultLotteryInfo struct {
	// 活动开启状态
	Status bool `json:"status"`
	// 金额
	CountArr []int32 `json:"countArr"`
	// 金额对应概率
	RateArr []int32 `json:"rateArr"`
}
type HongBaoInfo struct {
	// 活动开启状态
	Status bool `json:"status"`
	// 开始时间
	StartTime int64 `json:"startTime"`
	// 结束时间
	EndTime int64 `json:"endTime"`
	// 个数
	Count int32 `json:"count"`
	// 红包总分
	TotalScore int32 `json:"totalScore"`
}

type JoinRequest struct {
	// 请求者id
	Uid string `json:"uid"`
	// 昵称
	Nickname string `json:"nickname"`
	// 头像
	Avatar string `json:"avatar"`
	// 请求时间
	CreateTime int64 `json:"createTime"`
}

type RoomRule struct {
	// 游戏类型
	GameType int `json:"gameType"`
	// 房间名字
	RuleName string `json:"ruleName"`
	// 游戏规则
	GameRule string `json:"gameRule"`
}
