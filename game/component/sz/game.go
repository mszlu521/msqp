package sz

import (
	"common/logs"
	"common/utils"
	"encoding/json"
	"framework/remote"
	"game/component/base"
	"game/component/proto"
	"github.com/jinzhu/copier"
	"time"
)

type GameFrame struct {
	r          base.RoomFrame
	gameRule   proto.GameRule
	gameData   *GameData
	logic      *Logic
	gameResult *GameResult
}

func (g *GameFrame) sendData(data any, users []string, session *remote.Session) {
	g.r.SendData(session.GetMsg(), users, data)
}
func (g *GameFrame) sendDataAll(data any, session *remote.Session) {
	g.r.SendDataAll(session.GetMsg(), data)
}

func (g *GameFrame) OnEventUserOffLine(user *proto.RoomUser, session *remote.Session) {
	//TODO implement me
	panic("implement me")
}

func (g *GameFrame) IsUserEnableLeave() bool {
	return g.gameData.GameStatus == GameStatusNone
}

func (g *GameFrame) GameMessageHandle(user *proto.RoomUser, session *remote.Session, msg []byte) {
	//1. 解析参数
	var req MessageReq
	json.Unmarshal(msg, &req)
	//2. 根据不同的类型 触发不同的操作
	if req.Type == GameLookNotify {
		g.onGameLook(user, session, req.Data.Cuopai)
	} else if req.Type == GamePourScoreNotify {
		g.onGamePourScore(user, session, req.Data.Score, req.Data.Type)
	} else if req.Type == GameCompareNotify {
		g.onGameCompare(user, session, req.Data.ChairID)
	} else if req.Type == GameAbandonNotify {
		g.onGameAbandon(user, session)
	}
}

func NewGameFrame(rule proto.GameRule, r base.RoomFrame) *GameFrame {
	gameData := initGameData(rule)
	return &GameFrame{
		r:        r,
		gameRule: rule,
		gameData: gameData,
		logic:    NewLogic(),
	}
}

func initGameData(rule proto.GameRule) *GameData {
	g := &GameData{
		GameType:   GameType(rule.GameFrameType),
		BaseScore:  rule.BaseScore,
		ChairCount: rule.MaxPlayerCount,
	}
	g.PourScores = make([][]int, g.ChairCount)
	g.HandCards = make([][]int, g.ChairCount)
	g.LookCards = make([]int, g.ChairCount)
	g.CurScores = make([]int, g.ChairCount)
	g.UserStatusArray = make([]UserStatus, g.ChairCount)
	g.UserTrustArray = []bool{false, false, false, false, false, false, false, false, false, false}
	g.Loser = make([]int, 0)
	g.Winner = make([]int, 0)
	return g
}

func (g *GameFrame) GetGameData(session *remote.Session) any {
	user := g.r.GetUsers()[session.GetUid()]
	//判断当前用户 是否是已经看牌 如果已经看牌 返回牌 但是对其他用户仍旧是隐藏状态
	//深拷贝
	var gameData GameData
	copier.CopyWithOption(&gameData, g.gameData, copier.Option{DeepCopy: true})
	for i := 0; i < g.gameData.ChairCount; i++ {
		if g.gameData.HandCards[i] != nil {
			gameData.HandCards[i] = make([]int, 3)
		} else {
			gameData.HandCards[i] = nil
		}
	}
	if g.gameData.LookCards[user.ChairID] == 1 {
		//已经看牌了
		gameData.HandCards[user.ChairID] = g.gameData.HandCards[user.ChairID]
	}
	return gameData
}

func (g *GameFrame) StartGame(session *remote.Session, user *proto.RoomUser) {
	//1.用户信息变更推送（金币变化） {"gold": 9958, "pushRouter": 'UpdateUserInfoPush'}
	users := g.getAllUsers()
	g.sendDataAll(UpdateUserInfoPushGold(user.UserInfo.Gold), session)
	//2.庄家推送 {"type":414,"data":{"bankerChairID":0},"pushRouter":"GameMessagePush"}
	if g.gameData.CurBureau == 0 {
		//庄家是每次开始游戏 首次进行操作的座次
		g.gameData.BankerChairID = utils.Rand(len(users))
	}
	g.gameData.CurChairID = g.gameData.BankerChairID
	g.sendDataAll(GameBankerPushData(g.gameData.BankerChairID), session)
	//3.局数推送{"type":411,"data":{"curBureau":6},"pushRouter":"GameMessagePush"}
	g.gameData.CurBureau++
	g.sendDataAll(GameBureauPushData(g.gameData.CurBureau), session)
	//4.游戏状态推送 分两步推送 第一步 推送 发牌 牌发完之后 第二步 推送下分 需要用户操作了 推送操作
	//{"type":401,"data":{"gameStatus":1,"tick":0},"pushRouter":"GameMessagePush"}
	g.gameData.GameStatus = SendCards
	g.sendDataAll(GameStatusPushData(g.gameData.GameStatus, 0), session)
	//5.发牌推送
	g.sendCards(session)
	//6.下分推送
	//先推送下分状态
	g.gameData.GameStatus = PourScore
	g.sendDataAll(GameStatusPushData(g.gameData.GameStatus, 30), session)
	g.gameData.CurScore = g.gameRule.AddScores[0] * g.gameRule.BaseScore
	for _, v := range g.r.GetUsers() {
		g.sendData(GamePourScorePushData(v.ChairID, g.gameData.CurScore, g.gameData.CurScore, 1, 0), []string{v.UserInfo.Uid}, session)
	}
	//7. 轮数推送
	g.gameData.Round = 1
	g.sendDataAll(GameRoundPushData(g.gameData.Round), session)
	//8. 操作推送
	for _, v := range g.r.GetUsers() {
		//GameTurnPushData ChairID是做操作的座次号（是哪个用户在做操作）
		g.sendData(GameTurnPushData(g.gameData.CurChairID, g.gameData.CurScore), []string{v.UserInfo.Uid}, session)
	}
}

func (g *GameFrame) getAllUsers() []string {
	users := make([]string, 0)
	for _, v := range g.r.GetUsers() {
		users = append(users, v.UserInfo.Uid)
	}
	return users
}

func (g *GameFrame) sendCards(session *remote.Session) {
	//这就要发牌了 牌相关的逻辑
	//1.洗牌 然后发牌
	g.logic.washCards()
	for i := 0; i < g.gameData.ChairCount; i++ {
		if g.IsPlayingChairID(i) {
			g.gameData.HandCards[i] = g.logic.getCards()
		}
	}
	//发牌后 推送的时候 如果没有看牌的话 暗牌
	hands := make([][]int, g.gameData.ChairCount)
	for i, v := range g.gameData.HandCards {
		if v != nil {
			hands[i] = []int{0, 0, 0}
		}
	}
	g.sendDataAll(GameSendCardsPushData(hands), session)
}

func (g *GameFrame) IsPlayingChairID(chairID int) bool {
	for _, v := range g.r.GetUsers() {
		if v.ChairID == chairID && v.UserStatus == proto.Playing {
			return true
		}
	}
	return false
}

func (g *GameFrame) onGameLook(user *proto.RoomUser, session *remote.Session, cuopai bool) {
	//判断 如果是当前用户 推送其牌 给其他用户 推送此用户已经看牌
	if g.gameData.GameStatus != PourScore || g.gameData.CurChairID != user.ChairID {
		logs.Warn("ID:%s room, sanzhang game look err:gameStatus=%d,curChairID=%d,chairID=%d",
			g.r.GetId(), g.gameData.GameStatus, g.gameData.CurChairID, user.ChairID)
		return
	}
	if !g.IsPlayingChairID(user.ChairID) {
		logs.Warn("ID:%s room, sanzhang game look err: not playing",
			g.r.GetId())
		return
	}
	//代表已看牌
	g.gameData.UserStatusArray[user.ChairID] = Look
	g.gameData.LookCards[user.ChairID] = 1
	for _, v := range g.r.GetUsers() {
		if g.gameData.CurChairID == v.ChairID {
			//代表操作用户
			//{"type":403,"data":{"chairID":1,"cards":[60,2,44],"cuopai":false},"pushRouter":"GameMessagePush"}
			g.sendData(GameLookPushData(g.gameData.CurChairID, g.gameData.HandCards[v.ChairID], cuopai), []string{v.UserInfo.Uid}, session)
		} else {
			g.sendData(GameLookPushData(g.gameData.CurChairID, nil, cuopai), []string{v.UserInfo.Uid}, session)

		}
	}
}

func (g *GameFrame) onGamePourScore(user *proto.RoomUser, session *remote.Session, score int, t int) {
	//1. 处理下分 保存用户下的分数 同时推送当前用户下分的信息到客户端
	if g.gameData.GameStatus != PourScore || g.gameData.CurChairID != user.ChairID {
		logs.Warn("ID:%s room, sanzhang onGamePourScore err:gameStatus=%d,curChairID=%d,chairID=%d",
			g.r.GetId(), g.gameData.GameStatus, g.gameData.CurChairID, user.ChairID)
		return
	}
	if !g.IsPlayingChairID(user.ChairID) {
		logs.Warn("ID:%s room, sanzhang onGamePourScore err: not playing",
			g.r.GetId())
		return
	}
	if score < 0 {
		logs.Warn("ID:%s room, sanzhang onGamePourScore err: score lt zero",
			g.r.GetId())
		return
	}
	if g.gameData.PourScores[user.ChairID] == nil {
		g.gameData.PourScores[user.ChairID] = make([]int, 0)
	}
	g.gameData.PourScores[user.ChairID] = append(g.gameData.PourScores[user.ChairID], score)
	//所有人的分数
	scores := 0
	for i := 0; i < g.gameData.ChairCount; i++ {
		if g.gameData.PourScores[i] != nil {
			for _, sc := range g.gameData.PourScores[i] {
				scores += sc
			}
		}
	}
	//当前座次的总分
	chairCount := 0
	for _, sc := range g.gameData.PourScores[user.ChairID] {
		chairCount += sc
	}
	g.sendDataAll(GamePourScorePushData(user.ChairID, score, chairCount, scores, t), session)
	//2. 结束下分 座次移动到下一位 推送轮次 推送游戏状态 推送操作的座次
	g.endPourScore(session)
}

func (g *GameFrame) endPourScore(session *remote.Session) {
	//1. 推送轮次 TODO 轮数大于规则的限制 结束游戏 进行结算
	round := g.getCurRound()
	g.sendDataAll(GameRoundPushData(round), session)
	//判断当前的玩家 没有lose的 只剩下一个的时候
	gamerCount := 0
	for i := 0; i < g.gameData.ChairCount; i++ {
		if g.IsPlayingChairID(i) && !utils.Contains(g.gameData.Loser, i) {
			gamerCount++
		}
	}
	if gamerCount == 1 {
		g.startResult(session)
	} else {
		//2. 座次要向前移动一位
		for i := 0; i < g.gameData.ChairCount; i++ {
			g.gameData.CurChairID++
			g.gameData.CurChairID = g.gameData.CurChairID % g.gameData.ChairCount
			if g.IsPlayingChairID(g.gameData.CurChairID) {
				break
			}
		}
		//推送游戏状态
		g.gameData.GameStatus = PourScore
		g.sendDataAll(GameStatusPushData(g.gameData.GameStatus, 30), session)
		//该谁操作了
		g.sendDataAll(GameTurnPushData(g.gameData.CurChairID, g.gameData.CurScore), session)

	}
}

func (g *GameFrame) getCurRound() int {
	cur := g.gameData.CurChairID
	for i := 0; i < g.gameData.ChairCount; i++ {
		cur++
		cur = cur % g.gameData.ChairCount
		if g.IsPlayingChairID(cur) {
			return len(g.gameData.PourScores[cur])
		}
	}
	return 1
}

func (g *GameFrame) onGameCompare(user *proto.RoomUser, session *remote.Session, chairID int) {
	//1. TODO 先下分 跟注结束之后 进行比牌
	//2. 比牌
	fromChairID := user.ChairID
	toChairID := chairID
	result := g.logic.CompareCards(g.gameData.HandCards[fromChairID], g.gameData.HandCards[toChairID])
	//3. 处理比牌结果 推送轮数 状态 显示结果等信息
	if result == 0 {
		//主动比牌者 如果是和 主动比牌者输
		result = -1
	}
	winChairID := -1
	loseChairID := -1
	if result > 0 {
		g.sendDataAll(GameComparePushData(fromChairID, toChairID, fromChairID, toChairID), session)
		winChairID = fromChairID
		loseChairID = toChairID
	} else if result < 0 {
		g.sendDataAll(GameComparePushData(fromChairID, toChairID, toChairID, fromChairID), session)
		winChairID = toChairID
		loseChairID = fromChairID
	}
	if winChairID != -1 && loseChairID != -1 {
		g.gameData.UserStatusArray[winChairID] = Win
		g.gameData.UserStatusArray[loseChairID] = Lose
		g.gameData.Loser = append(g.gameData.Loser, loseChairID)
		g.gameData.Winner = append(g.gameData.Winner, winChairID)
	}
	if winChairID == fromChairID {
		//TODO 赢了之后 继续和其他人进行比牌
	}
	g.endPourScore(session)
}

func (g *GameFrame) startResult(session *remote.Session) {
	//推送 游戏结果状态
	g.gameData.GameStatus = Result
	g.sendDataAll(GameStatusPushData(g.gameData.GameStatus, 0), session)
	if g.gameResult == nil {
		g.gameResult = new(GameResult)
	}
	g.gameResult.Winners = g.gameData.Winner
	g.gameResult.HandCards = g.gameData.HandCards
	g.gameResult.CurScores = g.gameData.CurScores
	g.gameResult.Losers = g.gameData.Loser
	winScores := make([]int, g.gameData.ChairCount)
	for i := range winScores {
		if g.gameData.PourScores[i] != nil {
			scores := 0
			for _, v := range g.gameData.PourScores[i] {
				scores += v
			}
			winScores[i] = -scores
			for win := range g.gameData.Winner {
				winScores[win] += scores / len(g.gameData.Winner)
			}
		}
	}
	g.gameResult.WinScores = winScores
	g.sendDataAll(GameResultPushData(g.gameResult), session)
	//结算完成 重置游戏 开始下一把
	g.resetGame(session)
	g.gameEnd(session)
}

func (gf *GameFrame) resetGame(session *remote.Session) {
	g := &GameData{
		GameType:   GameType(gf.gameRule.GameFrameType),
		BaseScore:  gf.gameRule.BaseScore,
		ChairCount: gf.gameRule.MaxPlayerCount,
	}
	g.PourScores = make([][]int, g.ChairCount)
	g.HandCards = make([][]int, g.ChairCount)
	g.LookCards = make([]int, g.ChairCount)
	g.CurScores = make([]int, g.ChairCount)
	g.UserStatusArray = make([]UserStatus, g.ChairCount)
	g.UserTrustArray = []bool{false, false, false, false, false, false, false, false, false, false}
	g.Loser = make([]int, 0)
	g.Winner = make([]int, 0)
	g.GameStatus = GameStatus(None)
	gf.gameData = g
	gf.SendGameStatus(g.GameStatus, 0, session)
	gf.r.EndGame(session)
}

func (g *GameFrame) SendGameStatus(status GameStatus, tick int, session *remote.Session) {
	g.sendDataAll(GameStatusPushData(status, tick), session)
}

func (g *GameFrame) gameEnd(session *remote.Session) {
	//赢家当庄家
	for i := 0; i < g.gameData.ChairCount; i++ {
		if g.gameResult.WinScores[i] > 0 {
			g.gameData.BankerChairID = i
			g.gameData.CurChairID = g.gameData.BankerChairID
		}
	}
	time.AfterFunc(5*time.Second, func() {
		for _, v := range g.r.GetUsers() {
			g.r.UserReady(v.UserInfo.Uid, session)
		}
	})
}

func (g *GameFrame) onGameAbandon(user *proto.RoomUser, session *remote.Session) {
	if !g.IsPlayingChairID(user.ChairID) {
		return
	}
	if utils.Contains(g.gameData.Loser, user.ChairID) {
		return
	}
	g.gameData.Loser = append(g.gameData.Loser, user.ChairID)
	for i := 0; i < g.gameData.ChairCount; i++ {
		if g.IsPlayingChairID(i) && i != user.ChairID {
			g.gameData.Winner = append(g.gameData.Winner, i)
		}
	}
	g.gameData.UserStatusArray[user.ChairID] = Abandon
	//推送弃牌
	g.sendDataAll(GameAbandonPushData(user.ChairID, g.gameData.UserStatusArray[user.ChairID]), session)

	time.AfterFunc(time.Second, func() {
		g.endPourScore(session)
	})
}
