package request

import (
	"game/component/proto"
)

type AddRoomRuleReq struct {
	RuleName string         `json:"ruleName"`
	UnionID  int64          `json:"unionID"`
	GameType int            `json:"gameType"`
	RoomRule proto.GameRule `json:"roomRule"`
}
type UpdateRoomRuleReq struct {
	ID       string         `json:"_id"`
	RuleName string         `json:"ruleName"`
	UnionID  int64          `json:"unionID"`
	GameType int            `json:"gameType"`
	RoomRule proto.GameRule `json:"roomRule"`
}
type RemoveRoomRuleReq struct {
	RoomRuleId string `json:"roomRuleId"`
	UnionID    int64  `json:"unionID"`
}
type UpdateOpeningStatusReq struct {
	UnionID int64 `json:"unionID"`
	IsOpen  bool  `json:"isOpen"`
}
type UpdateUnionNoticeReq struct {
	UnionID int64  `json:"unionID"`
	Notice  string `json:"notice"`
}
type UpdateUnionNameReq struct {
	UnionID   int64  `json:"unionID"`
	UnionName string `json:"unionName"`
}
type UpdatePartnerNoticeSwitchReq struct {
	UnionID int64 `json:"unionID"`
	IsOpen  bool  `json:"isOpen"`
}
type DismissRoomReq struct {
	UnionID int64  `json:"unionID"`
	RoomID  string `json:"roomID"`
}
type HongBaoSettingReq struct {
	UnionID    int64 `json:"unionID"`
	Status     bool  `json:"status"`
	StartTime  int64 `json:"startTime"`
	EndTime    int64 `json:"endTime"`
	Count      int   `json:"count"`
	TotalScore int64 `json:"totalScore"`
}
type UpdateLotteryStatusReq struct {
	UnionID int64 `json:"unionID"`
	IsOpen  bool  `json:"isOpen"`
}
