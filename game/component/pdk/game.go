package pdk

import (
	"core/models/enums"
	"encoding/json"
	"framework/remote"
	"game/component/base"
	"game/component/proto"
	"math/rand"
	"sort"
	"time"
)

type GameFrame struct {
	r             base.RoomFrame
	rule          proto.GameRule
	data          *GameData
	hands         [][]int
	lastCards     []int
	passCount     int
	allCards      [][]int
	played        [][]int
	turnRecords   []preTurnRecord
	previousTurn  []preTurnRecord
	bureauRecords []any
	turnTimer     *time.Timer
	turnTimeout   time.Duration
	firstTimeout  time.Duration
}

type preTurnRecord struct {
	Avatar string `json:"avatar"`
	Name   string `json:"name"`
	Cards  []int  `json:"cards,omitempty"`
}

func NewGameFrame(rule proto.GameRule, r base.RoomFrame, _ *remote.Session) *GameFrame {
	count := rule.MaxPlayerCount
	if count < 2 || count > 3 {
		count = 3
	}
	g := &GameFrame{r: r, rule: rule, data: &GameData{
		GameStatus: GameStatusNone, CurChairID: -1, TurnWinerChairID: -1,
		FirstChairID: -1, AllUserCardCountArr: make([][]int, count),
		IsFirstTurnArray: make([]bool, count), UserBombTimes: make([]int, count),
		UserTrustArray: make([]bool, count),
	}, hands: make([][]int, count), allCards: make([][]int, count), turnTimeout: 15 * time.Second, firstTimeout: 30 * time.Second}
	return g
}

func (g *GameFrame) GetGameBureauData() any                                { return g.bureauRecords }
func (g *GameFrame) GetGameVideoData() any                                 { return g.bureauRecords }
func (g *GameFrame) IsUserEnableLeave(chairID int) bool                    { return g.data.GameStatus == GameStatusNone }
func (g *GameFrame) OnEventUserEntry(_ *proto.RoomUser, _ *remote.Session) {}
func (g *GameFrame) OnEventUserOffLine(user *proto.RoomUser, session *remote.Session) {
	if g.isPlayingUser(user) && user.ChairID == g.data.CurChairID {
		g.pass(user, session, true)
	}
}
func (g *GameFrame) OnEventRoomDismiss(reason enums.RoomDismissReason, session *remote.Session) {
	if g.turnTimer != nil {
		g.turnTimer.Stop()
		g.turnTimer = nil
	}
	if len(g.bureauRecords) == 0 {
		g.r.SendDataAll(session.GetMsg(), gameDismissPush(nil, nil, reason, nil))
		return
	}
	users := make([]*dismissUser, 0, len(g.r.GetUsers()))
	var creator *dismissCreator
	roomUsers := make([]*proto.RoomUser, 0, len(g.r.GetUsers()))
	for _, roomUser := range g.r.GetUsers() {
		if roomUser != nil && roomUser.UserInfo != nil && roomUser.ChairID >= 0 && roomUser.ChairID < len(g.hands) {
			roomUsers = append(roomUsers, roomUser)
		}
	}
	sort.Slice(roomUsers, func(i, j int) bool { return roomUsers[i].ChairID < roomUsers[j].ChairID })
	for _, roomUser := range roomUsers {
		if roomUser.UserInfo == nil {
			continue
		}
		item := &dismissUser{
			Uid: roomUser.UserInfo.Uid, Nickname: roomUser.UserInfo.Nickname,
			Avatar: roomUser.UserInfo.Avatar, WinScore: roomUser.WinScore,
		}
		for _, value := range g.bureauRecords {
			record, ok := value.(map[string]any)
			if !ok {
				continue
			}
			score := intAt(record["winArr"], roomUser.ChairID)
			if score > item.SingleMaxScore {
				item.SingleMaxScore = score
			}
			if score > 0 {
				item.WinCount++
			} else if score < 0 {
				item.LoseCount++
			}
			item.BoomCount += intAt(record["bombArr"], roomUser.ChairID)
		}
		users = append(users, item)
		if roomCreator := g.r.GetCreator(); roomCreator != nil && roomUser.UserInfo.Uid == roomCreator.Uid {
			creator = &dismissCreator{Uid: item.Uid, Nickname: item.Nickname, Avatar: item.Avatar}
		}
	}
	g.r.SendDataAll(session.GetMsg(), gameDismissPush(users, creator, reason, g.r.GetHongBaoList()))
}

func intAt(value any, index int) int {
	if index < 0 {
		return 0
	}
	switch values := value.(type) {
	case []int:
		if index < len(values) {
			return values[index]
		}
	case []any:
		if index < len(values) {
			switch number := values[index].(type) {
			case int:
				return number
			case float64:
				return int(number)
			}
		}
	}
	return 0
}

func (g *GameFrame) GetEnterGameData(session *remote.Session) any {
	copyData := *g.data
	copyData.SelfCardArr = nil
	if session != nil {
		if user := g.r.GetUsers()[session.GetUid()]; user != nil && user.ChairID < len(g.hands) {
			copyData.SelfCardArr = append([]int(nil), g.hands[user.ChairID]...)
		}
	}
	copyData.TurnCardDataArr = append([]int(nil), g.lastCards...)
	copyData.AllUserCardCountArr = g.cardCounts()
	copyData.EnablePass = g.enablePassForChair(g.data.CurChairID)
	return &copyData
}

func (g *GameFrame) OnEventGameStart(user *proto.RoomUser, session *remote.Session) {
	if g.data.GameStarted {
		return
	}
	users := g.playingUsers()
	if len(users) < 2 {
		return
	}
	deck := deck(g.rule.GameFrameType)
	rand.New(rand.NewSource(time.Now().UnixNano())).Shuffle(len(deck), func(i, j int) {
		deck[i], deck[j] = deck[j], deck[i]
	})
	g.hands, g.allCards = make([][]int, len(g.hands)), make([][]int, len(g.hands))
	g.played = make([][]int, len(g.hands))
	g.turnRecords = nil
	g.previousTurn = nil
	g.lastCards = nil
	g.passCount = 0
	g.data.EnablePass = false
	for i := range g.data.UserBombTimes {
		g.data.UserBombTimes[i] = 0
		if g.rule.XiaojuTrust && g.data.UserTrustArray[i] {
			g.data.UserTrustArray[i] = false
			g.sendAll(map[string]any{"type": GameTrustPush, "data": map[string]any{"chairID": i, "trust": false}, "pushRouter": "GameMessagePush"}, session)
		}
	}
	cardCount := 16
	if g.rule.GameFrameType == 2 {
		cardCount = 15
	}
	for i, roomUser := range users {
		start := i * cardCount
		end := start + cardCount
		if end > len(deck) {
			end = len(deck)
		}
		g.hands[roomUser.ChairID] = SortCards(deck[start:end])
		g.allCards[roomUser.ChairID] = append([]int(nil), g.hands[roomUser.ChairID]...)
	}
	g.data.GameStarted = true
	g.r.SetCurBureau(g.r.GetCurBureau() + 1)
	g.data.CurBureau = g.r.GetCurBureau()
	g.data.GameStatus = GameStatusOutCard
	g.data.FirstChairID = firstChair(g.hands, users)
	g.data.CurChairID, g.data.TurnWinerChairID = g.data.FirstChairID, g.data.FirstChairID
	for i := range g.data.IsFirstTurnArray {
		g.data.IsFirstTurnArray[i] = true
	}
	for _, roomUser := range users {
		g.send(gameStartPush(g.data.CurChairID, g.data.CurBureau, g.hands[roomUser.ChairID], g.cardCounts()), []string{roomUser.UserInfo.Uid}, session)
	}
	g.scheduleTurn(session)
}

func (g *GameFrame) GameMessageHandle(user *proto.RoomUser, session *remote.Session, msg []byte) {
	if user == nil || !g.data.GameStarted {
		return
	}
	var req MessageReq
	if json.Unmarshal(msg, &req) != nil {
		return
	}
	if req.Type != GameChatNotify && req.Type != GamePreTurnCardsNotify && !g.isPlayingUser(user) {
		return
	}
	switch req.Type {
	case GameUserOutCardNotify:
		g.out(user, req.Data.OutCardArr, session)
	case GameUserPassNotify:
		g.pass(user, session, false)
	case GameTrustNotify:
		if !g.rule.CanTrust {
			return
		}
		g.data.UserTrustArray[user.ChairID] = req.Data.Trust
		g.sendAll(map[string]any{"type": GameTrustPush, "data": map[string]any{"chairID": user.ChairID, "trust": req.Data.Trust}, "pushRouter": "GameMessagePush"}, session)
	case GameChatNotify:
		g.sendAll(map[string]any{"type": GameChatPush, "data": map[string]any{"chairID": user.ChairID, "type": req.Data.ChatType, "msg": req.Data.Msg.Value(), "recipientID": req.Data.RecipientID}, "pushRouter": "GameMessagePush"}, session)
	case GamePreTurnCardsNotify:
		g.send(userData(g.previousTurn), []string{user.UserInfo.Uid}, session)
	}
}

func (g *GameFrame) isPlayingUser(user *proto.RoomUser) bool {
	return user != nil && user.ChairID >= 0 && user.ChairID < len(g.hands) && user.UserStatus&enums.Playing > 0
}

func (g *GameFrame) out(user *proto.RoomUser, cards []int, session *remote.Session) {
	if user.ChairID != g.data.CurChairID || !containsCards(g.hands[user.ChairID], cards) {
		return
	}
	if g.rule.Heitao3 && g.data.CurBureau == 1 && user.ChairID == g.data.FirstChairID && g.data.IsFirstTurnArray[user.ChairID] {
		if required, ok := firstRequiredCard(g.hands[user.ChairID]); ok && !containsCard(cards, required) {
			return
		}
	}
	rule := Rule{ThreeABomb: g.rule.ThreeABomb, FourTakeTwo: g.rule.FourTakeTwo, FourTakeThree: g.rule.FourTakeThree}
	cardType := Type(cards, rule)
	if len(g.hands[user.ChairID]) == len(cards) {
		cardType = RestType(cards, rule)
	}
	if cardType == Error {
		return
	}
	if len(g.lastCards) > 0 && !Compare(g.lastCards, cards, rule, len(g.hands[user.ChairID]) == len(cards), g.rule.Baiwei) {
		return
	}
	g.hands[user.ChairID] = removeCards(g.hands[user.ChairID], cards)
	g.played[user.ChairID] = append(g.played[user.ChairID], cards...)
	g.turnRecords = append(g.turnRecords, newPreTurnRecord(user, cards))
	if cardType == Bomb {
		g.data.UserBombTimes[user.ChairID]++
	}
	g.lastCards, g.data.TurnWinerChairID, g.passCount = append([]int(nil), cards...), user.ChairID, 0
	g.data.IsFirstTurnArray[user.ChairID] = false
	if len(g.hands[user.ChairID]) == 0 {
		g.finish(user, cards, session)
		return
	}
	g.nextChair()
	g.pushOut(user, cards, session)
	g.scheduleTurn(session)
}

func (g *GameFrame) pass(user *proto.RoomUser, session *remote.Session, forced bool) {
	if user.ChairID != g.data.CurChairID {
		return
	}
	if canAutoPassOrPlay(forced) {
		g.autoTurn(user, session)
		return
	}
	if len(g.lastCards) == 0 {
		return
	}
	if g.rule.Bichu && g.enablePassForChair(user.ChairID) == false {
		return
	}
	g.passCount++
	g.turnRecords = append(g.turnRecords, newPreTurnRecord(user, nil))
	newTurn := g.passCount >= g.playingCount()-1
	if newTurn {
		g.previousTurn = clonePreTurnRecords(g.turnRecords)
		g.turnRecords = nil
		g.data.CurChairID, g.passCount, g.lastCards = g.data.TurnWinerChairID, 0, nil
	} else {
		g.nextChair()
	}
	g.data.IsFirstTurnArray[user.ChairID] = false
	g.sendAll(gamePassPush(user.ChairID, g.data.CurChairID, !newTurn, newTurn, g.hands[user.ChairID]), session)
	g.scheduleTurn(session)
}

func (g *GameFrame) autoTurn(user *proto.RoomUser, session *remote.Session) {
	hand := g.hands[user.ChairID]
	if len(hand) == 0 {
		return
	}
	if len(g.lastCards) == 0 {
		card := hand[len(hand)-1]
		if g.rule.Heitao3 && g.data.CurBureau == 1 && user.ChairID == g.data.FirstChairID &&
			g.data.IsFirstTurnArray[user.ChairID] {
			if required, ok := firstRequiredCard(hand); ok {
				card = required
			}
		}
		g.out(user, []int{card}, session)
		return
	}
	if g.rule.Bichu {
		rule := Rule{ThreeABomb: g.rule.ThreeABomb, FourTakeTwo: g.rule.FourTakeTwo, FourTakeThree: g.rule.FourTakeThree}
		if cards := g.findBeat(hand, g.lastCards, rule); len(cards) > 0 {
			g.out(user, cards, session)
			return
		}
	}
	g.pass(user, session, false)
}

func (g *GameFrame) finish(winner *proto.RoomUser, cards []int, session *remote.Session) {
	if g.turnTimer != nil {
		g.turnTimer.Stop()
		g.turnTimer = nil
	}
	g.data.GameStatus, g.data.GameStarted = GameStatusEnd, false
	ids := make([]string, len(g.hands))
	for uid, roomUser := range g.r.GetUsers() {
		if roomUser.ChairID < len(ids) {
			ids[roomUser.ChairID] = uid
		}
	}
	remaining := make([][]int, len(g.hands))
	handGroups := make([][][]int, len(g.hands))
	for i := range g.hands {
		remaining[i] = append([]int(nil), g.hands[i]...)
		handGroups[i] = cardGroups(g.hands[i])
	}
	baseScore := g.rule.BaseScore
	if baseScore <= 0 {
		baseScore = 1
	}
	hasChuntian := true
	for i := range g.played {
		if i != winner.ChairID && len(g.played[i]) > 0 {
			hasChuntian = false
			break
		}
	}
	winArr := make([]int, len(g.hands))
	for i := range remaining {
		if i == winner.ChairID {
			continue
		}
		multiplier := 1
		if hasChuntian {
			multiplier *= 2
		}
		if g.rule.Hongtao10 && containsCard(append(append([]int(nil), remaining[i]...), g.played[i]...), 0x2a) {
			multiplier *= 2
		}
		winArr[i] = -len(remaining[i]) * baseScore * multiplier
		winArr[winner.ChairID] -= winArr[i]
	}
	resultPush := gameResultPush(g.allCards, winner.ChairID, ids, handGroups, winArr, append([]int(nil), g.data.UserBombTimes...), hasChuntian)
	g.sendAll(resultPush, session)
	if pushMap, ok := resultPush.(map[string]any); ok {
		if resultData, ok := pushMap["data"].(map[string]any); ok {
			g.bureauRecords = append(g.bureauRecords, resultData)
		}
	}
	result := make([]*proto.EndData, 0, g.playingCount())
	for _, roomUser := range g.playingUsers() {
		score := winArr[roomUser.ChairID]
		result = append(result, &proto.EndData{Uid: roomUser.UserInfo.Uid, Score: score})
	}
	g.r.ConcludeGame(result, session)
}

func cardGroups(cards []int) [][]int {
	groups := make([][]int, 0, (len(cards)+9)/10)
	for len(cards) > 0 {
		count := len(cards)
		if count > 10 {
			count = 10
		}
		groups = append(groups, append([]int(nil), cards[:count]...))
		cards = cards[count:]
	}
	return groups
}

func containsCard(cards []int, target int) bool {
	for _, card := range cards {
		if card == target {
			return true
		}
	}
	return false
}

func (g *GameFrame) pushOut(user *proto.RoomUser, cards []int, session *remote.Session) {
	for _, roomUser := range g.playingUsers() {
		hand := []int(nil)
		if roomUser.ChairID == user.ChairID {
			hand = g.hands[roomUser.ChairID]
		}
		g.send(gameOutCardPush(user.ChairID, g.data.CurChairID, cards, g.hands[user.ChairID], g.enablePassForChair(g.data.CurChairID), hand), []string{roomUser.UserInfo.Uid}, session)
	}
}

func (g *GameFrame) nextChair() {
	for i := 1; i <= len(g.hands); i++ {
		chair := (g.data.CurChairID + i) % len(g.hands)
		if len(g.hands[chair]) > 0 {
			g.data.CurChairID = chair
			return
		}
	}
}

func (g *GameFrame) scheduleTurn(session *remote.Session) {
	if g.turnTimer != nil {
		g.turnTimer.Stop()
	}
	if !g.data.GameStarted || g.data.CurChairID < 0 {
		return
	}
	chairID := g.data.CurChairID
	timeout := g.timeoutForChair(chairID)
	g.turnTimer = time.AfterFunc(timeout, func() {
		g.r.RunGameAction(func() {
			if g.data.GameStarted && g.data.CurChairID == chairID {
				if user := g.userByChair(chairID); user != nil {
					g.pass(user, session, true)
				}
			}
		})
	})
}

func (g *GameFrame) timeoutForChair(chairID int) time.Duration {
	if chairID >= 0 && chairID < len(g.data.IsFirstTurnArray) && g.data.IsFirstTurnArray[chairID] {
		if g.firstTimeout > 0 {
			return g.firstTimeout
		}
		return 30 * time.Second
	}
	if g.turnTimeout > 0 {
		return g.turnTimeout
	}
	return 15 * time.Second
}

func (g *GameFrame) userByChair(chairID int) *proto.RoomUser {
	for _, user := range g.r.GetUsers() {
		if user.ChairID == chairID {
			return user
		}
	}
	return nil
}

func (g *GameFrame) enablePassForChair(chairID int) bool {
	if len(g.lastCards) == 0 || chairID < 0 || chairID >= len(g.hands) {
		return false
	}
	if !g.rule.Bichu {
		return true
	}
	rule := Rule{ThreeABomb: g.rule.ThreeABomb, FourTakeTwo: g.rule.FourTakeTwo, FourTakeThree: g.rule.FourTakeThree}
	return !g.canBeat(g.hands[chairID], g.lastCards, rule)
}

func (g *GameFrame) canBeat(hand, previous []int, rule Rule) bool {
	return len(g.findBeat(hand, previous, rule)) > 0
}

func (g *GameFrame) findBeat(hand, previous []int, rule Rule) []int {
	if len(previous) == 0 {
		return nil
	}
	if result := g.firstBeatOfLength(hand, previous, rule, len(previous)); len(result) > 0 {
		return result
	}
	if Type(previous, rule) != Bomb {
		if rule.ThreeABomb {
			if result := g.firstBeatOfLength(hand, previous, rule, 3); len(result) > 0 {
				return result
			}
		}
		return g.firstBeatOfLength(hand, previous, rule, 4)
	}
	return nil
}

func (g *GameFrame) firstBeatOfLength(hand, previous []int, rule Rule, length int) []int {
	if len(hand) < length {
		return nil
	}
	chosen := make([]int, 0, length)
	var search func(int) []int
	search = func(start int) []int {
		if len(chosen) == length {
			if Compare(previous, chosen, rule, len(hand) == len(chosen), g.rule.Baiwei) {
				return append([]int(nil), chosen...)
			}
			return nil
		}
		for i := start; i <= len(hand)-(length-len(chosen)); i++ {
			chosen = append(chosen, hand[i])
			if result := search(i + 1); len(result) > 0 {
				return result
			}
			chosen = chosen[:len(chosen)-1]
		}
		return nil
	}
	return search(0)
}
func (g *GameFrame) playingCount() int { return len(g.playingUsers()) }
func (g *GameFrame) playingUsers() []*proto.RoomUser {
	users := make([]*proto.RoomUser, 0)
	for _, user := range g.r.GetUsers() {
		if user != nil && user.ChairID >= 0 && user.ChairID < len(g.hands) && user.UserInfo != nil && user.UserStatus&enums.Playing > 0 {
			users = append(users, user)
		}
	}
	sort.Slice(users, func(i, j int) bool { return users[i].ChairID < users[j].ChairID })
	return users
}
func (g *GameFrame) cardCounts() [][]int {
	result := make([][]int, len(g.hands))
	for i, hand := range g.hands {
		result[i] = make([]int, len(hand))
		for j := range result[i] {
			result[i][j] = 1
		}
	}
	return result
}
func (g *GameFrame) send(data any, users []string, session *remote.Session) {
	g.r.SendData(session.GetMsg(), users, data)
}
func (g *GameFrame) sendAll(data any, session *remote.Session) {
	g.r.SendDataAll(session.GetMsg(), data)
}

func deck(frame int) []int {
	// Match the client's 48-card deck: three aces, one two, and 3-K in all suits.
	result := []int{0x01, 0x02, 0x11, 0x21}
	for _, suit := range []int{0, 1, 2, 3} {
		for value := 3; value <= 13; value++ {
			result = append(result, suit<<4|value)
		}
	}
	if frame == 2 && len(result) > 45 {
		result = result[:45]
	}
	return result
}
func firstChair(hands [][]int, users []*proto.RoomUser) int {
	for rank := 3; rank <= 12; rank++ {
		for suit := 3; suit >= 0; suit-- {
			target := suit<<4 | rank
			for _, user := range users {
				for _, card := range hands[user.ChairID] {
					if card == target {
						return user.ChairID
					}
				}
			}
		}
	}
	return users[0].ChairID
}
func containsCards(hand, cards []int) bool {
	copyHand := append([]int(nil), hand...)
	for _, card := range cards {
		found := -1
		for i, value := range copyHand {
			if value == card {
				found = i
				break
			}
		}
		if found < 0 {
			return false
		}
		copyHand = append(copyHand[:found], copyHand[found+1:]...)
	}
	return true
}

func canAutoPassOrPlay(forced bool) bool {
	return forced
}
func removeCards(hand, cards []int) []int {
	result := append([]int(nil), hand...)
	for _, card := range cards {
		for i, value := range result {
			if value == card {
				result = append(result[:i], result[i+1:]...)
				break
			}
		}
	}
	return result
}
func newPreTurnRecord(user *proto.RoomUser, cards []int) preTurnRecord {
	record := preTurnRecord{Cards: append([]int(nil), cards...)}
	if user != nil && user.UserInfo != nil {
		record.Avatar = user.UserInfo.Avatar
		record.Name = user.UserInfo.Nickname
	}
	return record
}

func clonePreTurnRecords(records []preTurnRecord) []preTurnRecord {
	result := make([]preTurnRecord, len(records))
	for index, record := range records {
		result[index] = record
		result[index].Cards = append([]int(nil), record.Cards...)
	}
	return result
}

func userData(records []preTurnRecord) any {
	return map[string]any{"type": GamePreTurnCardsPush, "data": map[string]any{"list": clonePreTurnRecords(records)}, "pushRouter": "GameMessagePush"}
}
