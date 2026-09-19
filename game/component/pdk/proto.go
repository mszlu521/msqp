package pdk

import (
	"core/models/enums"
	componentproto "game/component/proto"
)

type MessageReq struct {
	Type int         `json:"type"`
	Data MessageData `json:"data"`
}

type MessageData struct {
	OutCardArr  []int                      `json:"outCardArr"`
	Trust       bool                       `json:"trust"`
	ChatType    int                        `json:"type"`
	Msg         componentproto.ChatMessage `json:"msg"`
	RecipientID int                        `json:"recipientID"`
}

const (
	GameUserOutCardNotify  = 302
	GameUserPassNotify     = 303
	GameStartPush          = 405
	GameResultPush         = 406
	GameUserBombWinPush    = 407
	GameChatNotify         = 310
	GameChatPush           = 410
	GameDismissPush        = 411
	GameTrustNotify        = 312
	GameTrustPush          = 412
	GamePreTurnCardsNotify = 313
	GamePreTurnCardsPush   = 413
)

type GameStatus int

const (
	GameStatusNone GameStatus = iota
	GameStatusOutCard
	GameStatusEnd
)

type GameData struct {
	GameStarted         bool       `json:"gameStarted"`
	CurBureau           int        `json:"curBureau"`
	GameStatus          GameStatus `json:"gameStatus"`
	SelfCardArr         []int      `json:"selfCardArr"`
	AllUserCardCountArr [][]int    `json:"allUserCardCountArr"`
	TurnCardDataArr     []int      `json:"turnCardDataArr"`
	CurChairID          int        `json:"curChairID"`
	TurnWinerChairID    int        `json:"turnWinerChairID"`
	FirstChairID        int        `json:"firstChairID"`
	EnablePass          bool       `json:"enablePass"`
	IsFirstTurnArray    []bool     `json:"isFirstTurnArray"`
	UserBombTimes       []int      `json:"userBombTimes"`
	UserTrustArray      []bool     `json:"userTrustArray"`
}

type dismissUser struct {
	Uid            string `json:"uid"`
	Nickname       string `json:"nickname"`
	Avatar         string `json:"avatar"`
	WinScore       int    `json:"winScore"`
	SingleMaxScore int    `json:"singleMaxScore"`
	BoomCount      int    `json:"boomCount"`
	WinCount       int    `json:"winCount"`
	LoseCount      int    `json:"loseCount"`
}

type dismissCreator struct {
	Uid      string `json:"uid"`
	Nickname string `json:"nickname"`
	Avatar   string `json:"avatar"`
}

func gameDismissPush(users []*dismissUser, creator *dismissCreator, reason enums.RoomDismissReason, hongBaoList any) any {
	return map[string]any{"type": GameDismissPush, "data": map[string]any{
		"userArray": users, "creator": creator, "reason": reason, "hongBaoList": hongBaoList,
	}, "pushRouter": "GameMessagePush"}
}

func gameStartPush(chairID, bureau int, cards []int, countArrays ...[][]int) any {
	allUserCardCountArr := [][]int(nil)
	if len(countArrays) > 0 {
		allUserCardCountArr = countArrays[0]
	}
	return map[string]any{"type": GameStartPush, "data": map[string]any{
		"curChairID": chairID, "selfCardArr": cards, "curBureau": bureau,
		"allUserCardCountArr": allUserCardCountArr,
	}, "pushRouter": "GameMessagePush"}
}

func gameOutCardPush(chairID, curChairID int, cards []int, left []int, enablePass bool, hand []int) any {
	return map[string]any{"type": 402, "data": map[string]any{
		"chairID": chairID, "curChairID": curChairID, "outCardArr": cards,
		"leftCardCount": len(left), "enablePass": enablePass, "handCardArr": hand,
	}, "pushRouter": "GameMessagePush"}
}

func gamePassPush(chairID, curChairID int, enablePass, newTurn bool, hand []int) any {
	return map[string]any{"type": 403, "data": map[string]any{
		"chairID": chairID, "curChairID": curChairID, "enablePass": enablePass,
		"isNewTurn": newTurn, "handCardArr": hand,
	}, "pushRouter": "GameMessagePush"}
}

func gameResultPush(allCards [][]int, winner int, users []string, hands [][][]int, winArr, bombArr []int, hasChuntian bool) any {
	nicknameArr := make([]any, len(users))
	idArr := make([]any, len(users))
	for i, user := range users {
		if user != "" {
			nicknameArr[i], idArr[i] = user, user
		}
	}
	return map[string]any{"type": GameResultPush, "data": map[string]any{
		"allCardArr": allCards, "winChairID": winner, "winArr": winArr,
		"nicknameArr": nicknameArr, "allHandCards": hands, "headArr": make([]any, len(users)),
		"idArr": idArr, "bombArr": bombArr, "hasChuntian": hasChuntian,
	}, "pushRouter": "GameMessagePush"}
}
