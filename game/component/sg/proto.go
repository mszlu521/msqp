package sg

import (
	"encoding/json"
	componentproto "game/component/proto"
)

type MessageReq struct {
	Type int `json:"type"`
	Data struct {
		RobScale    int                        `json:"robScale"`
		Score       int                        `json:"score"`
		Cuopai      bool                       `json:"cuopai"`
		Trust       bool                       `json:"trust"`
		ChatType    int                        `json:"type"`
		Msg         componentproto.ChatMessage `json:"msg"`
		RecipientID int                        `json:"recipientID"`
	} `json:"data"`
}

const (
	RobNotify    = 301
	RobPush      = 401
	BankerPush   = 402
	PourNotify   = 303
	PourPush     = 403
	ShowNotify   = 304
	ShowPush     = 404
	StatusPush   = 405
	ResultPush   = 406
	SendPush     = 407
	BureauPush   = 408
	EndPush      = 409
	ChatNotify   = 310
	ChatPush     = 410
	TrustNotify  = 311
	TrustPush    = 411
	ReviewNotify = 312
	ReviewPush   = 412
)

type BureauReview struct {
	Uid       string `json:"uid"`
	Cards     []int  `json:"cards"`
	CardType  int    `json:"cardType"`
	Rob       int    `json:"rob"`
	PourScore int    `json:"pourScore"`
	WinScore  int    `json:"winScore"`
	Nickname  string `json:"nickname"`
	Avatar    string `json:"avatar"`
	IsBanker  bool   `json:"isBanker"`
}

const (
	StatusNone = iota
	StatusRob
	StatusPour
	StatusSend
	StatusShow
	StatusResult
)

func push(typ int, data any) any {
	return map[string]any{"type": typ, "data": data, "pushRouter": "GameMessagePush"}
}
func status(typ, tick int) any {
	return push(StatusPush, map[string]any{"gameStatus": typ, "tick": tick})
}
func encode(v any) []byte { b, _ := json.Marshal(v); return b }
