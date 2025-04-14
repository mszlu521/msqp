package mj

import (
	"common/logs"
	"common/tasks"
	"common/utils"
	"core/models/enums"
	"encoding/json"
	"framework/remote"
	"game/component/base"
	"game/component/mj/mp"
	"game/component/proto"
	"sort"
	"sync"
	"time"
)

type GameFrame struct {
	sync.RWMutex
	r                       base.RoomFrame
	gameRule                proto.GameRule
	gameType                GameType
	logic                   *Logic
	baseScore               int
	trustTm                 int
	userWinRecord           map[string]*UserWinRecord
	userTrustArray          []bool
	gameStarted             bool
	result                  *GameResult
	reviewRecord            []*ReviewRecord //牌面回顾记录
	testCardArray           []mp.CardID
	trustTmArray            []int
	resultRecord            []*GameResult
	scoreRecord             []int //输赢分记录
	huRecord                []int //胡牌次数记录
	gongGangRecord          []int //共杠次数记录
	anGangRecord            []int //暗杠次数记录
	maRecord                []int //中马次数
	gameStatus              GameStatus
	tick                    int
	curChairID              int
	operateRecord           []*OperateRecord
	operateArrays           [][]OperateType
	handCards               [][]mp.CardID
	turnSchedule            *tasks.Task
	scheduleOperate         []*tasks.Task
	userAutoOperateSch      *time.Timer
	isDismissed             bool
	gangChairID             int
	userRecord              []*UserRecord
	userTrustSchedule       *tasks.Task
	forcePrepareID          *tasks.Task
	bankerChairID           int
	stopTurnScheduleChan    chan struct{}
	stopForcePrepareChan    chan struct{}
	stopScheduleOperateChan chan int
	stopUserTrustSchedule   chan struct{}
}

const PlayerCount = 4

func (g *GameFrame) GetGameBureauData() any {
	var gameData [][]*BureauReview
	for _, v := range g.reviewRecord {
		var bureauReview []*BureauReview
		for _, user := range v.UserArray {
			bureauReview = append(bureauReview, &BureauReview{
				Uid:      user.Uid,
				WinScore: user.Score,
				Nickname: user.Nickname,
				Avatar:   user.Avatar,
				IsBanker: user.IsBanker,
			})
		}
		gameData = append(gameData, bureauReview)
	}
	return gameData
}

func (g *GameFrame) GetGameVideoData() any {
	if len(g.reviewRecord) <= 0 {
		return nil
	}
	lastItem := g.reviewRecord[len(g.reviewRecord)-1]
	if lastItem != nil && lastItem.Result == nil {
		g.reviewRecord = g.reviewRecord[:len(g.reviewRecord)-1]
	}
	return g.reviewRecord
}

func (g *GameFrame) OnEventRoomDismiss(reason enums.RoomDismissReason, session *remote.Session) {
	g.isDismissed = true
	g.delScheduleIDs()
	if len(g.resultRecord) == 0 {
		g.sendDataAll(GameDismissPushData(nil, nil, reason, nil), session)
		return
	}
	var userArray []*DismissUser
	var creator Creator
	for _, user := range g.r.GetUsers() {
		userArray = append(userArray, &DismissUser{
			Uid:           user.UserInfo.Uid,
			Nickname:      user.UserInfo.Nickname,
			Avatar:        user.UserInfo.Avatar,
			HuCount:       g.huRecord[user.ChairID],
			GongGangCount: g.gongGangRecord[user.ChairID],
			AnGangCount:   g.anGangRecord[user.ChairID],
			MaCount:       g.maRecord[user.ChairID],
			WinScore:      g.scoreRecord[user.ChairID],
		})
		if user.UserInfo.Uid == g.r.GetCreator().Uid {
			creator = Creator{
				Uid:      user.UserInfo.Uid,
				Nickname: user.UserInfo.Nickname,
				Avatar:   user.UserInfo.Avatar,
			}
		}
	}
	hongBaoList := g.r.GetHongBaoList()
	g.r.SendDataAll(session.GetMsg(), GameDismissPushData(userArray, &creator, reason, hongBaoList))
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
	if g.curChairID == user.ChairID {
		g.offlineUserAutoOperation(user, session)
	}
}

func (g *GameFrame) IsUserEnableLeave(chairID int) bool {
	return g.gameStatus == GameStatusNone
}

func (g *GameFrame) GetEnterGameData(session *remote.Session) any {
	//获取场景 获取游戏的数据
	//mj 打牌的时候 别人的牌 不能看到
	user := g.r.GetUsers()[session.GetUid()]
	if user == nil {
		return nil
	}
	chairID := user.ChairID
	if g.curChairID == -1 {
		//证明是第一个人进入 座次号分配给此人
		g.curChairID = chairID
	}
	gameData := &GameData{
		GameStatus:     g.gameStatus,
		GameStarted:    g.gameStarted,
		Tick:           g.tick,
		BankerChairID:  g.bankerChairID,
		CurBureau:      g.r.GetCurBureau(),
		MaxBureau:      g.r.GetMaxBureau(),
		ChairCount:     g.gameRule.MaxPlayerCount,
		UserTrustArray: g.userTrustArray,
		CurChairID:     g.curChairID,
		HandCards:      g.handCards,
		OperateArrays:  g.operateArrays,
		OperateRecord:  g.operateRecord,
		RestCardsCount: g.logic.getRestCardsCount(),
		Result:         g.result,
	}
	if g.handCards[0] != nil {
		chairCount := g.getChairCount()
		handCards := make([][]mp.CardID, chairCount)
		for i := 0; i < chairCount; i++ {
			if i == chairID {
				handCards[i] = g.handCards[i]
			} else {
				handCards[i] = make([]mp.CardID, len(g.handCards[i]))
				for j := 0; j < len(g.handCards[i]); j++ {
					handCards[i][j] = 36
				}
			}
		}
		gameData.HandCards = handCards
	}
	if g.gameStatus == GameStatusNone {
		gameData.RestCardsCount = 9*3*4 + 4
		if g.gameType == HongZhong8 {
			gameData.RestCardsCount = 9*3*4 + 8
		}
	}
	return gameData
}

func (g *GameFrame) startGame(session *remote.Session) {
	// 开始游戏
	g.recordGameUserMsg()
	g.scheduleOperate = make([]*tasks.Task, PlayerCount)
	if g.forcePrepareID != nil {
		g.stopForcePrepareChan <- struct{}{}
	}
	g.gameStarted = true
	g.trustTmArray = make([]int, PlayerCount)
	if g.gameRule.CanTrust {
		if g.userTrustSchedule != nil {
			g.stopUserTrustSchedule <- struct{}{}
		}
		g.userTrustSchedule = tasks.NewTask("userTrustSchedule", time.Second, func() {
			if g.r.IsDismissing() {
				return
			}
			if g.gameStatus == Playing {
				for i, v := range g.operateArrays {
					if v != nil && len(v) > 0 && !g.userTrustArray[i] {
						g.trustTmArray[i]++
						if g.trustTmArray[i] > g.trustTm {
							g.onGameTrust(i, session, MessageData{
								Trust: true,
							})
						}
					}
				}
			}
		})
	}
	//1 游戏状态 初始状态 推送
	g.gameStatus = Dices
	g.tick = GameStatusTmDices
	g.sendDataAll(GameStatusPushData(g.gameStatus, g.tick), session)
	//2. 庄家推送
	if g.r.GetCurBureau() == 0 {
		g.bankerChairID = 0
	} else {
		lastReview := g.reviewRecord[len(g.reviewRecord)-1]
		if lastReview != nil && lastReview.Result != nil &&
			len(lastReview.Result.WinChairIDArray) > 0 {
			g.bankerChairID = lastReview.Result.WinChairIDArray[0]
		} else {
			g.bankerChairID = (g.bankerChairID + 1) % g.getChairCount()
		}
	}
	g.sendDataAll(GameBankerPushData(g.bankerChairID), session)
	//3. 摇骰子推送
	dice1 := utils.Rand(6) + 1
	dice2 := utils.Rand(6) + 1
	g.sendDataAll(GameDicesPushData(dice1, dice2), session)
	//4. 发牌推送
	g.sendHandCards(session)

	//10 当前的局数推送+1
	g.r.SetCurBureau(g.r.GetCurBureau() + 1)
	g.sendDataAll(GameBureauPushData(g.r.GetCurBureau()), session)
}

func (g *GameFrame) GameMessageHandle(user *proto.RoomUser, session *remote.Session, msg []byte) {
	var req MessageReq
	json.Unmarshal(msg, &req)
	if req.Type == GameChatNotify {
		g.onGameChat(user, session, req.Data)
	} else if req.Type == GameTurnOperateNotify {
		g.onGameTurnOperate(user.ChairID, session, req.Data, false)
	} else if req.Type == GameGetCardNotify {
		g.onGetCard(user.ChairID, session, req.Data)
	} else if req.Type == GameTrustNotify {
		g.onGameTrust(user.ChairID, session, req.Data)
	} else if req.Type == GameReviewNotify {
		g.onGameReview(user.ChairID, session, req.Data)
	}
}

func (g *GameFrame) getAllUsers() []string {
	users := make([]string, 0)
	for _, v := range g.r.GetUsers() {
		users = append(users, v.UserInfo.Uid)
	}
	return users
}

func (g *GameFrame) sendHandCards(session *remote.Session) {
	//先洗牌 在发牌
	g.logic.washCards()
	chairCount := g.getChairCount()
	var userArray []proto.UserRoomData
	for i := 0; i < chairCount; i++ {
		g.handCards[i] = g.logic.getCards(13)
		user := g.getUserByChairID(i)
		userArray = append(userArray, proto.UserRoomData{
			Avatar:   user.UserInfo.Avatar,
			Nickname: user.UserInfo.Nickname,
			Score:    user.UserInfo.Score,
			Uid:      user.UserInfo.Uid,
			IsBanker: user.ChairID == g.bankerChairID,
		})
		//if i == 1 {
		//	g.handCards[i] = []mp.CardID{
		//		mp.Wan1, mp.Wan1, mp.Wan2, mp.Wan2, mp.Wan3, mp.Wan5, mp.Wan5, mp.Wan5,
		//		mp.Tong1, mp.Tong1, mp.Tong1, mp.Zhong, mp.Tong4,
		//	}
		//}
	}
	var cloneHandCards = make([][]mp.CardID, chairCount)
	for i := 0; i < chairCount; i++ {
		cloneHandCards[i] = make([]mp.CardID, len(g.handCards[i]))
		copy(cloneHandCards[i], g.handCards[i])
	}
	g.reviewRecord = append(g.reviewRecord, &ReviewRecord{
		RoomID:        g.r.GetId(),
		HandCards:     cloneHandCards,
		OperateRecord: g.operateRecord,
		UserArray:     userArray,
		CardsCount:    g.getCardsCount(),
		MaxBureau:     g.r.GetMaxBureau(),
		Qidui:         g.gameRule.Qidui,
	})
	for i := 0; i < chairCount; i++ {
		handCards := make([][]mp.CardID, chairCount)
		for j := 0; j < chairCount; j++ {
			if i == j {
				handCards[j] = append([]mp.CardID{}, g.handCards[i]...)
			} else {
				handCards[j] = make([]mp.CardID, len(g.handCards[j]))
				for k := range g.handCards[j] {
					handCards[j][k] = 36
				}
			}
		}
		//推送牌
		uid := g.getUserByChairID(i).UserInfo.Uid
		g.sendData(GameSendCardsPushData(handCards, i), []string{uid}, session)
	}

	//5. 剩余牌数推送
	restCardsCount := g.logic.getRestCardsCount()
	g.sendDataAll(GameRestCardsCountPushData(restCardsCount), session)
	time.AfterFunc(time.Second, func() {
		if g.isDismissed {
			return
		}
		//7. 开始游戏状态推送
		g.gameStatus = Playing
		g.tick = 0
		g.sendDataAll(GameStatusPushData(g.gameStatus, g.tick), session)
		//玩家的操作时间了
		g.setTurn(g.bankerChairID, session)
	})
}

func (g *GameFrame) getUserByChairID(chairID int) *proto.RoomUser {
	for _, v := range g.r.GetUsers() {
		if v.ChairID == chairID {
			return v
		}
	}
	return nil
}

func (g *GameFrame) setTurn(chairID int, session *remote.Session) {
	if g.logic.getRestCardsCount() <= g.gameRule.Ma {
		//流局
		g.gameEnd(session)
	} else {
		//8. 拿牌推送
		g.curChairID = chairID
		//牌不能大于14
		if len(g.handCards[g.curChairID]) == 14 {
			logs.Warn("已经拿过牌了,chairID:%d", chairID)
			return
		}
		card := g.testCardArray[chairID]
		if card > 0 && card < 36 {
			//从牌堆中 拿指定的牌
			card = g.logic.getCard(card)
			g.testCardArray[chairID] = 0
		}
		if card <= 0 || card >= 36 {
			cards := g.logic.getCards(1)
			if cards == nil || len(cards) == 0 {
				return
			}
			card = cards[0]
		}
		//获取操作列表前，先把牌放到handCards里
		g.handCards[g.curChairID] = append([]mp.CardID{}, g.handCards[g.curChairID]...) // 深拷贝
		g.handCards[g.curChairID] = append(g.handCards[g.curChairID], card)             // 追加新卡片
		//需要给所有的用户推送 这个玩家拿到了牌 给当前用户是明牌 其他人是暗牌
		operateArray := g.getMyOperateArray(session, chairID, card)
		chairCount := g.getChairCount()
		for i := 0; i < chairCount; i++ {
			if i == g.curChairID {
				user := g.getUserByChairID(g.curChairID)
				g.sendData(GameTurnPushData(g.curChairID, card, operateTm1, operateArray), []string{user.UserInfo.Uid}, session)
				g.operateArrays[g.curChairID] = operateArray
				g.operateRecord = append(g.operateRecord, &OperateRecord{
					ChairID: chairID,
					Card:    &card,
					Operate: Get,
				})
			} else {
				user := g.getUserByChairID(i)
				g.sendData(GameTurnPushData(g.curChairID, 36, operateTm1, operateArray), []string{user.UserInfo.Uid}, session)
			}
		}
		g.tick = operateTm1
		if g.turnSchedule != nil {
			g.stopTurnScheduleChan <- struct{}{}
		}
		g.turnSchedule = tasks.NewTask("turnSchedule", 1*time.Second, func() {
			if g.r.IsDismissing() {
				return
			}
			g.tick--
			if g.tick <= 0 {
				g.userAutoOperate(chairID, 0, session)
				g.stopTurnScheduleChan <- struct{}{}
			}
		})
		//9. 剩余牌数推送
		restCardsCount := g.logic.getRestCardsCount()
		g.sendDataAll(GameRestCardsCountPushData(restCardsCount), session)
		if g.userTrustArray[chairID] {
			g.userAutoOperate(chairID, 1, session)
		}
	}
}

func (g *GameFrame) getMyOperateArray(session *remote.Session, chairID int, card mp.CardID) []OperateType {
	//需要获取用户可操作的行为 ，比如 弃牌 碰牌 杠牌 胡牌等
	var operateArray = []OperateType{Qi}
	cards := g.sortCard(chairID)
	var hasGangZi bool
	for i := 3; i < len(cards); i++ {
		if cards[i] == cards[i-1] && cards[i] == cards[i-2] && cards[i] == cards[i-3] {
			hasGangZi = true
		}
	}
	if hasGangZi {
		operateArray = append(operateArray, GangZi)
	}
	var operateCount int
	for i := 0; i < len(g.operateRecord); i++ {
		if g.operateRecord[i] != nil && g.operateRecord[i].ChairID == chairID &&
			g.operateRecord[i].Operate != Get {
			operateCount++
			if g.operateRecord[i].Operate == Peng &&
				*g.operateRecord[i].Card == card {
				operateArray = append(operateArray, GangBu)
			}
		}
	}
	if g.logic.canHu(cards, -1) {
		operateArray = append(operateArray, HuZi)
	}
	if operateCount == 0 {
		//第一手出牌，4或8个红中胡牌
		maxHongZhongCount := g.getZhongCount()
		if g.logic.getHongZhongCount(g.handCards[chairID]) == maxHongZhongCount {
			operateArray = append(operateArray, HuZi)
		}
	}
	return operateArray
}

func (g *GameFrame) sortCard(chairID int) []mp.CardID {
	cards := make([]mp.CardID, len(g.handCards[chairID]))
	copy(cards, g.handCards[chairID])
	sort.Slice(cards, func(i, j int) bool {
		return cards[i] < cards[j]
	})
	return cards
}

func (g *GameFrame) onGameChat(user *proto.RoomUser, session *remote.Session, data MessageData) {
	g.sendDataAll(GameChatPushData(user.ChairID, data.Type, data.Msg, data.RecipientID), session)
}

func (g *GameFrame) onGameTurnOperate(chairID int, session *remote.Session, data MessageData, auto bool) {
	if !auto {
		g.trustTmArray[chairID] = 0
	}
	lastOperate := g.operateRecord[len(g.operateRecord)-1]
	if lastOperate != nil &&
		lastOperate.ChairID == chairID &&
		lastOperate.Operate == Qi &&
		data.Operate == Qi {
		logs.Warn("已经操作过，不能再操作")
		return
	}
	operateArray := g.operateArrays[chairID]
	if operateArray == nil || IndexOf(operateArray, data.Operate) == -1 {
		logs.Warn("操作错误")
		return
	}
	if g.turnSchedule != nil {
		g.stopTurnScheduleChan <- struct{}{}
	}
	if data.Card <= 0 {
		if IndexOf([]OperateType{Peng, GangChi, HuChi, Guo}, data.Operate) != -1 {
			data.Card = *g.operateRecord[len(g.operateRecord)-1].Card
		}
		if GangBu == data.Operate {
			data.Card = g.handCards[chairID][len(g.handCards[chairID])-1]
		} else if GangZi == data.Operate {
			cards := g.sortCard(chairID)
			for i := 3; i < len(cards); i++ {
				if cards[i] == cards[i-1] &&
					cards[i] == cards[i-2] &&
					cards[i] == cards[i-3] {
					data.Card = cards[i]
				}
			}
			if data.Card <= 0 {
				data.Card = cards[len(cards)-1]
			}
		} else if HuZi == data.Operate {
			data.Card = g.handCards[chairID][len(g.handCards[chairID])-1]
		}
	}
	count := g.logic.getCardCount(g.handCards[chairID], data.Card)
	//碰杠胡过
	if g.curChairID != chairID {
		if data.Operate == Peng {
			if count < 2 {
				return
			} else {
				if g.scheduleOperate[chairID] != nil {
					g.stopScheduleOperateChan <- chairID
				}
				g.sendDataAll(GameTurnOperatePushData(chairID, data.Card, data.Operate, true), session)
				g.handCards[chairID] = g.delCardFromArray(g.handCards[chairID], data.Card, 2)
				g.operateRecord = append(g.operateRecord,
					&OperateRecord{chairID, &data.Card, data.Operate})
				g.operateArrays[chairID] = []OperateType{Qi}
				g.sendDataAll(GameTurnPushData(chairID, -1, operateTm1, g.operateArrays[chairID]), session)
				g.curChairID = chairID
				if g.userTrustArray[chairID] {
					g.userAutoOperate(chairID, 1, session)
				}
			}
		} else if data.Operate == GangChi {
			if count < 3 {
				logs.Warn("不能杠...")
				return
			} else {
				if g.scheduleOperate[chairID] != nil {
					g.stopScheduleOperateChan <- chairID
				}
				g.sendDataAll(GameTurnOperatePushData(chairID, data.Card, data.Operate, true), session)
				g.handCards[chairID] = g.delCardFromArray(g.handCards[chairID], data.Card, 3)
				g.operateRecord = append(g.operateRecord,
					&OperateRecord{chairID, &data.Card, data.Operate})
				g.setTurn(chairID, session)
			}
		} else if data.Operate == Guo {

			if g.scheduleOperate[chairID] != nil {
				g.stopScheduleOperateChan <- chairID
			}
			uid := g.getUserByChairID(chairID).Uid
			g.sendData(GameTurnOperatePushData(chairID, data.Card, data.Operate, true), []string{uid}, session)
			g.operateRecord = append(g.operateRecord, &OperateRecord{chairID, &data.Card, data.Operate})
			g.operateArrays[chairID] = nil
			hasOtherOperator := false
			for _, v := range g.scheduleOperate {
				if v != nil {
					hasOtherOperator = true
				}
			}
			if !hasOtherOperator {
				nextChairID := (g.curChairID + 1) % g.getChairCount()
				g.setTurn(nextChairID, session)
			}
		}
	} else {
		//胡杠弃
		if data.Operate == HuZi {

			zhongCount := g.getZhongCount()
			//isFirstTurn := g.getIsFirstTurn(chairID)
			if g.logic.canHu(g.handCards[chairID], -1) ||
				(g.getIsFirstTurn(chairID) && g.getZhongCardCount(g.handCards[chairID]) == zhongCount) {
				g.sendDataAll(GameTurnOperatePushData(chairID, data.Card, data.Operate, true), session)
				g.operateRecord = append(g.operateRecord, &OperateRecord{chairID, &data.Card, data.Operate})
				g.operateArrays[chairID] = nil
				g.gameEnd(session)
			} else {
				return
			}
		} else if data.Operate == GangBu {

			canBu := false
			for _, v := range g.operateRecord {
				if v.ChairID == chairID && v.Operate == Peng {
					canBu = true
					break
				}
			}
			if !canBu {
				return
			} else {
				g.gangChairID = chairID
				g.operateArrays[chairID] = nil
				hasChiHu := false
				chairCount := g.getChairCount()
				for i := 0; i < chairCount; i++ {
					if i == chairID || !g.logic.canHu(g.handCards[i], data.Card) {
						continue
					}
					g.sendDataAll(GameTurnOperatePushData(i, data.Card, HuChi, true), session)
					g.handCards[i] = append([]mp.CardID{}, g.handCards[i]...)
					g.handCards[i] = append(g.handCards[i], data.Card)
					g.operateRecord = append(g.operateRecord, &OperateRecord{i, &data.Card, HuChi})
					g.operateArrays[i] = nil
					hasChiHu = true
				}
				if !hasChiHu {
					g.sendDataAll(GameTurnOperatePushData(chairID, data.Card, data.Operate, true), session)
					g.handCards[chairID] = g.delCardFromArray(g.handCards[chairID], data.Card, 1)
					g.operateRecord = append(g.operateRecord, &OperateRecord{chairID, &data.Card, data.Operate})
					g.gangChairID = -1
					g.setTurn(chairID, session)
				} else {
					// 有吃胡
					g.handCards[chairID] = g.delCardFromArray(g.handCards[chairID], data.Card, 1)
					g.gameEnd(session)
				}
			}
		} else if data.Operate == GangZi {
			if count < 4 {
				return
			} else {
				g.sendDataAll(GameTurnOperatePushData(chairID, data.Card, data.Operate, true), session)
				g.handCards[chairID] = g.delCardFromArray(g.handCards[chairID], data.Card, 4)
				g.operateRecord = append(g.operateRecord, &OperateRecord{chairID, &data.Card, data.Operate})
				g.setTurn(chairID, session)
			}
		} else if data.Operate == Qi {

			index := IndexOf(g.handCards[chairID], data.Card)
			if index == -1 {
				logs.Error("%d没有这张牌:%v", chairID, data.Card)
				return
			}
			g.sendDataAll(GameTurnOperatePushData(chairID, data.Card, data.Operate, true), session)
			g.handCards[chairID] = g.delCardFromArray(g.handCards[chairID], data.Card, 1)
			g.operateRecord = append(g.operateRecord, &OperateRecord{chairID, &data.Card, data.Operate})
			g.operateArrays[chairID] = nil
			g.nextTurn(data.Card, session)
		}
	}
}

func (g *GameFrame) nextTurn(lastCard mp.CardID, session *remote.Session) {
	//在下一个用户摸牌之前，需要判断 其他玩家 是否有人可以碰 杠 胡 等等操作
	var hasOtherOperator bool
	chairCount := g.getChairCount()
	if lastCard > 0 && lastCard < 36 {
		for i := 0; i < chairCount; i++ {
			if i == g.curChairID {
				continue
			}
			operateArray := g.logic.getOperateArray(g.handCards[i], lastCard)
			huIndex := IndexOf(operateArray, HuChi)
			if huIndex != -1 {
				if g.gangChairID < 0 {
					operateArray = Splice(operateArray, huIndex, 1)
				}
			}
			if len(operateArray) > 1 {
				hasOtherOperator = true
				user := g.getUserByChairID(i)
				g.sendData(GameTurnPushData(i, lastCard, operateTm2, operateArray), []string{user.UserInfo.Uid}, session)
				g.operateArrays[i] = operateArray
				tick := operateTm2
				if g.scheduleOperate[i] != nil {
					g.stopScheduleOperateChan <- i
				}
				g.scheduleOperate[i] = tasks.NewTask("scheduleOperate", time.Second, func() {
					if g.r.IsDismissing() {
						return
					}
					tick--
					if tick <= 0 {
						g.stopScheduleOperateChan <- i
						g.sendData(GameTurnOperatePushData(i, -1, Guo, false), []string{user.UserInfo.Uid}, session)
						g.operateRecord = append(g.operateRecord, &OperateRecord{i, nil, Guo})
						g.operateArrays[i] = nil
						canNext := true
						for _, item := range g.scheduleOperate {
							if item != nil {
								canNext = false
							}
						}
						if canNext {
							nextChairID := (g.curChairID + 1) % chairCount
							g.setTurn(nextChairID, session)
						}
					}
				})
				if g.userTrustArray[i] {
					g.userAutoOperate(i, 0, session)
				}
			}
		}
	}
	if !hasOtherOperator {
		nextChairID := (g.curChairID + 1) % chairCount
		g.setTurn(nextChairID, session)
	}
}

func (g *GameFrame) gameEnd(session *remote.Session) {
	g.gameStatus = Result
	if g.userTrustSchedule != nil {
		g.stopUserTrustSchedule <- struct{}{}
	}
	g.tick = 0
	g.sendDataAll(GameStatusPushData(g.gameStatus, g.tick), session)
	var lastOperate *OperateRecord
	var winChairIDArray []int
	winChairID := -1
	for i := 3; i >= 1; i-- {
		if len(g.operateRecord)-i >= 0 {
			lastOperate = g.operateRecord[len(g.operateRecord)-i]
			if lastOperate.Operate == HuZi ||
				lastOperate.Operate == HuChi {
				winChairID = lastOperate.ChairID
			}
			if winChairID != -1 {
				winChairIDArray = append(winChairIDArray, winChairID)
			}
		}
	}
	chairCount := g.getChairCount()
	scores := make([]int, chairCount)
	var maWinCount int
	var myMaCards []MyMaCard
	if len(winChairIDArray) > 0 {
		maWinCards := g.logic.getMaCardsByChairID()
		isChunniunai := false
		if g.gameRule.Chunniunai && len(winChairIDArray) == 1 &&
			IndexOf(g.handCards[winChairIDArray[0]], Zhong) == -1 {
			isChunniunai = true
		}
		maCount := g.gameRule.Ma
		if isChunniunai {
			maCount = maCount + 1
		}
		maCards := g.logic.getCards(maCount)
		//一码全中
		if g.gameRule.Ma == 1 {
			maWinCount = 10
			if maCards[0] != Zhong {
				maWinCount = int(maCards[0]) % 10
				myMaCards = append(myMaCards, MyMaCard{maCards[0], true})
			}
		} else {
			for i := 0; i < len(maCards); i++ {
				if IndexOf(maWinCards, maCards[i]) != -1 {
					maWinCount++
					myMaCards = append(myMaCards, MyMaCard{maCards[i], true})
				} else {
					myMaCards = append(myMaCards, MyMaCard{maCards[i], false})
				}
			}
		}
	}
	if lastOperate.Operate == HuZi {
		for i := 0; i < chairCount; i++ {
			if IndexOf(winChairIDArray, i) != -1 {
				continue
			}
			score := -(2 + maWinCount*2) * g.baseScore * len(winChairIDArray)
			if g.gameRule.Ma == 1 {
				score = -(2 + maWinCount) * g.baseScore * len(winChairIDArray)
			}
			user := g.getUserByChairID(i)
			if g.isUnionCreate() && score+user.UserInfo.Score < 0 {
				//不够赔付
				score = -user.UserInfo.Score
			}
			scores[i] += score
			for _, v := range winChairIDArray {
				scores[v] -= score / len(winChairIDArray)
			}
		}
	} else if lastOperate.Operate == HuChi {
		//抢杠
		if g.gangChairID > -1 {
			score := (-2 - maWinCount*2) * g.baseScore * (chairCount - 1) * len(winChairIDArray)
			if g.gameRule.Ma == 1 {
				score = (-2 - maWinCount) * g.baseScore * (chairCount - 1) * len(winChairIDArray)
			}
			user := g.getUserByChairID(g.gangChairID)
			if g.isUnionCreate() && score+user.UserInfo.Score < 0 {
				//不够赔付
				score = -user.UserInfo.Score
			}
			scores[g.gangChairID] += score
			for _, v := range winChairIDArray {
				scores[v] -= score / len(winChairIDArray)
			}
		} else {
			loseChairID := g.operateRecord[len(g.operateRecord)-2].ChairID
			score := (-1 - maWinCount*2) * g.baseScore
			if g.gameRule.Ma == 1 {
				score = (-1 - maWinCount) * g.baseScore
			}
			user := g.getUserByChairID(g.gangChairID)
			if g.isUnionCreate() && score+user.UserInfo.Score < 0 {
				//不够赔付
				score = -user.UserInfo.Score
			}
			scores[loseChairID] = score
			scores[winChairIDArray[0]] -= score
		}
	}
	if len(winChairIDArray) > 0 {
		for i := 0; i < len(g.operateRecord); i++ {
			if g.operateRecord[i].Operate == GangZi {
				//暗杠
				if g.isUnionCreate() {
					for j := 0; j < chairCount; j++ {
						if j == g.operateRecord[i].ChairID {
							continue
						}
						score := -2 * g.baseScore
						user := g.getUserByChairID(j)
						if score+scores[j]+user.UserInfo.Score < 0 {
							score = -user.UserInfo.Score - scores[j]
						}
						scores[j] += score
						scores[g.operateRecord[i].ChairID] -= score
					}
				} else {
					for j := 0; j < chairCount; j++ {
						rate := -2
						if j == g.operateRecord[i].ChairID {
							rate = 2 * (chairCount - 1)
						}
						scores[j] += g.baseScore * rate
					}
				}
			} else if g.operateRecord[i].Operate == GangChi {
				//接杠
				if g.isUnionCreate() {
					score := -3 * g.baseScore
					user := g.getUserByChairID(g.operateRecord[i-1].ChairID)
					if score+scores[g.operateRecord[i-1].ChairID]+user.UserInfo.Score < 0 {
						score = -user.UserInfo.Score - scores[g.operateRecord[i-1].ChairID]
					}
					scores[g.operateRecord[i-1].ChairID] += score
					scores[g.operateRecord[i].ChairID] += -score
				} else {
					scores[g.operateRecord[i-1].ChairID] -= 3 * g.baseScore
					scores[g.operateRecord[i].ChairID] += 3 * g.baseScore
				}
			} else if g.operateRecord[i].Operate == GangBu {
				if g.isUnionCreate() {
					for j := 0; j < chairCount; j++ {
						if j == g.operateRecord[i].ChairID {
							continue
						}
						score := -1 * g.baseScore
						user := g.getUserByChairID(j)
						if score+scores[j]+user.UserInfo.Score < 0 {
							score = -user.UserInfo.Score - scores[j]
						}
						scores[j] += score
						scores[g.operateRecord[i].ChairID] -= score
					}
				} else {
					for j := 0; j < chairCount; j++ {
						rate := -1
						if j == g.operateRecord[i].ChairID {
							rate = 1 * (chairCount - 1)
						}
						scores[j] += g.baseScore * rate
					}
				}
			}
		}
		for i := 0; i < len(scores); i++ {
			g.scoreRecord[i] += scores[i]
		}
		for _, v := range winChairIDArray {
			g.huRecord[v] += 1
			g.maRecord[v] += maWinCount
		}
		for _, item := range g.operateRecord {
			if item.Operate == GangChi ||
				item.Operate == GangBu {
				g.gongGangRecord[item.ChairID] += 1
			} else if item.Operate == GangZi {
				g.anGangRecord[item.ChairID] += 1
			}
		}
	}
	var fangGangArray []int
	for i := 0; i < len(g.operateRecord); i++ {
		item := g.operateRecord[i]
		if item.Operate == GangChi {
			fangGangArray = append(fangGangArray, g.operateRecord[i-1].ChairID)
		}
	}
	result := &GameResult{
		Scores:          scores,
		HandCards:       g.handCards,
		MyMaCards:       myMaCards,
		WinChairIDArray: winChairIDArray,
		FangGangArray:   fangGangArray,
		RestCards:       g.logic.getRestCards(),
		HuType:          lastOperate.Operate,
		GangChairID:     g.gangChairID,
	}
	g.reviewRecord[len(g.reviewRecord)-1].Result = result
	g.resultRecord = append(g.resultRecord, result)
	g.sendDataAll(GameResultPushData(result), session)
	g.result = result
	var endData []*proto.EndData
	for i := 0; i < chairCount; i++ {
		user := g.getUserByChairID(i)
		if user != nil {
			endData = append(endData, &proto.EndData{
				Uid:   user.UserInfo.Uid,
				Score: scores[i],
			})
		}
	}
	time.AfterFunc(3*time.Second, func() {
		if g.isDismissed {
			return
		}
		g.r.ConcludeGame(endData, session)
		g.resetGame(session)
	})
	tick := 33
	if g.r.GetCurBureau() != g.r.GetMaxBureau() {
		if g.forcePrepareID != nil {
			g.stopForcePrepareChan <- struct{}{}
		}
		g.forcePrepareID = tasks.NewTask("forcePrepareID", 1*time.Second, func() {
			if g.r.IsDismissing() {
				return
			}
			tick--
			if tick <= 0 {
				if g.gameStatus == GameStatusNone {
					for _, user := range g.r.GetUsers() {
						if user.UserStatus&enums.Ready > 0 {
							//手动准备过，倒计时清零
							g.trustTmArray[user.ChairID] = 0
						}
						if user.UserStatus&enums.Ready == 0 &&
							g.gameStatus == GameStatusNone && !g.r.GetGameStarted() {
							g.r.UserReady(user.UserInfo.Uid, session)
						}
					}
				}
				g.stopForcePrepareChan <- struct{}{}
			}
		})
	}
}

func (g *GameFrame) resetGame(session *remote.Session) {
	g.gameStatus = GameStatusNone
	g.tick = 0
	g.sendDataAll(GameStatusPushData(g.gameStatus, g.tick), session)
	restCardsCount := 9*3*4 + 4
	if g.gameType != HongZhong4 {
		restCardsCount = 9*3*4 + 8
	}
	g.sendDataAll(GameRestCardsCountPushData(restCardsCount), session)
	g.curChairID = -1
	g.gangChairID = -1
	g.operateArrays = make([][]OperateType, PlayerCount)
	g.operateRecord = make([]*OperateRecord, 0)
	g.handCards = make([][]mp.CardID, PlayerCount)
	g.result = nil
}

func (g *GameFrame) onGetCard(chairID int, session *remote.Session, data MessageData) {
	g.testCardArray[chairID] = data.Card
}

func (g *GameFrame) userAutoOperate(chairID int, delayTime int, session *remote.Session) {
	if g.userAutoOperateSch != nil {
		g.userAutoOperateSch = nil
	}
	g.userAutoOperateSch = time.AfterFunc(time.Duration(delayTime)*time.Second, func() {
		if !g.isDismissed {
			operateArray := g.operateArrays[chairID]
			if len(operateArray) > 0 {
				if IndexOf(operateArray, Qi) != -1 {
					g.onGameTurnOperate(chairID,
						session,
						MessageData{
							Operate: Qi,
							Card:    g.handCards[chairID][len(g.handCards[chairID])-1],
						}, true)
				} else if IndexOf(operateArray, Guo) != -1 {
					g.onGameTurnOperate(chairID,
						session,
						MessageData{
							Operate: Guo,
						}, true)
				}
			}
			if g.gameStatus == GameStatusNone {
				user := g.getUserByChairID(chairID)
				if user != nil && (user.UserStatus&enums.Ready) == 0 {
					g.r.UserReady(user.UserInfo.Uid, session)
				}
			}
		}
	})
	//indexOf := IndexOf(operateArray, Qi)
	//user := g.getUserByChairID(chairID)
	//if indexOf != -1 {
	//	//操作有弃牌
	//	////1. 向所有人通告 当前用户做了什么操作
	//	//g.sendData(GameTurnOperatePushData(chairID, card, Qi, true), session)
	//	////弃牌了 牌需要删除
	//	//g.HandCards[chairID] = g.delCards(g.HandCards[chairID], card, 1)
	//	//g.OperateRecord = append(g.OperateRecord, OperateRecord{chairID, card, Qi})
	//	//g.operateArrays[chairID] = nil
	//	//g.nextTurn(card, session)
	//	g.onGameTurnOperate(user, session, MessageData{Operate: Qi, Card: card})
	//} else if IndexOf(operateArray, Guo) != -1 {
	//	g.onGameTurnOperate(user, session, MessageData{Operate: Guo, Card: 0})
	//	////操作过
	//	//g.sendData(GameTurnOperatePushData(chairID, card, Guo, true), session)
	//	//g.OperateRecord = append(g.OperateRecord, OperateRecord{chairID, card, Guo})
	//}
}

func (g *GameFrame) onGameTrust(chairID int, session *remote.Session, data MessageData) {
	g.trustTmArray[chairID] = 0
	g.userTrustArray[chairID] = data.Trust
	uid := g.getUserByChairID(chairID).Uid
	g.sendData(GameTrustPushData(chairID, data.Trust), []string{uid}, session)
	if data.Trust {
		g.userAutoOperate(chairID, 0, session)
	}
}

func (g *GameFrame) onGameReview(chairID int, session *remote.Session, data MessageData) {
	var reviewRecord []*ReviewRecord
	for _, v := range g.reviewRecord {
		if v.Result != nil {
			reviewRecord = append(reviewRecord, v)
		}
	}
	uid := g.getUserByChairID(chairID).Uid
	g.sendData(GameReviewPushData(reviewRecord), []string{uid}, session)
}

func (g *GameFrame) offlineUserAutoOperation(user *proto.RoomUser, session *remote.Session) {
	//自动操作

}

func (g *GameFrame) delCardFromArray(cards []mp.CardID, card mp.CardID, times int) []mp.CardID {
	g.Lock()
	defer g.Unlock()
	for i := len(cards) - 1; i >= 0 && times > 0; i-- {
		if cards[i] == card {
			// 删除切片中的元素
			cards = append(cards[:i], cards[i+1:]...)
			times--
		}
	}
	return cards
}

func (g *GameFrame) getChairCount() int {
	return len(g.r.GetUsers())
}

func (g *GameFrame) getZhongCount() int {
	zhongCount := 4
	if g.gameType == HongZhong8 {
		zhongCount = 8
	}
	return zhongCount
}
func (g *GameFrame) getZhongCardCount(arr []mp.CardID) int {
	count := 0
	for _, v := range arr {
		if v == Zhong {
			count++
		}
	}
	return count
}
func (g *GameFrame) getIsFirstTurn(chairID int) bool {
	for _, v := range g.operateRecord {
		if v.ChairID == chairID && v.Operate != Get {
			return false
		}
	}
	return true
}

func (g *GameFrame) recordGameUserMsg() {
	if g.userRecord == nil {
		g.userRecord = make([]*UserRecord, 0)
		chairCount := g.getChairCount()
		for i := 0; i < chairCount; i++ {
			user := g.getUserByChairID(i)
			if user == nil {
				continue
			}
			g.userRecord = append(g.userRecord, &UserRecord{
				ChairID:  i,
				Uid:      user.Uid,
				Nickname: user.UserInfo.Nickname,
				Avatar:   user.UserInfo.Avatar,
			})
		}
	}
}

func (g *GameFrame) delScheduleIDs() {
	for i, v := range g.scheduleOperate {
		if v != nil {
			g.stopScheduleOperateChan <- i
		}
	}
	if g.userTrustSchedule != nil {
		g.stopUserTrustSchedule <- struct{}{}
	}
	if g.turnSchedule != nil {
		g.stopTurnScheduleChan <- struct{}{}
	}
	if g.forcePrepareID != nil {
		g.stopForcePrepareChan <- struct{}{}
	}
}

func (g *GameFrame) getCardsCount() int {
	if g.gameType == HongZhong4 {
		return 9*12 + 4
	}
	return 9*12 + 8
}

func (g *GameFrame) stopSchedule() {
	for {
		select {
		case <-g.stopTurnScheduleChan:
			if g.turnSchedule != nil {
				g.turnSchedule.Stop()
				g.turnSchedule = nil
			}
		case <-g.stopForcePrepareChan:
			if g.forcePrepareID != nil {
				g.stopForcePrepareChan <- struct{}{}
			}
		case <-g.stopUserTrustSchedule:
			if g.userTrustSchedule != nil {
				g.userTrustSchedule.Stop()
				g.userTrustSchedule = nil
			}
		case chairID := <-g.stopScheduleOperateChan:
			if g.scheduleOperate[chairID] != nil {
				g.scheduleOperate[chairID].Stop()
				g.scheduleOperate[chairID] = nil
			}

		}
	}
}

func (g *GameFrame) isUnionCreate() bool {
	return g.r.GetCreator().CreatorType == enums.UnionCreatorType
}

func NewGameFrame(rule proto.GameRule, r base.RoomFrame, session *remote.Session) *GameFrame {
	//gameData := initGameData(rule, playerCount)
	baseScore := 1
	if rule.BaseScore > 0 {
		baseScore = rule.BaseScore
	}
	g := &GameFrame{
		r:        r,
		gameRule: rule,
		gameType: GameType(rule.GameFrameType),
		//gameData:       gameData,
		logic:                   NewLogic(GameType(rule.GameFrameType), rule.Qidui),
		baseScore:               baseScore,
		trustTm:                 rule.TrustTm,
		userWinRecord:           map[string]*UserWinRecord{},
		reviewRecord:            make([]*ReviewRecord, 0),
		userTrustArray:          make([]bool, PlayerCount),
		gameStarted:             false,
		testCardArray:           make([]mp.CardID, PlayerCount), //设定测试牌
		trustTmArray:            make([]int, PlayerCount),
		resultRecord:            make([]*GameResult, 0),
		scoreRecord:             make([]int, PlayerCount),
		huRecord:                make([]int, PlayerCount),
		gongGangRecord:          make([]int, PlayerCount),
		anGangRecord:            make([]int, PlayerCount),
		maRecord:                make([]int, PlayerCount),
		scheduleOperate:         make([]*tasks.Task, PlayerCount),
		bankerChairID:           -1,
		stopTurnScheduleChan:    make(chan struct{}, 1),
		stopForcePrepareChan:    make(chan struct{}, 1),
		stopScheduleOperateChan: make(chan int, 1),
		stopUserTrustSchedule:   make(chan struct{}, 1),
	}
	g.resetGame(session)
	go g.stopSchedule()
	return g
}

//func initGameData(rule proto.GameRule, playerCount int) *GameData {
//	g := new(GameData)
//	g.chairCount = rule.MaxPlayerCount
//	g.HandCards = make([][]mp.CardID, g.chairCount)
//	g.gameStatus = GameStatusNone
//	g.GameStarted = false
//	g.OperateRecord = make([]OperateRecord, 0)
//	g.operateArrays = make([][]OperateType, g.chairCount)
//	g.curChairID = -1
//	g.bankerChairID = -1
//	g.RestCardsCount = 9*3*4 + 4
//	g.UserTrustArray = make([]bool, playerCount)
//	if rule.GameFrameType == HongZhong8 {
//		g.RestCardsCount = 9*3*4 + 8
//	}
//	return g
//}
