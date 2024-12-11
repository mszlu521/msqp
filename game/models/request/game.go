package request

import "game/component/proto"

type RoomMessageReq struct {
	Type proto.RoomMessageType `json:"type"`
	Data RoomMessageData       `json:"data"`
}

type RoomMessageData struct {
	IsReady     bool   `json:"isReady"`
	IsExit      bool   `json:"isExit"`
	ToChairID   int    `json:"toChairID"`
	Msg         string `json:"msg"`
	FromChairID int    `json:"fromChairID"`
}
