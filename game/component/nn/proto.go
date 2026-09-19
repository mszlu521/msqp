package nn

import componentproto "game/component/proto"

type MessageReq struct {
	Type int         `json:"type"`
	Data MessageData `json:"data"`
}

type MessageData struct {
	RobScale    int                        `json:"robScale"`
	Score       int                        `json:"score"`
	Cuopai      bool                       `json:"cuopai"`
	Trust       bool                       `json:"trust"`
	ChatType    int                        `json:"type"`
	Msg         componentproto.ChatMessage `json:"msg"`
	RecipientID int                        `json:"recipientID"`
}

const (
	GameRobBankNotify   = 301
	GameRobBankPush     = 401
	GameBankerPush      = 402
	GamePourScoreNotify = 303
	GamePourScorePush   = 403
	GameShowCardsNotify = 304
	GameShowCardsPush   = 404
	GameStatusPush      = 405
	GameResultPush      = 406
	GameSendCardsPush   = 407
	GameBureauPush      = 408
	GameEndPush         = 409
	GameChatNotify      = 310
	GameChatPush        = 410
	GameTrustNotify     = 311
	GameTrustPush       = 411
	GameReviewNotify    = 312
	GameReviewPush      = 412
)

const (
	StatusNone = iota
	StatusRobBank
	StatusPourScore
	StatusSendCards
	StatusShowCards
	StatusResult
)

type GameData struct {
	GameStatus     int     `json:"gameStatus"`
	GameStarted    bool    `json:"gameStarted"`
	Tick           int     `json:"tick"`
	BankerChairID  int     `json:"bankerChairID"`
	PourScores     []int   `json:"pourScores"`
	RobBankScales  []int   `json:"robBankScales"`
	HandCards      [][]int `json:"handCards"`
	CurBureau      int     `json:"curBureau"`
	MaxBureau      int     `json:"maxBureau"`
	ChairCount     int     `json:"chairCount"`
	ShowCards      []int   `json:"showCards"`
	UserTrustArray []bool  `json:"userTrustArray"`
	Result         any     `json:"result"`
	CurScores      []int   `json:"curScores"`
	TuiArr         []int   `json:"tuiArr"`
}

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

func push(typ int, data any) any {
	return map[string]any{"type": typ, "data": data, "pushRouter": "GameMessagePush"}
}

func statusPush(status, tick int) any {
	return push(GameStatusPush, map[string]any{"gameStatus": status, "tick": tick})
}
func robPush(chair, scale int) any {
	return push(GameRobBankPush, map[string]any{"chairID": chair, "robScale": scale})
}
func bankerPush(chair, scale int, rob []int) any {
	return push(GameBankerPush, map[string]any{"bankerChairID": chair, "robScale": scale, "robChairIDs": rob})
}
func pourPush(chair, score int) any {
	return push(GamePourScorePush, map[string]any{"chairID": chair, "score": score})
}
func showPush(chair int, cards []int, cuopai bool) any {
	return push(GameShowCardsPush, map[string]any{"chairID": chair, "cards": cards, "cuopai": cuopai})
}
func sendCardsPush(cards [][]int) any {
	return push(GameSendCardsPush, map[string]any{"handCards": cards})
}
func resultPush(result any) any { return push(GameResultPush, map[string]any{"result": result}) }
func chatPush(chair, typ int, msg any, recipient int) any {
	return push(GameChatPush, map[string]any{"chairID": chair, "type": typ, "msg": msg, "recipientID": recipient})
}
func trustPush(chair int, trust bool) any {
	return push(GameTrustPush, map[string]any{"chairID": chair, "trust": trust})
}
