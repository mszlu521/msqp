package request

import "game/component/proto"

type CreateRoomReq struct {
	UnionID    int64          `json:"unionID"` // 1 普通用户创建
	GameRuleID string         `json:"gameRuleID"`
	GameRule   proto.GameRule `json:"gameRule"`
}

type JoinRoomReq struct {
	RoomID string `json:"roomID"`
}

type GetUnionReq struct {
	UnionID int64 `json:"unionID"`
}
type QuickJoinReq struct {
	UnionID    int64  `json:"unionID"`
	GameRuleID string `json:"gameRuleID"`
}

type HoneBaoReq struct {
	UnionID int64 `json:"unionID"`
}
