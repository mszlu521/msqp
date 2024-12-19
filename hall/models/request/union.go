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
