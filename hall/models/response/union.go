package response

type UnionRecord struct {
	UnionID       int64  `json:"unionID"`
	UnionName     string `json:"unionName"`
	OwnerUid      string `json:"ownerUid"`
	OwnerAvatar   string `json:"ownerAvatar"`
	OwnerNickname string `json:"ownerNickname"`
	MemberCount   int32  `json:"memberCount"`
	OnlineCount   int32  `json:"onlineCount"`
}
type UnionListResp struct {
	RecordArr []UnionRecord `json:"recordArr"`
}

type CreateUnionResp struct {
	Code           int            `json:"code"`
	Msg            any            `json:"msg"`
	UpdateUserData map[string]any `json:"updateUserData"`
}
type JoinUnionResp struct {
	Code           int            `json:"code"`
	Msg            any            `json:"msg"`
	UpdateUserData map[string]any `json:"updateUserData"`
}
type UnionMember struct {
	Uid                       string `json:"uid"`
	Nickname                  string `json:"nickname"`
	Avatar                    string `json:"avatar"`
	FrontendId                string `json:"frontendId"`
	RoomId                    string `json:"roomId"`
	SpreaderID                string `json:"spreaderID" bson:"spreaderID"`                               //推广员ID
	ProhibitGame              bool   `json:"prohibitGame" bson:"prohibitGame"`                           // 禁止游戏
	RebateRate                int    `json:"rebateRate" bson:"rebateRate"`                               // 返利比例
	YesterdayDraw             int    `bson:"yesterdayDraw" json:"yesterdayDraw"`                         // 昨日总局数
	YesterdayBigWinDraw       int    `bson:"yesterdayBigWinDraw" json:"yesterdayBigWinDraw"`             // 昨日大赢家局数
	YesterdayRebate           int    `bson:"yesterdayRebate" json:"yesterdayRebate"`                     // 昨日返利
	TodayRebate               int    `bson:"todayRebate" json:"todayRebate"`                             // 今日返利
	TotalDraw                 int    `bson:"totalDraw" json:"totalDraw"`                                 // 总局数
	MemberYesterdayDraw       int    `bson:"memberYesterdayDraw" json:"memberYesterdayDraw"`             // 成员昨日总局数
	MemberYesterdayBigWinDraw int    `bson:"memberYesterdayBigWinDraw" json:"memberYesterdayBigWinDraw"` // 成员昨日大赢家局数
	YesterdayProvideRebate    int    `bson:"yesterdayProvideRebate" json:"yesterdayProvideRebate"`       // 昨日贡献返利
	Score                     int    `json:"score"`
	SafeScore                 int    `json:"safeScore"`
}
type MemberListResp struct {
	RecordArr  []*UnionMember `json:"recordArr"`
	TotalCount int64          `json:"totalCount"`
}

type ExitUnionResp struct {
	Code           int            `json:"code"`
	UpdateUserData map[string]any `json:"updateUserData"`
}
type MemberStatisticsInfoResp struct {
	YesterdayTotalDraw          int64 `json:"yesterdayTotalDraw"`
	YesterdayTotalProvideRebate int64 `json:"yesterdayTotalProvideRebate"`
	TotalCount                  int64 `json:"totalCount"`
}
type MemberScoreRecord struct {
	Uid       string `json:"uid"`
	Nickname  string `json:"nickname"`
	Avatar    string `json:"avatar"`
	Score     int32  `json:"score"`
	SafeScore int32  `json:"safeScore"`
}
type MemberScoreListResp struct {
	RecordArr  []*MemberScoreRecord `json:"recordArr"`
	TotalCount int64                `json:"totalCount"`
	TotalScore int64                `json:"totalScore"`
}
type SafeBoxOperationRecord struct {
	Code           int            `json:"code"`
	UpdateUserData map[string]any `json:"updateUserData"`
}
