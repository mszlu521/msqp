package entity

import "go.mongodb.org/mongo-driver/bson/primitive"

type User struct {
	Id               primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	Uid              string             `bson:"uid" json:"uid"`                           // 用户唯一ID
	IsBlockedAccount int                `bson:"isBlockedAccount" json:"isBlockedAccount"` // 是否冻结帐号
	Location         string             `bson:"location" json:"location"`                 // 地理位置信息，国家省市街道
	FrontendId       string             `bson:"frontendId" json:"frontendId"`             // 前端服务器ID
	RoomID           string             `bson:"roomID" json:"roomID"`                     // 房间ID
	IsAgent          bool               `bson:"isAgent" json:"isAgent"`                   // 是否是代理  true代表有创建亲友圈的权限
	RealName         string             `bson:"realName" json:"realName"`                 // 实名认证信息
	MobilePhone      string             `bson:"mobilePhone" json:"mobilePhone"`           // 绑定的手机
	InviteMsg        InviteMsg          `bson:"inviteMsg" json:"inviteMsg"`
	EmailArr         string             `bson:"emailArr" json:"emailArr"` // 邮件
	Gold             int64              `bson:"gold" json:"gold"`         // 金币(房卡)
	UnionInfo        []*UnionInfo       `bson:"unionInfo" json:"unionInfo"`
	Sex              int                `bson:"sex" json:"sex"`                     // 性别
	CreateTime       int64              `bson:"createTime" json:"createTime"`       // 创建时间
	LastLoginTime    int64              `bson:"lastLoginTime" json:"lastLoginTime"` // 最后登录时间
	LastLoginIp      string             `bson:"lastLoginIp" json:"lastLoginIp"`     // 最后登录IP
	Address          string             `bson:"address" json:"address"`             // 地理位置经纬度
	AvatarFrame      string             `bson:"avatarFrame" json:"avatarFrame"`     // 头像框
	Nickname         string             `bson:"nickname" json:"nickname"`           // 昵称
	Avatar           string             `bson:"avatar" json:"avatar"`               // 头像
}

type InviteMsg struct {
	Uid       string `bson:"uid" json:"uid"`             // 邀请人ID
	Nickname  string `bson:"nickname" json:"nickname"`   // 邀请人名字
	UnionId   string `bson:"unionId" json:"unionId"`     // 俱乐部ID
	Partner   bool   `bson:"partner" json:"partner"`     // 是否标记为合伙人
	UnionName string `bson:"unionName" json:"unionName"` // 俱乐部名字
}

// UnionInfo 联盟(俱乐部)信息
type UnionInfo struct {
	InviteId     string `bson:"inviteId" json:"inviteId"`         //我的邀请ID
	UnionID      int64  `bson:"unionID" json:"unionID"`           //联盟ID
	Score        int    `json:"score" bson:"score"`               //积分数量
	SafeScore    int    `json:"safeScore" bson:"safeScore"`       //保险柜积分
	Partner      bool   `json:"partner" bson:"partner"`           // 是否是合伙人
	SpreaderID   string `json:"spreaderID" bson:"spreaderID"`     //推广员ID
	ProhibitGame bool   `json:"prohibitGame" bson:"prohibitGame"` // 禁止游戏
	RebateRate   int    `json:"rebateRate" bson:"rebateRate"`     // 返利比例

	TodayDraw                 int   `bson:"todayDraw" json:"todayDraw"`                                 // 今日总局数
	YesterdayDraw             int   `bson:"yesterdayDraw" json:"yesterdayDraw"`                         // 昨日总局数
	TotalDraw                 int   `bson:"totalDraw" json:"totalDraw"`                                 // 总局数
	WeekDraw                  int   `bson:"weekDraw" json:"weekDraw"`                                   // 每周局数
	MemberTodayDraw           int   `bson:"memberTodayDraw" json:"memberTodayDraw"`                     // 成员今日总局数
	MemberYesterdayDraw       int   `bson:"memberYesterdayDraw" json:"memberYesterdayDraw"`             // 成员昨日总局数
	TodayBigWinDraw           int   `bson:"todayBigWinDraw" json:"todayBigWinDraw"`                     // 今日大赢家局数
	YesterdayBigWinDraw       int   `bson:"yesterdayBigWinDraw" json:"yesterdayBigWinDraw"`             // 昨日大赢家局数
	MemberTodayBigWinDraw     int   `bson:"memberTodayBigWinDraw" json:"memberTodayBigWinDraw"`         // 成员今日大赢家局数
	MemberYesterdayBigWinDraw int   `bson:"memberYesterdayBigWinDraw" json:"memberYesterdayBigWinDraw"` // 成员昨日大赢家局数
	TodayProvideRebate        int   `bson:"todayProvideRebate" json:"todayProvideRebate"`               // 今日贡献返利
	YesterdayProvideRebate    int   `bson:"yesterdayProvideRebate" json:"yesterdayProvideRebate"`       // 昨日贡献返利
	TodayRebate               int   `bson:"todayRebate" json:"todayRebate"`                             // 今日返利
	YesterdayRebate           int   `bson:"yesterdayRebate" json:"yesterdayRebate"`                     // 昨日返利
	TotalRebate               int   `bson:"totalRebate" json:"totalRebate"`                             // 总返利
	TodayWin                  int   `bson:"todayWin" json:"todayWin"`                                   // 今日赢分
	YesterdayWin              int   `bson:"yesterdayWin" json:"yesterdayWin"`                           // 昨日赢分
	JoinTime                  int64 `bson:"joinTime" json:"joinTime"`                                   // 加入时间
}

const (
	Man = iota
	Woman
)
