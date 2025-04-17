package sz

import (
	"common/logs"
	"common/tasks"
	"common/utils"
	"core/models/enums"
	"encoding/json"
	"framework/remote"
	"game/component/base"
	"game/component/proto"
	"github.com/jinzhu/copier"
	"time"
)

type GameFrame struct {
	r                       base.RoomFrame
	gameRule                proto.GameRule
	gameData                *GameData
	UserWinRecord           map[string]*UserWinRecord
	ReviewRecord            []*BureauReview
	logic                   *Logic
	gameResult              *GameResult
	pourScoreScheduleID     *tasks.Task
	startPourScoreID        *time.Timer
	forcePrepareID          *tasks.Task
	userTrustID             *tasks.Task
	sendCardsScheduleID     *time.Timer
	compareID               *time.Timer
	endResultID             *time.Timer
	stopPourScoreScheduleID chan struct{}
	stopForcePrepareID      chan struct{}
}

func (g *GameFrame) GetGameBureauData() any {
	return g.ReviewRecord
}

func (g *GameFrame) GetGameVideoData() any {
	return nil
}

type DismissResult struct {
	Uid      string `json:"uid"`
	Nickname string `json:"nickname"`
	Score    int    `json:"score"`
	Avatar   string `json:"avatar"`
}

func (g *GameFrame) OnEventRoomDismiss(reason enums.RoomDismissReason, session *remote.Session) {
	g.delScheduleIDs()
	var result = make([]*DismissResult, 0)
	for _, v := range g.UserWinRecord {
		result = append(result, &DismissResult{
			Uid:      v.Uid,
			Nickname: v.Nickname,
			Score:    v.Score,
			Avatar:   v.Avatar,
		})
	}
	var creator Creator
	for _, v := range g.r.GetUsers() {
		if v.UserInfo.Uid == g.r.GetCreator().Uid {
			creator = Creator{
				Uid:      v.UserInfo.Uid,
				Nickname: v.UserInfo.Nickname,
				Avatar:   v.UserInfo.Avatar,
			}
		}
	}
	var winMost any
	var lostMost any
	if len(result) > 0 {
		win := 0
		lost := 0
		for index, v := range result {
			if v.Score > result[win].Score {
				win = index
			}
			if v.Score < result[lost].Score {
				lost = index
			}
		}
		winMost = result[win].Uid
		lostMost = result[lost].Uid
	}
	g.sendDataAll(GameEndPushData(result, winMost, lostMost, &creator, nil), session)
}

func (g *GameFrame) OnEventGameStart(user *proto.RoomUser, session *remote.Session) {
	g.startGame(session)
}

func (g *GameFrame) OnEventUserEntry(user *proto.RoomUser, session *remote.Session) {
	//TODO implement me
}

func (g *GameFrame) sendData(data any, users []string, session *remote.Session) {
	g.r.SendData(session.GetMsg(), users, data)
}
func (g *GameFrame) sendDataAll(data any, session *remote.Session) {
	g.r.SendDataAll(session.GetMsg(), data)
}

func (g *GameFrame) OnEventUserOffLine(user *proto.RoomUser, session *remote.Session) {
	if g.gameData.CurChairID == user.ChairID {
		g.offlineUserAutoOperation(user, session)
	}
}

func (g *GameFrame) IsUserEnableLeave(int) bool {
	return g.gameData.GameStatus == GameStatusNone
}

func (g *GameFrame) GameMessageHandle(user *proto.RoomUser, session *remote.Session, msg []byte) {
	//1. 解析参数
	var req MessageReq
	json.Unmarshal(msg, &req)
	//2. 根据不同的类型 触发不同的操作
	if req.Type == GameLookNotify {
		g.onGameLook(user, req.Data.Cuopai, true, session)
	} else if req.Type == GamePourScoreNotify {
		g.onGamePourScore(user.ChairID, req.Data.Score, req.Data.Type, false, false, session)
	} else if req.Type == GameCompareNotify {
		g.onGameCompare(user.ChairID, req.Data.ChairID, false, false, session)
	} else if req.Type == GameAbandonNotify {
		g.onGameAbandon(user.ChairID, req.Data.Type, true, session)
	} else if req.Type == GameChatNotify {
		g.onGameChat(user, req.Data, session)
	} else if req.Type == GameTrustNotify {
		g.onGameTrust(user, req.Data.Trust, session)
	} else if req.Type == GameReviewNotify {
		g.onGameReview(user, session)
	}
}

func NewGameFrame(rule proto.GameRule, r base.RoomFrame, session *remote.Session) *GameFrame {
	gameData := initGameData(rule)
	g := &GameFrame{
		r:                       r,
		gameRule:                rule,
		gameData:                gameData,
		UserWinRecord:           make(map[string]*UserWinRecord),
		ReviewRecord:            make([]*BureauReview, 0),
		logic:                   NewLogic(),
		stopPourScoreScheduleID: make(chan struct{}, 1),
		stopForcePrepareID:      make(chan struct{}, 1),
	}
	g.resetGame(session)
	go g.stopSchedule()
	return g
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
	g.UserTrustArray = make([]bool, g.ChairCount)
	g.TrustTmArray = make([]int, g.ChairCount)
	g.Loser = make([]int, 0)
	g.Winner = make([]int, 0)
	return g
}

func (g *GameFrame) GetEnterGameData(session *remote.Session) any {
	user := g.r.GetUsers()[session.GetUid()]
	//判断当前用户 是否是已经看牌 如果已经看牌 返回牌 但是对其他用户仍旧是隐藏状态
	//深拷贝
	var gameData GameData
	copier.CopyWithOption(&gameData, g.gameData, copier.Option{DeepCopy: true})
	gameData.CurScores = g.getCurScores()
	if g.gameData.GameStatus != Result {
		handCards := make([][]int, g.gameData.ChairCount)
		for i := 0; i < g.gameData.ChairCount; i++ {
			if g.gameData.HandCards[i] != nil {
				handCards[i] = make([]int, 3)
			} else {
				handCards[i] = nil
			}
		}
		if user.ChairID < len(g.gameData.LookCards) {
			//确保数据正确
			if g.gameData.LookCards[user.ChairID] != 0 {
				handCards[user.ChairID] = g.gameData.HandCards[user.ChairID]
			}
		}
		gameData.HandCards = handCards
	}
	gameData.CurBureau = g.r.GetCurBureau()
	gameData.MaxBureau = g.gameRule.Bureau
	g.gameData.CurBureau = g.r.GetCurBureau()
	g.gameData.MaxBureau = g.gameRule.Bureau
	return gameData
}

func (g *GameFrame) startGame(session *remote.Session) {
	g.gameData.GameStarter = true
	if g.forcePrepareID != nil {
		g.forcePrepareID.Stop()
		g.forcePrepareID = nil
	}
	if g.gameRule.CanTrust && g.userTrustID == nil {
		g.userTrustID = tasks.NewTask("userTrustID", time.Second, func() {
			if g.r.IsDismissing() {
				return
			}
			for _, u := range g.r.GetUsers() {
				if u.ChairID == g.gameData.CurChairID &&
					!g.gameData.UserTrustArray[u.ChairID] &&
					g.gameData.GameStatus == PourScore {
					g.gameData.TrustTmArray[u.ChairID]++
					if g.gameData.TrustTmArray[u.ChairID] >= 30 {
						g.onGameTrust(u, true, session)
					}
				}
			}
		})
	}
	if g.r.GetCurBureau() == 0 {
		var idArray []int
		for _, v := range g.r.GetUsers() {
			idArray = append(idArray, v.ChairID)
		}
		g.gameData.BankerChairID = idArray[utils.Rand(len(idArray))]
	}
	for i := 0; i < g.gameData.ChairCount; i++ {
		g.gameData.CurChairID = (g.gameData.BankerChairID + i + 1 + g.gameData.ChairCount) % g.gameData.ChairCount
		if g.IsPlayingChairID(g.gameData.CurChairID) {
			break
		}
	}
	g.gameData.CurScore = g.gameRule.AddScores[0] * g.gameData.BaseScore
	g.sendDataAll(GameBankerPushData(g.gameData.BankerChairID), session)
	g.gameData.CurBureau++
	g.r.SetCurBureau(g.gameData.CurBureau)
	g.sendDataAll(GameBureauPushData(g.r.GetCurBureau()), session)
	g.startSendCards(session)
	pourScores := 0
	for i := 0; i < g.gameData.ChairCount; i++ {
		if g.IsPlayingChairID(i) {
			g.gameData.PourScores[i] = []int{g.gameData.CurScore}
			pourScores += g.gameData.CurScore
		}
	}
	for i := 0; i < g.gameData.ChairCount; i++ {
		if g.IsPlayingChairID(i) {
			g.sendDataAll(GamePourScorePushData(i, g.gameData.CurScore, g.gameData.CurScore, pourScores, PourTypeNone), session)
		}
	}
	g.gameData.Round = 1
	g.sendDataAll(GameRoundPushData(g.gameData.Round), session)
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

func (g *GameFrame) getUserByChairID(chairID int) *proto.RoomUser {
	for _, v := range g.r.GetUsers() {
		if v.ChairID == chairID {
			return v
		}
	}
	return nil
}
func (g *GameFrame) IsPlayingChairID(chairID int) bool {
	user := g.getUserByChairID(chairID)
	if user != nil {
		if user.UserStatus&enums.Ready > 0 || user.UserStatus&enums.Playing > 0 {
			return true
		}
	}
	return false
}

func (g *GameFrame) onGameLook(user *proto.RoomUser, cuopai bool, fromUser bool, session *remote.Session) {
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
	/* 闷123轮 */
	if g.gameData.GameType == Men1 && g.gameData.Round <= 1 {
		return
	} else if g.gameData.GameType == Men2 && g.gameData.Round <= 2 {
		return
	} else if g.gameData.GameType == Men3 && g.gameData.Round <= 3 {
		return
	}
	if fromUser {
		g.gameData.TrustTmArray[user.ChairID] = 0
	}
	//代表已看牌
	g.gameData.UserStatusArray[user.ChairID] = Look
	g.gameData.LookCards[user.ChairID] = 1
	for _, v := range g.r.GetUsers() {
		if user.ChairID == v.ChairID {
			//代表操作用户
			//{"type":403,"data":{"chairID":1,"cards":[60,2,44],"cuopai":false},"pushRouter":"GameMessagePush"}
			g.sendData(GameLookPushData(g.gameData.CurChairID, g.gameData.HandCards[v.ChairID], cuopai), []string{v.UserInfo.Uid}, session)
		} else {
			g.sendData(GameLookPushData(g.gameData.CurChairID, nil, cuopai), []string{v.UserInfo.Uid}, session)

		}
	}
}

/*
*

	fromCompare 比牌下注 true
	types 1跟注 2加注 0其他方式（底或比牌）
	force bool 强制下分
*/
func (g *GameFrame) onGamePourScore(chairID int,
	score int,
	types int,
	fromCompare bool,
	force bool,
	session *remote.Session) bool {
	user := g.getUserByChairID(chairID)
	if !g.isUserPlaying(user) {
		return false
	}
	if !force && !g.userHasEnoughScore(user.ChairID) {
		restChairIDArr := g.getRestChairIDArray(session)
		restChairIDArr = append(restChairIDArr, user.ChairID)
		restChairIDArr = utils.Splice(restChairIDArr, utils.IndexOf(restChairIDArr, user.ChairID), 1)
		if len(restChairIDArr) > 0 {
			g.onGameCompare(user.ChairID, utils.Pop(restChairIDArr), false, true, session)
		}
		return false
	}
	hasPour := 0
	for _, v := range g.gameData.PourScores[user.ChairID] {
		hasPour += v
	}
	if score < 0 {
		return false
	}
	look := 1
	if g.gameData.LookCards[user.ChairID] > 0 {
		look = 2
	}
	if g.isUnionCreate() && look*score+hasPour > user.UserInfo.Score && !fromCompare {
		return false
	}
	if g.gameData.GameStatus != PourScore ||
		utils.IndexOf(g.gameData.Loser, user.ChairID) != -1 ||
		(score < g.gameData.CurScore && !fromCompare) ||
		g.gameData.CurChairID != user.ChairID {
		return false
	}
	if !fromCompare {
		g.gameData.TrustTmArray[user.ChairID] = 0
	}
	if score > g.gameRule.MaxScore*g.gameData.BaseScore {
		score = g.gameRule.MaxScore * g.gameData.BaseScore
	}
	if score > g.gameData.CurScore {
		g.gameData.CurScore = score
	}
	if g.gameData.PourScores[user.ChairID] == nil {
		g.gameData.PourScores[user.ChairID] = make([]int, 0)
	}
	if g.gameData.LookCards[user.ChairID] > 0 {
		score *= 2
	}
	if g.isUnionCreate() && score > user.UserInfo.Score-hasPour {
		score = user.UserInfo.Score - hasPour
	}
	g.gameData.PourScores[user.ChairID] = append(g.gameData.PourScores[user.ChairID], score)
	if score > 0 {
		scores := 0
		chairScore := 0
		for i := 0; i < g.gameData.ChairCount; i++ {
			if g.gameData.PourScores[i] != nil {
				for sc := range g.gameData.PourScores[i] {
					scores += sc
				}
			}
		}
		for sc := range g.gameData.PourScores[user.ChairID] {
			chairScore += sc
		}
		g.sendDataAll(GamePourScorePushData(user.ChairID, score, chairScore, scores, types), session)
	}
	if !fromCompare {
		//结束下分 不是比牌下注
		g.endPourScore(false, session)
	}
	return true
}

/*
 * 结束下分
 * comparing 分数不够，正在持续比牌中
 */
func (g *GameFrame) endPourScore(comparing bool, session *remote.Session) {
	if g.pourScoreScheduleID != nil {
		g.pourScoreScheduleID.Stop()
		g.pourScoreScheduleID = nil
	}
	if comparing {
		var restChairIDArr []int
		for _, user := range g.r.GetUsers() {
			if g.isUserPlaying(user) && utils.IndexOf(restChairIDArr, user.ChairID) == -1 {
				restChairIDArr = append(restChairIDArr, user.ChairID)
			}
		}
		restChairIDArr = utils.Splice(restChairIDArr, utils.IndexOf(restChairIDArr, g.gameData.CurChairID), 1)
		if len(restChairIDArr) > 0 {
			g.onGameCompare(g.gameData.CurChairID, utils.Pop(restChairIDArr), false, true, session)
			return
		}
	}
	//1. 推送轮次
	g.gameData.Round = g.getCurRound()
	maxRound := rounds[g.gameRule.RoundType]
	lastChairID := g.gameData.BankerChairID
	for i := 0; i < g.gameData.ChairCount; i++ {
		lastChairID = (g.gameData.BankerChairID - i + g.gameData.ChairCount) % g.gameData.ChairCount
		if g.IsPlayingChairID(lastChairID) && !utils.Contains(g.gameData.Loser, lastChairID) {
			break
		}
	}
	if g.gameData.Round > maxRound && g.gameData.CurChairID == lastChairID {
		g.startResult(session)
		return
	} else {
		g.sendDataAll(GameRoundPushData(g.gameData.Round), session)
	}
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
			g.gameData.CurChairID = (g.gameData.CurChairID + 1) % g.gameData.ChairCount
			if g.IsPlayingChairID(g.gameData.CurChairID) && !utils.Contains(g.gameData.Loser, g.gameData.CurChairID) {
				break
			}
		}
		g.startPourScore(session)
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

/*
*
fromEnd 是否是最后统一比牌
force 强制比牌
*/
func (g *GameFrame) onGameCompare(fromChairID int, toChairID int, fromEnd, force bool, session *remote.Session) {
	if !g.IsPlayingChairID(fromChairID) || !g.IsPlayingChairID(toChairID) {
		return
	}
	if utils.IndexOf(g.gameData.Loser, fromChairID) != -1 || utils.IndexOf(g.gameData.Loser, toChairID) != -1 {
		return
	}
	if g.gameData.GameStatus == PourScore && fromChairID != g.gameData.CurChairID {
		return
	}
	if g.gameData.GameStatus == GameStatusNone || g.gameData.GameStatus == SendCards {
		return
	}
	if !force && !g.userHasEnoughScore(fromChairID) {
		restChairIDArr := g.getRestChairIDArray(session)
		restChairIDArr = utils.Splice(restChairIDArr, utils.IndexOf(restChairIDArr, fromChairID), 1)
		g.onGameCompare(fromChairID, utils.Pop(restChairIDArr), false, true, session)
		return
	}
	//手动比牌
	if g.gameData.GameStatus == PourScore {
		if !g.onGamePourScore(fromChairID, g.gameData.CurScore, PourTypeNone, true, true, session) {
			//金币不足比牌失败
			return
		}
		if fromEnd {
			g.gameData.TrustTmArray[fromChairID] = 0
		}
	}
	//2. 比牌
	result := g.logic.CompareCards(g.gameData.HandCards[fromChairID], g.gameData.HandCards[toChairID])
	loseChairID := -1
	winChairID := -1
	if result == 0 {
		if !fromEnd {
			//不是最后统一比牌，先比牌输
			result = -1
		}
	}
	if result > 0 {
		if g.gameData.GameStatus == PourScore {
			g.sendDataAll(GameComparePushData(fromChairID, toChairID, fromChairID, toChairID), session)
		}
		loseChairID = toChairID
		winChairID = fromChairID
	} else if result < 0 {
		if g.gameData.GameStatus == PourScore {
			g.sendDataAll(GameComparePushData(fromChairID, toChairID, toChairID, fromChairID), session)
		}
		loseChairID = fromChairID
		winChairID = toChairID
	}
	if loseChairID != -1 && winChairID != -1 {
		g.gameData.UserStatusArray[loseChairID] |= Lose
		g.gameData.UserStatusArray[winChairID] |= Win
		g.gameData.Loser = append(g.gameData.Loser, loseChairID)
	} else {
		g.gameData.UserStatusArray[fromChairID] |= He
		g.gameData.UserStatusArray[toChairID] |= He
	}
	if g.pourScoreScheduleID != nil {
		g.pourScoreScheduleID.Stop()
		g.pourScoreScheduleID = nil
	}
	if g.gameData.GameStatus == PourScore {
		if g.compareID != nil {
			g.compareID.Stop()
		}
		g.compareID = time.AfterFunc(3*time.Second, func() {
			g.endPourScore((winChairID == fromChairID) && force, session)
		})
	}
}

func (g *GameFrame) startResult(session *remote.Session) {
	g.gameData.Tick = 0
	//推送 游戏结果状态
	g.gameData.GameStatus = Result
	g.SendGameStatus(session)
	var gamerChairIDs []int
	var heChairIDs []int
	for i := 0; i < g.gameData.ChairCount; i++ {
		chairID := (g.gameData.BankerChairID + i) % g.gameData.ChairCount
		if g.IsPlayingChairID(chairID) && utils.IndexOf(g.gameData.Loser, chairID) == -1 {
			gamerChairIDs = append(gamerChairIDs, chairID)
		}
	}
	for len(gamerChairIDs) > 1 || (len(gamerChairIDs) == 1 && len(heChairIDs) > 0) {
		if len(heChairIDs) > 0 {
			g.onGameCompare(heChairIDs[0], gamerChairIDs[0], true, true, session)
			if g.gameData.UserStatusArray[heChairIDs[0]]&Lose > 0 {
				heChairIDs = utils.Splice(heChairIDs, 0, 1)
			}
			if g.gameData.UserStatusArray[gamerChairIDs[0]]&Lose > 0 {
				gamerChairIDs = utils.Splice(gamerChairIDs, 0, 1)
			} else if g.gameData.UserStatusArray[gamerChairIDs[0]]&He > 0 {
				heChairIDs = append(heChairIDs, gamerChairIDs[0])
				gamerChairIDs = utils.Splice(gamerChairIDs, 0, 1)
			}
		} else {
			g.onGameCompare(gamerChairIDs[0], gamerChairIDs[1], true, true, session)
			if g.gameData.UserStatusArray[gamerChairIDs[1]]&Lose > 0 {
				gamerChairIDs = utils.Splice(gamerChairIDs, 1, 1)
			} else if g.gameData.UserStatusArray[gamerChairIDs[1]]&He > 0 {
				heChairIDs = append(heChairIDs, gamerChairIDs[1])
				gamerChairIDs = utils.Splice(gamerChairIDs, 1, 1)
			}
			if g.gameData.UserStatusArray[gamerChairIDs[0]]&Lose > 0 {
				gamerChairIDs = utils.Splice(gamerChairIDs, 0, 1)
			} else if g.gameData.UserStatusArray[gamerChairIDs[0]]&He > 0 {
				heChairIDs = append(heChairIDs, gamerChairIDs[0])
				gamerChairIDs = utils.Splice(gamerChairIDs, 0, 1)
			}
		}
	}
	winners := append(heChairIDs, gamerChairIDs...)
	winScores := make([]int, g.gameData.ChairCount)
	for i, v := range g.gameData.PourScores {
		if v != nil && utils.IndexOf(winners, i) == -1 {
			scores := 0
			for score := range v {
				scores += score
			}
			winScores[i] = -scores
			for winner := range winners {
				winScores[winner] += scores / len(winners)
			}
		}
	}
	for i := 0; i < len(winScores); i++ {
		user := g.getUserByChairID(i)
		if winScores[i] != 0 && user != nil {
			if g.UserWinRecord[user.UserInfo.Uid] == nil {
				g.UserWinRecord[user.UserInfo.Uid] = &UserWinRecord{
					Uid:      user.UserInfo.Uid,
					Nickname: user.UserInfo.Nickname,
					Avatar:   user.UserInfo.Avatar,
					Score:    0,
				}
			}
			g.UserWinRecord[user.UserInfo.Uid].Score += winScores[i]
		}
	}
	result := &GameResult{
		Winners:   winners,
		WinScores: winScores,
		HandCards: g.gameData.HandCards,
		CurScores: g.getCurScores(),
	}
	g.gameResult = result
	g.sendDataAll(GameResultPushData(result), session)
	var bureauReviews []*BureauReview
	//牌面回顾记录
	for _, user := range g.r.GetUsers() {
		if !g.IsPlayingChairID(user.ChairID) {
			continue
		}
		pourScore := 0
		for score := range g.gameData.PourScores[user.ChairID] {
			pourScore += score
		}
		bureauReview := &BureauReview{
			Uid:       user.UserInfo.Uid,
			Nickname:  user.UserInfo.Nickname,
			Avatar:    user.UserInfo.Avatar,
			Cards:     g.gameData.HandCards[user.ChairID],
			PourScore: pourScore,
			WinScore:  winScores[user.ChairID],
			IsBanker:  g.gameData.BankerChairID == user.ChairID,
			IsAbandon: g.gameData.UserStatusArray[user.ChairID]&Abandon > 0,
		}
		bureauReviews = append(bureauReviews, bureauReview)
	}
	g.ReviewRecord = append(g.ReviewRecord, bureauReviews...)
	if g.endResultID != nil {
		g.endResultID.Stop()
		g.endResultID = nil
	}
	g.endResultID = time.AfterFunc(3*time.Second, func() {
		g.endResult(session)
	})
}

func (g *GameFrame) resetGame(session *remote.Session) {
	g.gameData.GameStatus = GameStatusNone
	g.gameData.Tick = 0
	g.gameData.HandCards = make([][]int, g.gameData.ChairCount)
	g.gameData.PourScores = make([][]int, g.gameData.ChairCount)
	g.gameData.LookCards = make([]int, g.gameData.ChairCount)
	g.gameData.Loser = make([]int, 0)
	g.gameData.UserStatusArray = make([]UserStatus, g.gameData.ChairCount)
	g.gameData.Round = 0
	g.SendGameStatus(session)
}

func (g *GameFrame) SendGameStatus(session *remote.Session) {
	g.sendDataAll(GameStatusPushData(g.gameData.GameStatus, g.gameData.Tick), session)
}

func (g *GameFrame) gameEnd(session *remote.Session) {
	winScores := g.gameResult.WinScores
	var endData []*proto.EndData
	for i := 0; i < len(winScores); i++ {
		user := g.getUserByChairID(i)
		if user != nil {
			endData = append(endData, &proto.EndData{
				Uid:   user.UserInfo.Uid,
				Score: winScores[i],
			})
		}
		if winScores[i] > 0 {
			g.gameData.BankerChairID = i
		}
	}
	g.r.ConcludeGame(endData, session)
	g.gameData.Tick = 3
	if g.r.GetCurBureau() != g.r.GetMaxBureau() {
		if g.forcePrepareID != nil {
			g.forcePrepareID.Stop()
			g.forcePrepareID = nil
		}
		g.forcePrepareID = tasks.NewTask("forcePrepareID", 1*time.Second, func() {
			if g.r.IsDismissing() {
				return
			}
			g.gameData.Tick--
			if g.gameData.Tick <= 0 {
				if g.gameData.GameStatus == GameStatusNone {
					for _, user := range g.r.GetUsers() {
						if user.UserStatus&enums.Ready > 0 {
							//手动准备过，倒计时清零
							g.gameData.TrustTmArray[user.ChairID] = 0
						}
						if user.ChairID < g.gameData.ChairCount &&
							user.UserStatus&enums.Ready == 0 &&
							g.gameData.GameStatus == GameStatusNone &&
							!g.gameData.GameStarter {
							g.r.UserReady(user.UserInfo.Uid, session)
						}
					}
				}
				g.stopForcePrepareID <- struct{}{}
			}
		})
	}
}

func (g *GameFrame) onGameAbandon(chairID int, types int, fromUser bool, session *remote.Session) {
	if !g.IsPlayingChairID(chairID) {
		return
	}
	if utils.Contains(g.gameData.Loser, chairID) ||
		g.gameData.GameStatus != PourScore || g.gameData.CurChairID != chairID {
		return
	}
	if g.pourScoreScheduleID != nil {
		g.pourScoreScheduleID.Stop()
		g.pourScoreScheduleID = nil
	}
	g.gameData.UserStatusArray[chairID] |= Abandon
	if fromUser {
		g.gameData.TrustTmArray[chairID] = 0
	} else {
		g.gameData.UserStatusArray[chairID] |= TimeoutAbandon
	}
	g.gameData.Loser = append(g.gameData.Loser, chairID)
	g.sendDataAll(GameAbandonPushData(chairID, g.gameData.UserStatusArray[chairID], types), session)
	time.AfterFunc(time.Second, func() {
		g.endPourScore(false, session)
	})
}

func (g *GameFrame) getCurScores() []int {
	curScores := make([]int, g.gameData.ChairCount)
	for i := 0; i < g.gameData.ChairCount; i++ {
		curScores[i] = 0
	}
	for _, user := range g.r.GetUsers() {
		if user.ChairID < g.gameData.ChairCount {
			if g.UserWinRecord[user.UserInfo.Uid] != nil {
				curScores[user.ChairID] = g.UserWinRecord[user.UserInfo.Uid].Score
			}
		}
	}
	return curScores
}

func (g *GameFrame) onGameChat(user *proto.RoomUser, data MessageData, session *remote.Session) {
	g.sendDataAll(gameChatPushData(user.ChairID, data.Type, data.Msg, data.RecipientID), session)
}

func (g *GameFrame) onGameTrust(user *proto.RoomUser, trust bool, session *remote.Session) {
	if user.ChairID >= g.gameData.ChairCount {
		return
	}
	g.gameData.UserTrustArray[user.ChairID] = trust
	g.gameData.TrustTmArray[user.ChairID] = 0
	g.sendDataAll(gameTrustPushData(user.ChairID, trust), session)
	if trust {
		g.userAutoOperate(user, session)
	}
}

func (g *GameFrame) userAutoOperate(user *proto.RoomUser, session *remote.Session) {
	if g.gameData.GameStatus == GameStatusNone && user.UserStatus&enums.Ready == 0 {
		g.r.UserReady(user.UserInfo.Uid, session)
	} else if g.gameData.GameStatus == PourScore && g.gameData.CurChairID == user.ChairID {
		g.onGameAbandon(user.ChairID, AbandonTypeAbandon, false, session)
	}
}

/*
 * 牌面回顾
 */
func (g *GameFrame) onGameReview(user *proto.RoomUser, session *remote.Session) {
	g.sendData(gameReviewPushData(g.ReviewRecord), []string{user.UserInfo.Uid}, session)
}

func (g *GameFrame) isUserPlaying(user *proto.RoomUser) bool {
	if user != nil && (user.UserStatus&enums.Ready > 0 || user.UserStatus&enums.Playing > 0) {
		return true
	}
	return false
}

func (g *GameFrame) userHasEnoughScore(chairID int) bool {
	user := g.getUserByChairID(chairID)
	if user == nil || !g.isUserPlaying(user) {
		return false
	}
	if !g.isUnionCreate() {
		return true
	}
	hasPour := 0
	for _, v := range g.gameData.PourScores[chairID] {
		hasPour += v
	}
	look := 1
	if g.gameData.LookCards[chairID] > 0 {
		look = 2
	}
	if look*g.gameData.CurScore+hasPour > user.UserInfo.Score {
		return false
	}
	return true
}

func (g *GameFrame) isUnionCreate() bool {
	return g.r.GetCreator().CreatorType == enums.UnionCreatorType
}

/*
 * 获取剩下的还在游戏中的玩家chairID
 */
func (g *GameFrame) getRestChairIDArray(session *remote.Session) []int {
	var restChairIDArr []int
	for _, user := range g.r.GetUsers() {
		if g.isUserPlaying(user) && !utils.Contains(g.gameData.Loser, user.ChairID) {
			restChairIDArr = append(restChairIDArr, user.ChairID)
		}
	}
	return restChairIDArr
}

func (g *GameFrame) startPourScore(session *remote.Session) {
	g.gameData.Tick = TmPourScore
	//推送游戏状态
	g.gameData.GameStatus = PourScore
	if g.pourScoreScheduleID != nil {
		g.pourScoreScheduleID.Stop()
		g.pourScoreScheduleID = nil
	}
	g.pourScoreScheduleID = tasks.NewTask("pourScoreScheduleID", 1*time.Second, func() {
		if g.r.IsDismissing() {
			return
		}
		g.gameData.Tick--
		if g.gameData.Tick <= 0 {
			g.onGameAbandon(g.gameData.CurChairID, 1, false, session)
		}
	})
	g.SendGameStatus(session)
	g.sendDataAll(GameTurnPushData(g.gameData.CurChairID, g.gameData.CurScore), session)
	chairID := g.gameData.CurChairID
	if g.startPourScoreID != nil {
		g.startPourScoreID.Stop()
		g.startPourScoreID = nil
	}
	g.startPourScoreID = time.AfterFunc(time.Second, func() {
		if g.gameData.GameStatus == PourScore &&
			g.gameData.UserTrustArray[chairID] &&
			g.gameData.CurChairID == chairID {
			g.stopPourScoreScheduleID <- struct{}{}
			g.onGameAbandon(g.gameData.CurChairID, 1, false, session)
		}
	})
}

func (g *GameFrame) delScheduleIDs() {
	if g.userTrustID != nil {
		g.userTrustID.Stop()
		g.userTrustID = nil
	}
	if g.forcePrepareID != nil {
		g.forcePrepareID.Stop()
		g.forcePrepareID = nil
	}
	if g.pourScoreScheduleID != nil {
		g.pourScoreScheduleID.Stop()
		g.pourScoreScheduleID = nil
	}
}

func (g *GameFrame) offlineUserAutoOperation(user *proto.RoomUser, session *remote.Session) {
	//TODO
}

func (g *GameFrame) startSendCards(session *remote.Session) {
	g.gameData.GameStatus = SendCards
	g.SendGameStatus(session)
	if g.sendCardsScheduleID != nil {
		g.sendCardsScheduleID.Stop()
		g.sendCardsScheduleID = nil
	}
	g.sendCardsScheduleID = time.AfterFunc(time.Duration(TmSendCards)*time.Second, func() {
		g.endSendCards(session)
	})
	g.logic.washCards()
	for i := 0; i < g.gameData.ChairCount; i++ {
		if g.IsPlayingChairID(i) {
			g.gameData.HandCards[i] = g.logic.getCards()
		}
	}
	//发牌要保密
	handCards := make([][]int, g.gameData.ChairCount)
	for index, v := range g.gameData.HandCards {
		if v != nil {
			handCards[index] = make([]int, 3)
		}
	}
	g.sendDataAll(GameSendCardsPushData(handCards), session)
	//
	////1.用户信息变更推送（金币变化） {"gold": 9958, "pushRouter": 'UpdateUserInfoPush'}
	//users := g.getAllUsers()
	//g.sendDataAll(UpdateUserInfoPushGold(user.UserInfo.Gold), session)
	////2.庄家推送 {"type":414,"data":{"bankerChairID":0},"pushRouter":"GameMessagePush"}
	//if g.gameData.CurBureau == 0 {
	//	//庄家是每次开始游戏 首次进行操作的座次
	//	g.gameData.BankerChairID = utils.Rand(len(users))
	//}
	//g.gameData.CurChairID = g.gameData.BankerChairID
	//g.sendDataAll(GameBankerPushData(g.gameData.BankerChairID), session)
	////3.局数推送{"type":411,"data":{"curBureau":6},"pushRouter":"GameMessagePush"}
	//g.gameData.CurBureau++
	//g.sendDataAll(GameBureauPushData(g.gameData.CurBureau), session)
	////4.游戏状态推送 分两步推送 第一步 推送 发牌 牌发完之后 第二步 推送下分 需要用户操作了 推送操作
	////{"type":401,"data":{"gameStatus":1,"tick":0},"pushRouter":"GameMessagePush"}
	//g.gameData.GameStatus = SendCards
	//g.sendDataAll(GameStatusPushData(g.gameData.GameStatus, 0), session)
	////5.发牌推送
	//g.sendCards(session)
	////6.下分推送
	////先推送下分状态
	//g.gameData.GameStatus = PourScore
	//g.sendDataAll(GameStatusPushData(g.gameData.GameStatus, 30), session)
	//g.gameData.CurScore = g.gameRule.AddScores[0] * g.gameRule.BaseScore
	//for _, v := range g.r.GetUsers() {
	//	g.sendData(GamePourScorePushData(v.ChairID, g.gameData.CurScore, g.gameData.CurScore, 1, 0), []string{v.UserInfo.Uid}, session)
	//}
	////7. 轮数推送
	//g.gameData.Round = 1
	//g.sendDataAll(GameRoundPushData(g.gameData.Round), session)
	////8. 操作推送
	//for _, v := range g.r.GetUsers() {
	//	//GameTurnPushData ChairID是做操作的座次号（是哪个用户在做操作）
	//	g.sendData(GameTurnPushData(g.gameData.CurChairID, g.gameData.CurScore), []string{v.UserInfo.Uid}, session)
	//}
}

func (g *GameFrame) endSendCards(session *remote.Session) {
	g.startPourScore(session)
}

func (g *GameFrame) stopSchedule() {
	for {
		select {
		case <-g.stopPourScoreScheduleID:
			if g.pourScoreScheduleID != nil {
				logs.Info("======stopSchedule:%v", g.pourScoreScheduleID.Name)
				g.pourScoreScheduleID.Stop()
				logs.Info("======stopSchedule end:%v", g.pourScoreScheduleID.Name)
				g.pourScoreScheduleID = nil
			}
		case <-g.stopForcePrepareID:
			if g.forcePrepareID != nil {
				logs.Info("======stopSchedule:%v", g.forcePrepareID.Name)
				g.forcePrepareID.Stop()
				logs.Info("======stopSchedule end:%v", g.forcePrepareID.Name)
				g.forcePrepareID = nil
			}

		}
	}
}

// endResult 结束结算
func (g *GameFrame) endResult(session *remote.Session) {
	g.resetGame(session)
	g.gameEnd(session)
}
