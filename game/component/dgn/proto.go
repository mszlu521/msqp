package dgn

import componentproto "game/component/proto"

type MessageReq struct {
	Type int `json:"type"`
	Data struct {
		BeBanker    bool                       `json:"beBanker"`
		Score       int                        `json:"score"`
		Shemen      bool                       `json:"shemen"`
		Trust       bool                       `json:"trust"`
		ChatType    int                        `json:"type"`
		Msg         componentproto.ChatMessage `json:"msg"`
		RecipientID int                        `json:"recipientID"`
	} `json:"data"`
}

const (
	StatusPush       = 401
	ChooseBankPush   = 402
	ChooseBankNotify = 302
	BankerPush       = 403
	CardsPush        = 404
	PourNotify       = 305
	PourPush         = 405
	ShowNotify       = 306
	ShowPush         = 406
	ResultPush       = 407
	EndPush          = 408
	BureauPush       = 409
	ChatNotify       = 310
	ChatPush         = 410
	TrustNotify      = 311
	TrustPush        = 411
	ClearPoolPush    = 412
	ReviewNotify     = 313
	ReviewPush       = 413
)

type BureauReview struct {
	Uid            string `json:"uid"`
	Cards          []int  `json:"cards"`
	CardType       int    `json:"cardType"`
	PourScore      int    `json:"pourScore"`
	WinScore       int    `json:"winScore"`
	Nickname       string `json:"nickname"`
	Avatar         string `json:"avatar"`
	IsBanker       bool   `json:"isBanker"`
	CurBureau      int    `json:"curBureau"`
	BankerTurn     int    `json:"bankerTurn"`
	BankerPourTurn int    `json:"bankerPourTurn"`
}

const (
	StatusNone = iota
	StatusPrepare
	StatusChooseBank
	StatusSend
	StatusPour
	StatusShow
	StatusResult
)

func push(typ int, data any) any {
	return map[string]any{"type": typ, "data": data, "pushRouter": "GameMessagePush"}
}
func status(typ, tick int) any {
	return push(StatusPush, map[string]any{"gameStatus": typ, "tick": tick})
}
