package request

import "go.mongodb.org/mongo-driver/bson"

type CreateUnionReq struct {
	UnionName string `json:"unionName"`
}

type JoinUnionReq struct {
	InviteID string `json:"inviteID"`
}

type MemberListReq struct {
	UnionID    int64  `json:"unionID"`
	MatchData  bson.M `json:"matchData"`
	StartIndex int    `json:"startIndex"`
	Count      int    `json:"count"`
}
type ExitUnionReq struct {
	UnionID int64 `json:"unionID"`
}
type MemberStatisticsInfoReq struct {
	UnionID   int64  `json:"unionID"`
	MatchData bson.M `json:"matchData"`
}
type MemberScoreReq struct {
	UnionID    int64  `json:"unionID"`
	MatchData  bson.M `json:"matchData"`
	StartIndex int    `json:"startIndex"`
	Count      int    `json:"count"`
}

type SafeBoxOperation struct {
	UnionID    int64 `json:"unionID"`
	StartIndex int   `json:"startIndex"`
	Count      int   `json:"count"`
}

type ModifyScoreReq struct {
	UnionID   int64  `json:"unionID"`
	Count     int    `json:"count"`
	MemberUid string `json:"memberUid"`
}

type AddPartnerReq struct {
	UnionID   int64  `json:"unionID"`
	MemberUid string `json:"memberUid"`
}
type GetScoreModifyRecordReq struct {
	UnionID    int64  `json:"unionID"`
	MatchData  bson.M `json:"matchData"`
	StartIndex int    `json:"startIndex"`
	Count      int    `json:"count"`
}
type InviteJoinUnionReq struct {
	UnionID int64  `json:"unionID"`
	Uid     string `json:"uid"`
	Partner bool   `json:"partner"`
}
type OperationInviteJoinUnionReq struct {
	UnionID int64  `json:"unionID"`
	Uid     string `json:"uid"`
	Agree   bool   `json:"agree"`
	Partner bool   `json:"partner"`
}
type UpdateUnionRebateReq struct {
	UnionID    int64   `json:"unionID"`
	MemberUid  string  `json:"memberUid"`
	RebateRate float64 `json:"rebateRate"`
}
type UpdateUnionNoticeReq struct {
	UnionID int64  `json:"unionID"`
	Notice  string `json:"notice"`
}
type GiveScoreReq struct {
	UnionID int64  `json:"unionID"`
	GiveUid string `json:"giveUid"`
	Count   int    `json:"count"`
}
type GetGameRecordReq struct {
	MatchData  bson.M `json:"matchData"`
	StartIndex int    `json:"startIndex"`
	Count      int    `json:"count"`
}
type GetVideoRecordReq struct {
	VideoRecordID string `json:"videoRecordID"`
}
type GetUnionRebateRecordReq struct {
	MatchData  bson.M `json:"matchData"`
	StartIndex int    `json:"startIndex"`
	Count      int    `json:"count"`
}
type UpdateForbidGameStatusReq struct {
	UnionID int64  `json:"unionID"`
	Uid     string `json:"uid"`
	Forbid  bool   `json:"forbid"`
}
type GetGiveScoreRecordReq struct {
	MatchData  bson.M `json:"matchData"`
	StartIndex int    `json:"startIndex"`
	Count      int    `json:"count"`
	UnionID    int64  `json:"unionID"`
}
type GetRankReq struct {
	UnionID    int64  `json:"unionID"`
	MatchData  bson.M `json:"matchData"`
	SortData   bson.M `json:"sortData"`
	StartIndex int    `json:"startIndex"`
	Count      int    `json:"count"`
}
type GetRankSingleDrawReq struct {
	UnionID    int64  `json:"unionID"`
	MatchData  bson.M `json:"matchData"`
	SortData   bson.M `json:"sortData"`
	StartIndex int    `json:"startIndex"`
	Count      int    `json:"count"`
}
