package sz

type MessageReq struct {
	Type int         `json:"type"`
	Data MessageData `json:"data"`
}
type MessageData struct {
	Cuopai  bool `json:"cuopai"`
	Score   int  `json:"score"`
	Type    int  `json:"type"` //1 跟注 2 加注
	ChairID int  `json:"chairID"`
}
type GameStatus int

type GameData struct {
	BankerChairID   int                      `json:"bankerChairID"`
	ChairCount      int                      `json:"chairCount"`
	CurBureau       int                      `json:"curBureau"`
	CurScore        int                      `json:"curScore"`
	CurScores       []int                    `json:"curScores"`
	GameStarter     bool                     `json:"gameStarter"`
	GameStatus      GameStatus               `json:"gameStatus"`
	HandCards       [][]int                  `json:"handCards"`
	LookCards       []int                    `json:"lookCards"`
	Loser           []int                    `json:"loser"`
	Winner          []int                    `json:"winner"`
	MaxBureau       int                      `json:"maxBureau"`
	PourScores      [][]int                  `json:"pourScores"`
	GameType        GameType                 `json:"gameType"`
	BaseScore       int                      `json:"baseScore"`
	Result          any                      `json:"result"`
	Round           int                      `json:"round"`
	Tick            int                      `json:"tick"` //倒计时
	UserTrustArray  []bool                   `json:"userTrustArray"`
	UserStatusArray []UserStatus             `json:"userStatusArray"`
	UserWinRecord   map[string]UserWinRecord `json:"userWinRecord"`
	ReviewRecord    []BureauReview           `json:"reviewRecord"`
	TrustTmArray    []int                    `json:"trustTmArray"`
	CurChairID      int                      `json:"curChairID"`
}

// None 初始状态
const None int = 0
const (
	GameStatusNone GameStatus = iota
	SendCards                 //发牌中
	PourScore                 //下分中
	Result                    //显示结果
)

type GameStatusTm int

const (
	TmSendCards GameStatusTm = 1
	TmPourScore              = 30 //下分中
	TmResult                 = 5  //显示结果
)

type GameType int

const (
	Men1 GameType = 1 //闷1轮
	Men2          = 2 //闷2轮
	Men3          = 3 //闷3轮
)

type RoundType int

const (
	Round10 RoundType = 1 //10轮
	Round20           = 2 //15轮
	Round30           = 3 //20轮
)

type CardsType int

const (
	DanZhang CardsType = 1 //单牌
	DuiZi              = 2 //对子
	ShunZi             = 3 //顺子
	JinHua             = 4 //金花
	ShunJin            = 5 //顺金
	BaoZi              = 6 //豹子
)

type UserStatus int

const (
	Abandon        UserStatus = 1  // 放弃
	TimeoutAbandon            = 2  //超时放弃
	Look                      = 4  //看牌
	Lose                      = 8  //比牌失败
	Win                       = 16 //胜利
	He                        = 32 //和
)

type UserWinRecord struct {
	Uid      string `json:"uid"`
	Nickname string `json:"nickname"`
	Avatar   string `json:"avatar"`
	Score    int    `json:"score"`
}

type BureauReview struct {
	Uid       string `json:"uid"`
	Cards     []int  `json:"cards"`
	PourScore int    `json:"pourScore"`
	WinScore  int    `json:"winScore"`
	NickName  string `json:"nickName"`
	Avatar    string `json:"avatar"`
	IsBanker  bool   `json:"isBanker"`
	IsAbandon bool   `json:"isAbandon"`
}

const (
	GameStatusPush      = 401 //游戏状态推送
	GameSendCardsPush   = 402 //发牌推送
	GameLookNotify      = 303 //看牌请求
	GameLookPush        = 403
	GamePourScoreNotify = 304 //下分请求
	GamePourScorePush   = 404
	GameCompareNotify   = 305 //比牌请求
	GameComparePush     = 405
	GameTurnPush        = 406 //操作推送
	GameResultPush      = 407 //结果推送
	GameEndPush         = 409 //结束推送
	GameChatNotify      = 310 //游戏聊天
	GameChatPush        = 410
	GameBureauPush      = 411 //局数推送
	GameAbandonNotify   = 312 //弃牌请求
	GameAbandonPush     = 412
	GameRoundPush       = 413 //轮数推送
	GameBankerPush      = 414 //庄家推送
	GameTrustNotify     = 315 //托管
	GameTrustPush       = 415 //托管推送
	GameReviewNotify    = 316 //牌面回顾
	GameReviewPush      = 416
)

// UpdateUserInfoPushGold  {"gold": 9958, "pushRouter": 'UpdateUserInfoPush'}
func UpdateUserInfoPushGold(gold int64) any {
	return map[string]any{
		"gold":       gold,
		"pushRouter": "UpdateUserInfoPush",
	}
}

//{"type":414,"data":{"bankerChairID":0},"pushRouter":"GameMessagePush"}

func GameBankerPushData(bankerChairID int) any {
	return map[string]any{
		"type": GameBankerPush,
		"data": map[string]any{
			"bankerChairID": bankerChairID,
		},
		"pushRouter": "GameMessagePush",
	}
}

//{"type":411,"data":{"curBureau":6},"pushRouter":"GameMessagePush"}

func GameBureauPushData(curBureau int) any {
	return map[string]any{
		"type": GameBureauPush,
		"data": map[string]any{
			"curBureau": curBureau,
		},
		"pushRouter": "GameMessagePush",
	}
}

//{"type":401,"data":{"gameStatus":1,"tick":0},"pushRouter":"GameMessagePush"}

func GameStatusPushData(gameStatus GameStatus, tick int) any {
	return map[string]any{
		"type": GameStatusPush,
		"data": map[string]any{
			"gameStatus": gameStatus,
			"tick":       tick,
		},
		"pushRouter": "GameMessagePush",
	}
}
func GameSendCardsPushData(handCards [][]int) any {
	return map[string]any{
		"type": GameSendCardsPush,
		"data": map[string]any{
			"handCards": handCards,
		},
		"pushRouter": "GameMessagePush",
	}
}

//{
//    "type":404,
//    "data":{
//        "chairID":0, //座次
//        "score":1, //玩家拥有分数
//        "chairScore":1, //当前座次所下分数
//        "scores":2, //金池 所有用户下的分数
//        "type":0 //
//    },
//    "pushRouter":"GameMessagePush"
//}

func GamePourScorePushData(chairID, score, chairScore, scores, t int) any {
	return map[string]any{
		"type": GamePourScorePush,
		"data": map[string]any{
			"chairID":    chairID,
			"score":      score,
			"chairScore": chairScore,
			"scores":     scores,
			"type":       t,
		},
		"pushRouter": "GameMessagePush",
	}
}

//	{
//	   "type":413,
//	   "data":{"round":1},
//	   "pushRouter":"GameMessagePush"
//	}
func GameRoundPushData(round int) any {
	return map[string]any{
		"type": GameRoundPush,
		"data": map[string]any{
			"round": round,
		},
		"pushRouter": "GameMessagePush",
	}
}

//{
//    "type":406,
//    "data":{"curChairID":1,"curScore":1},
//    "pushRouter":"GameMessagePush"
//}

func GameTurnPushData(curChairID, curScore int) any {
	return map[string]any{
		"type": GameTurnPush,
		"data": map[string]any{
			"curChairID": curChairID,
			"curScore":   curScore,
		},
		"pushRouter": "GameMessagePush",
	}
}
func GameLookPushData(chairID int, cards []int, cuopai bool) any {
	return map[string]any{
		"type": GameLookPush,
		"data": map[string]any{
			"cards":   cards,
			"chairID": chairID,
			"cuopai":  cuopai,
		},
		"pushRouter": "GameMessagePush",
	}
}
func GameComparePushData(fromChairID, toChairID, winChairID, loseChairID int) any {
	return map[string]any{
		"type": GameComparePush,
		"data": map[string]any{
			"fromChairID": fromChairID,
			"toChairID":   toChairID,
			"winChairID":  winChairID,
			"loseChairID": loseChairID,
		},
		"pushRouter": "GameMessagePush",
	}
}

type GameResult struct {
	Winners   []int   `json:"winners"`
	WinScores []int   `json:"winScores"`
	HandCards [][]int `json:"handCards"`
	CurScores []int   `json:"curScores"`
	Losers    []int   `json:"losers"`
}

func GameResultPushData(result *GameResult) any {
	return map[string]any{
		"type": GameResultPush,
		"data": map[string]any{
			"result": result,
		},
		"pushRouter": "GameMessagePush",
	}
}
func GameAbandonPushData(chairID int, userStatus UserStatus) any {
	return map[string]any{
		"type": GameAbandonPush,
		"data": map[string]any{
			"chairID":    chairID,
			"userStatus": userStatus,
		},
		"pushRouter": "GameMessagePush",
	}
}

//{
//    "type":404,
//    "data":{
//        "chairID":0, //座次
//        "score":1, //玩家拥有分数
//        "chairScore":1, //当前座次所下分数
//        "scores":2, //金池 所有用户下的分数
//        "type":0 //
//    },
//    "pushRouter":"GameMessagePush"
//}
