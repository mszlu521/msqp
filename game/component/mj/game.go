package mj

import (
	"common/logs"
	"common/utils"
	"encoding/json"
	"framework/remote"
	"game/component/base"
	"game/component/mj/mp"
	"game/component/proto"
	"github.com/jinzhu/copier"
	"sync"
	"time"
)

type GameFrame struct {
	sync.RWMutex
	r             base.RoomFrame
	gameRule      proto.GameRule
	gameData      *GameData
	logic         *Logic
	testCardArray []mp.CardID
	turnSchedule  []*time.Timer
}

func (g *GameFrame) OnEventRoomDismiss(reason proto.RoomDismissReason, session *remote.Session) {
	var userArray []*DismissUser
	var creator Creator
	for _, v := range g.r.GetUsers() {
		userArray = append(userArray, &DismissUser{
			Uid:      v.UserInfo.Uid,
			Nickname: v.UserInfo.Nickname,
			Avatar:   v.UserInfo.Avatar,
		})
		if v.UserInfo.Uid == g.r.GetCreator().Uid {
			creator = Creator{
				Uid:      v.UserInfo.Uid,
				Nickname: v.UserInfo.Nickname,
				Avatar:   v.UserInfo.Avatar,
			}
		}
	}
	g.r.SendDataAll(session.GetMsg(), GameDismissPushData(userArray, &creator, reason, nil))
}

func (g *GameFrame) OnEventGameStart(user *proto.RoomUser, session *remote.Session) {
	g.startGame(session, user)
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

func (g *GameFrame) IsUserEnableLeave(chairID int) bool {
	return g.gameData.GameStatus == GameStatusNone
}

func (g *GameFrame) GetGameData(session *remote.Session) any {
	//获取场景 获取游戏的数据
	//mj 打牌的时候 别人的牌 不能看到
	user := g.r.GetUsers()[session.GetUid()]
	if user == nil {
		return nil
	}
	chairID := user.ChairID
	var gameData GameData
	copier.CopyWithOption(&gameData, g.gameData, copier.Option{IgnoreEmpty: true, DeepCopy: true})
	handCards := make([][]mp.CardID, g.gameData.ChairCount)
	for i := range gameData.HandCards {
		if i == chairID {
			handCards[i] = gameData.HandCards[i]
		} else {
			//每张牌 置为 36
			handCards[i] = make([]mp.CardID, len(g.gameData.HandCards[i]))
			for j := range g.gameData.HandCards[i] {
				handCards[i][j] = 36
			}
		}
	}
	gameData.HandCards = handCards
	if g.gameData.GameStatus == GameStatusNone {
		gameData.RestCardsCount = 9*3*4 + 4
		if g.gameRule.GameFrameType == HongZhong8 {
			gameData.RestCardsCount = 9*3*4 + 8
		}
	}
	return gameData
}

func (g *GameFrame) startGame(session *remote.Session, user *proto.RoomUser) {
	// 开始游戏
	//1 游戏状态 初始状态 推送
	g.gameData.GameStarted = true
	g.gameData.GameStatus = Dices
	g.sendDataAll(GameStatusPushData(g.gameData.GameStatus, GameStatusTmDices), session)
	//2. 庄家推送
	if g.gameData.CurBureau == 0 {
		g.gameData.BankerChairID = 0
	} else {
		//TODO win是庄家
	}
	//BD ai提示
	g.sendDataAll(GameBankerPushData(g.gameData.BankerChairID), session)
	//3. 摇骰子推送
	dice1 := utils.Rand(6) + 1
	dice2 := utils.Rand(6) + 1
	g.sendDataAll(GameDicesPushData(dice1, dice2), session)
	//4. 发牌推送
	g.sendHandCards(session)

	//10 当前的局数推送+1
	g.gameData.CurBureau++
	g.sendDataAll(GameBureauPushData(g.gameData.CurBureau), session)
}

func (g *GameFrame) GameMessageHandle(user *proto.RoomUser, session *remote.Session, msg []byte) {
	var req MessageReq
	json.Unmarshal(msg, &req)
	if req.Type == GameChatNotify {
		g.onGameChat(user, session, req.Data)
	} else if req.Type == GameTurnOperateNotify {
		g.onGameTurnOperate(user, session, req.Data)
	} else if req.Type == GameGetCardNotify {
		g.onGetCard(user, session, req.Data)
	} else if req.Type == GameTrustNotify {
		g.onGameTrust(user, session, req.Data)
	} else if req.Type == GameReviewNotify {
		g.onGameReview(user, session, req.Data)
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
	for i := 0; i < g.gameData.ChairCount; i++ {
		g.gameData.HandCards[i] = g.logic.getCards(13)
		if i == 1 {
			g.gameData.HandCards[i] = []mp.CardID{
				mp.Wan1, mp.Wan1, mp.Wan2, mp.Wan2, mp.Wan3, mp.Wan5, mp.Wan5, mp.Wan5,
				mp.Tong1, mp.Tong1, mp.Tong1, mp.Zhong, mp.Tong4,
			}
		}
	}
	for i := 0; i < g.gameData.ChairCount; i++ {
		handCards := make([][]mp.CardID, g.gameData.ChairCount)
		for j := 0; j < g.gameData.ChairCount; j++ {
			if i == j {
				handCards[i] = g.gameData.HandCards[i]
			} else {
				handCards[j] = make([]mp.CardID, len(g.gameData.HandCards[j]))
				for k := range g.gameData.HandCards[j] {
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
		//7. 开始游戏状态推送
		g.gameData.GameStatus = Playing
		g.sendDataAll(GameStatusPushData(g.gameData.GameStatus, GameStatusTmPlay), session)
		//玩家的操作时间了
		g.setTurn(g.gameData.BankerChairID, session)
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
	//8. 拿牌推送
	g.gameData.CurChairID = chairID
	//牌不能大于14
	if len(g.gameData.HandCards[chairID]) >= 14 {
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

	g.gameData.HandCards[chairID] = append(g.gameData.HandCards[chairID], card)
	//需要给所有的用户推送 这个玩家拿到了牌 给当前用户是明牌 其他人是暗牌
	operateArray := g.getMyOperateArray(session, chairID, card)
	for i := 0; i < g.gameData.ChairCount; i++ {
		uid := g.getUserByChairID(i).UserInfo.Uid
		if i == chairID {
			g.gameTurn([]string{uid}, chairID, card, operateArray, session)
			//g.sendDataUsers([]string{uid}, GameTurnPushData(chairID, card, OperateTime, operateArray), session)
			g.gameData.OperateArrays[i] = operateArray
			g.gameData.OperateRecord = append(g.gameData.OperateRecord, OperateRecord{
				ChairID: i,
				Card:    card,
				Operate: Get,
			})
			g.turnScheduleExecute(chairID, card, operateArray, session)
		} else {
			g.gameTurn([]string{uid}, i, 36, operateArray, session)
			//暗牌
			//g.sendDataUsers([]string{uid}, GameTurnPushData(chairID, 36, OperateTime, operateArray), session)
		}
		//只能进行一次触发

	}
	//9. 剩余牌数推送
	restCardsCount := g.logic.getRestCardsCount()
	g.sendDataAll(GameRestCardsCountPushData(restCardsCount), session)
}

func (g *GameFrame) gameTurn(uids []string, chairID int, card mp.CardID, operateArray []OperateType, session *remote.Session) {
	//触发定时
	g.gameData.Tick = OperateTime
	if uids == nil {
		g.sendDataAll(GameTurnPushData(chairID, card, g.gameData.Tick, operateArray), session)
	} else {
		g.sendData(GameTurnPushData(chairID, card, g.gameData.Tick, operateArray), uids, session)
	}
}
func (g *GameFrame) turnScheduleExecute(chairID int, card mp.CardID, operateArray []OperateType, session *remote.Session) {
	if g.turnSchedule[chairID] != nil {
		g.turnSchedule[chairID].Stop()
	}
	g.turnSchedule[chairID] = time.AfterFunc(time.Second, func() {
		if g.gameData.Tick <= 0 {
			//取消定时
			if g.turnSchedule[chairID] != nil {
				g.turnSchedule[chairID].Stop()
			}
			g.userAutoOperate(chairID, card, operateArray, session)
		} else {
			g.gameData.Tick--
			g.turnSchedule[chairID].Reset(time.Second)
		}
	})
}
func (g *GameFrame) getMyOperateArray(session *remote.Session, chairID int, card mp.CardID) []OperateType {
	//需要获取用户可操作的行为 ，比如 弃牌 碰牌 杠牌 胡牌等
	//TODO
	var operateArray = []OperateType{Qi}
	if g.logic.canHu(g.gameData.HandCards[chairID], -1) {
		operateArray = append(operateArray, HuZi)
	}
	cardCount := 0
	for _, v := range g.gameData.HandCards[chairID] {
		if v == card {
			cardCount++
		}
	}
	if cardCount == 4 {
		//杠 自
		operateArray = append(operateArray, GangZi)
	}
	//已经碰了 这时候又来了一张牌 可以和碰的牌 组成 杠 补杠
	//已经拿牌之后 判断操作
	for _, v := range g.gameData.OperateRecord {
		if v.ChairID == chairID && v.Operate == Peng && v.Card == card {
			operateArray = append(operateArray, GangBu)
		}
	}
	return operateArray
}

func (g *GameFrame) onGameChat(user *proto.RoomUser, session *remote.Session, data MessageData) {
	g.sendDataAll(GameChatPushData(user.ChairID, data.Type, data.Msg, data.RecipientID), session)
}

func (g *GameFrame) onGameTurnOperate(user *proto.RoomUser, session *remote.Session, data MessageData) {
	if user == nil {
		return
	}
	if g.turnSchedule != nil && g.turnSchedule[user.ChairID] != nil {
		g.turnSchedule[user.ChairID].Stop()
	}
	if data.Operate == Qi {
		//1. 向所有人通告 当前用户做了什么操作
		g.sendDataAll(GameTurnOperatePushData(user.ChairID, data.Card, data.Operate, true), session)
		//弃牌了 牌需要删除
		g.gameData.HandCards[user.ChairID] = g.delCards(g.gameData.HandCards[user.ChairID], data.Card, 1)
		g.gameData.OperateRecord = append(g.gameData.OperateRecord, OperateRecord{user.ChairID, data.Card, data.Operate})
		g.gameData.OperateArrays[user.ChairID] = nil
		g.nextTurn(data.Card, session)
	} else if data.Operate == Guo {
		//1. 当前用户的操作是否成功 告诉所有人
		g.sendDataAll(GameTurnOperatePushData(user.ChairID, data.Card, data.Operate, true), session)
		g.gameData.OperateRecord = append(g.gameData.OperateRecord, OperateRecord{user.ChairID, data.Card, data.Operate})
		//TODO 如果牌14 先去弃牌 然后才能做其他的
		//继续操作
		g.setTurn(user.ChairID, session)
	} else if data.Operate == Peng {
		//碰一张牌 出一张牌
		//1. 当前用户的操作是否成功 告诉所有人
		if data.Card == 0 {
			length := len(g.gameData.OperateRecord)
			if length == 0 {
				//没有记录 出错了
				logs.Error("用户碰操作，但是没有上一个操作记录")
			} else {
				data.Card = g.gameData.OperateRecord[length-1].Card
			}
		}
		g.sendDataAll(GameTurnOperatePushData(user.ChairID, data.Card, data.Operate, true), session)
		//g.gameData.HandCards[user.ChairID] = append(g.gameData.HandCards[user.ChairID], data.Card) //有14张
		//碰相当于 损失了2张牌 当用户重新进入房间时  加载gameData handCards 14 放在左下角
		g.gameData.HandCards[user.ChairID] = g.delCards(g.gameData.HandCards[user.ChairID], data.Card, 2)
		g.gameData.OperateRecord = append(g.gameData.OperateRecord, OperateRecord{user.ChairID, data.Card, data.Operate})
		g.gameData.OperateArrays[user.ChairID] = []OperateType{Qi}
		g.sendDataAll(GameTurnPushData(user.ChairID, 0, OperateTime, g.gameData.OperateArrays[user.ChairID]), session)
		//2. 让用户开始出牌
		g.gameData.CurChairID = user.ChairID
	} else if data.Operate == GangChi {
		//碰一张牌 出一张牌
		//1. 当前用户的操作是否成功 告诉所有人
		if data.Card == 0 {
			length := len(g.gameData.OperateRecord)
			if length == 0 {
				//没有记录 出错了
				logs.Error("用户吃杠操作，但是没有上一个操作记录")
			} else {
				data.Card = g.gameData.OperateRecord[length-1].Card
			}
		}
		g.sendDataAll(GameTurnOperatePushData(user.ChairID, data.Card, data.Operate, true), session)
		//g.gameData.HandCards[user.ChairID] = append(g.gameData.HandCards[user.ChairID], data.Card) //有14张
		//杠相当于 损失了3张牌 当用户重新进入房间时  加载gameData handCards 14 放在左下角
		g.gameData.HandCards[user.ChairID] = g.delCards(g.gameData.HandCards[user.ChairID], data.Card, 3)
		g.gameData.OperateRecord = append(g.gameData.OperateRecord, OperateRecord{user.ChairID, data.Card, data.Operate})
		g.gameData.OperateArrays[user.ChairID] = []OperateType{Qi}
		g.sendDataAll(GameTurnPushData(user.ChairID, 0, OperateTime, g.gameData.OperateArrays[user.ChairID]), session)
		//2. 让用户开始出牌
		g.gameData.CurChairID = user.ChairID
	} else if data.Operate == HuChi {
		if data.Card == 0 {
			length := len(g.gameData.OperateRecord)
			if length == 0 {
				//没有记录 出错了
				logs.Error("用户吃胡操作，但是没有上一个操作记录")
			} else {
				data.Card = g.gameData.OperateRecord[length-1].Card
			}
		}
		g.sendDataAll(GameTurnOperatePushData(user.ChairID, data.Card, data.Operate, true), session)
		g.gameData.HandCards[user.ChairID] = append(g.gameData.HandCards[user.ChairID], data.Card)
		g.gameData.OperateRecord = append(g.gameData.OperateRecord, OperateRecord{user.ChairID, data.Card, data.Operate})
		g.gameData.OperateArrays[user.ChairID] = nil
		//2. 让用户开始出牌
		g.gameData.CurChairID = user.ChairID
		g.gameEnd(session)
	} else if data.Operate == HuZi {
		//一定是自己摸牌操作
		g.sendDataAll(GameTurnOperatePushData(user.ChairID, data.Card, data.Operate, true), session)
		//g.gameData.HandCards[user.ChairID] = append(g.gameData.HandCards[user.ChairID], data.Card)
		g.gameData.OperateRecord = append(g.gameData.OperateRecord, OperateRecord{user.ChairID, data.Card, data.Operate})
		g.gameData.OperateArrays[user.ChairID] = nil
		//2. 让用户开始出牌
		g.gameData.CurChairID = user.ChairID
		g.gameEnd(session)
	} else if data.Operate == GangZi {
		//1. 当前用户的操作是否成功 告诉所有人
		card := g.gameData.HandCards[user.ChairID][len(g.gameData.HandCards[user.ChairID])-1]
		//自摸 是暗杠 其他玩家 不应该看到杠的牌
		for i := 0; i < g.gameData.ChairCount; i++ {
			if i == user.ChairID {
				g.sendData(GameTurnOperatePushData(user.ChairID, card, data.Operate, true), []string{g.getUserByChairID(i).UserInfo.Uid}, session)
			} else {
				g.sendData(GameTurnOperatePushData(user.ChairID, data.Card, data.Operate, true), []string{g.getUserByChairID(i).UserInfo.Uid}, session)
			}
		}
		g.gameData.HandCards[user.ChairID] = g.delCards(g.gameData.HandCards[user.ChairID], card, 4)

		g.gameData.OperateRecord = append(g.gameData.OperateRecord, OperateRecord{user.ChairID, card, data.Operate})
		//继续操作
		g.setTurn(user.ChairID, session)
	} else if data.Operate == GangBu {
		//1. 自摸 杠补
		if g.gameData.CurChairID == user.ChairID {
			card := g.gameData.HandCards[user.ChairID][len(g.gameData.HandCards[user.ChairID])-1]
			for i := 0; i < g.gameData.ChairCount; i++ {
				if i == user.ChairID {
					g.sendData(GameTurnOperatePushData(user.ChairID, card, data.Operate, true), []string{g.getUserByChairID(i).UserInfo.Uid}, session)
				} else {
					g.sendData(GameTurnOperatePushData(user.ChairID, data.Card, data.Operate, true), []string{g.getUserByChairID(i).UserInfo.Uid}, session)
				}
			}
			g.gameData.HandCards[user.ChairID] = g.delCards(g.gameData.HandCards[user.ChairID], card, 1)

			g.gameData.OperateRecord = append(g.gameData.OperateRecord, OperateRecord{user.ChairID, card, data.Operate})
			//继续操作
			g.setTurn(user.ChairID, session)
		} else {
			//2. 吃牌 杠补  特殊的 有些mj实现中 是不允许这种情况的
			//if data.Card == 0 {
			//	length := len(g.gameData.OperateRecord)
			//	if length == 0 {
			//		//没有记录 出错了
			//		logs.Error("用户吃杠操作，但是没有上一个操作记录")
			//	} else {
			//		data.Card = g.gameData.OperateRecord[length-1].Card
			//	}
			//}
			//g.sendData(GameTurnOperatePushData(user.ChairID, data.Card, data.Operate, true), session)
			////g.gameData.HandCards[user.ChairID] = append(g.gameData.HandCards[user.ChairID], data.Card) //有14张
			////杠相当于 损失了3张牌 当用户重新进入房间时  加载gameData handCards 14 放在左下角
			////g.gameData.HandCards[user.ChairID] = g.delCards(g.gameData.HandCards[user.ChairID], data.Card, 3)
			//g.gameData.OperateRecord = append(g.gameData.OperateRecord, OperateRecord{user.ChairID, data.Card, data.Operate})
			//g.setTurn(user.ChairID, session)
			//g.gameData.OperateArrays[user.ChairID] = []OperateType{Qi}
			//g.sendData(GameTurnPushData(user.ChairID, 0, OperateTime, g.gameData.OperateArrays[user.ChairID]), session)
			////2. 让用户开始出牌
			//g.gameData.CurChairID = user.ChairID
		}

	}

}

func (g *GameFrame) delCards(cards []mp.CardID, card mp.CardID, times int) []mp.CardID {
	g.Lock()
	defer g.Unlock()
	newCards := make([]mp.CardID, 0)
	//循环删除多个元素 是有越界风险的
	count := 0
	for _, v := range cards {
		if v != card {
			newCards = append(newCards, v)
		} else {
			if count == times {
				newCards = append(newCards, v)
			} else {
				count++
				continue
			}
		}
	}
	return newCards
}

func (g *GameFrame) nextTurn(lastCard mp.CardID, session *remote.Session) {
	//在下一个用户摸牌之前，需要判断 其他玩家 是否有人可以碰 杠 胡 等等操作
	hasOtherOperate := false
	if lastCard > 0 && lastCard < 36 {
		for i := 0; i < g.gameData.ChairCount; i++ {
			if i == g.gameData.CurChairID {
				continue
			}
			operateArray := g.logic.getOperateArray(g.gameData.HandCards[i], lastCard)
			//for _, v := range g.gameData.OperateRecord {
			//	if v.ChairID == i && v.Operate == Peng && v.Card == lastCard {
			//		//可以补杠
			//		operateArray = append(operateArray, GangBu)
			//	}
			//}
			if len(operateArray) > 0 {
				//有用户可以做一些操作
				hasOtherOperate = true
				g.gameData.Tick = OperateTime
				g.sendDataAll(GameTurnPushData(i, lastCard, OperateTime, operateArray), session)
				g.gameData.OperateArrays[i] = operateArray
				g.turnScheduleExecute(i, 0, operateArray, session)
			}
		}
	}
	if !hasOtherOperate {
		//简单的直接让下一个用户进行摸牌
		nextTurnID := (g.gameData.CurChairID + 1) % g.gameData.ChairCount
		g.setTurn(nextTurnID, session)
	}
}

func (g *GameFrame) gameEnd(session *remote.Session) {
	g.gameData.GameStatus = Result
	g.sendDataAll(GameStatusPushData(g.gameData.GameStatus, 0), session)
	scores := make([]int, g.gameData.ChairCount)
	//结算推送
	//for i := 0; i < g.gameData.ChairCount; i++ {
	//
	//}
	l := len(g.gameData.OperateRecord)
	if l <= 0 {
		logs.Error("没有操作记录，不可能游戏结束，请检查")
		return
	}
	lastOperateRecord := g.gameData.OperateRecord[l-1]
	if lastOperateRecord.Operate != HuChi && lastOperateRecord.Operate != HuZi {
		logs.Error("最后一次操作，不是胡牌，不可能游戏结束，请检查")
		return
	}
	result := GameResult{
		Scores:          scores,
		HandCards:       g.gameData.HandCards,
		RestCards:       g.logic.getRestCards(),
		WinChairIDArray: []int{lastOperateRecord.ChairID},
		HuType:          lastOperateRecord.Operate,
		MyMaCards:       []MyMaCard{},
		FangGangArray:   []int{},
	}
	g.gameData.Result = &result
	g.sendDataAll(GameResultPushData(result), session)

	time.AfterFunc(3*time.Second, func() {
		g.r.EndGame(session)
		g.resetGame(session)
	})
	//倒计时30秒 如果用户未操作 自动准备或者踢出房间
}

func (g *GameFrame) resetGame(session *remote.Session) {
	g.gameData.GameStarted = false
	g.gameData.GameStatus = GameStatusNone
	g.sendDataAll(GameStatusPushData(g.gameData.GameStatus, 0), session)
	g.sendDataAll(GameRestCardsCountPushData(g.logic.getRestCardsCount()), session)
	for i := 0; i < g.gameData.ChairCount; i++ {
		g.gameData.HandCards[i] = nil
		g.gameData.OperateArrays[i] = nil
	}
	g.gameData.OperateRecord = make([]OperateRecord, 0)
	g.gameData.CurChairID = -1
	g.gameData.Result = nil
}

func (g *GameFrame) onGetCard(user *proto.RoomUser, session *remote.Session, data MessageData) {
	g.testCardArray[user.ChairID] = data.Card
}

func (g *GameFrame) userAutoOperate(chairID int, card mp.CardID, operateArray []OperateType, session *remote.Session) {
	indexOf := IndexOf(operateArray, Qi)
	user := g.getUserByChairID(chairID)
	if indexOf != -1 {
		//操作有弃牌
		////1. 向所有人通告 当前用户做了什么操作
		//g.sendData(GameTurnOperatePushData(chairID, card, Qi, true), session)
		////弃牌了 牌需要删除
		//g.gameData.HandCards[chairID] = g.delCards(g.gameData.HandCards[chairID], card, 1)
		//g.gameData.OperateRecord = append(g.gameData.OperateRecord, OperateRecord{chairID, card, Qi})
		//g.gameData.OperateArrays[chairID] = nil
		//g.nextTurn(card, session)
		g.onGameTurnOperate(user, session, MessageData{Operate: Qi, Card: card})
	} else if IndexOf(operateArray, Guo) != -1 {
		g.onGameTurnOperate(user, session, MessageData{Operate: Guo, Card: 0})
		////操作过
		//g.sendData(GameTurnOperatePushData(chairID, card, Guo, true), session)
		//g.gameData.OperateRecord = append(g.gameData.OperateRecord, OperateRecord{chairID, card, Guo})
	}
}

func (g *GameFrame) onGameTrust(user *proto.RoomUser, session *remote.Session, data MessageData) {

}

func (g *GameFrame) onGameReview(user *proto.RoomUser, session *remote.Session, data MessageData) {

}

func (g *GameFrame) offlineUserAutoOperation(user *proto.RoomUser, session *remote.Session) {
	//自动操作

}

func NewGameFrame(rule proto.GameRule, r base.RoomFrame) *GameFrame {
	gameData := initGameData(rule)
	return &GameFrame{
		r:             r,
		gameRule:      rule,
		gameData:      gameData,
		logic:         NewLogic(GameType(rule.GameFrameType), rule.Qidui),
		testCardArray: make([]mp.CardID, gameData.ChairCount),
		turnSchedule:  make([]*time.Timer, gameData.ChairCount),
	}
}

func initGameData(rule proto.GameRule) *GameData {
	g := new(GameData)
	g.ChairCount = rule.MaxPlayerCount
	g.HandCards = make([][]mp.CardID, g.ChairCount)
	g.GameStatus = GameStatusNone
	g.OperateRecord = make([]OperateRecord, 0)
	g.OperateArrays = make([][]OperateType, g.ChairCount)
	g.CurChairID = 0
	g.RestCardsCount = 9*3*4 + 4
	if rule.GameFrameType == HongZhong8 {
		g.RestCardsCount = 9*3*4 + 8
	}
	return g
}
