package nn

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
	r                base.RoomFrame
	rule             proto.GameRule
	data             *GameData
	ruleCards        Rule
	timer            *time.Timer
	timeout          time.Duration
	dealt            [][]int
	reviewRecord     [][]*BureauReview
	completedBureaus int
	resultDelay      time.Duration
}

const (
	gameModeNiuNiu             = 1
	gameModeRoundBanker        = 3
	gameModeOwnerBanker        = 5
	gameModeMingPaiQiangZhuang = 6
	roundResetRetryDelay       = 10 * time.Millisecond
)

func NewGameFrame(rule proto.GameRule, r base.RoomFrame, _ *remote.Session) *GameFrame {
	count := rule.MaxPlayerCount
	if count < 2 {
		count = 2
	}
	if count > 10 {
		count = 10
	}
	return &GameFrame{r: r, rule: rule, ruleCards: cardRule(rule), data: &GameData{
		GameStatus: StatusNone, BankerChairID: -1, ChairCount: count,
		MaxBureau: rule.Bureau, PourScores: make([]int, count), RobBankScales: fill(count, -1),
		HandCards: make([][]int, count), ShowCards: make([]int, count),
		UserTrustArray: make([]bool, count), CurScores: make([]int, count), TuiArr: make([]int, count),
	}, dealt: make([][]int, count), reviewRecord: make([][]*BureauReview, 0), timeout: 10 * time.Second, resultDelay: 3 * time.Second}
}

func cardRule(rule proto.GameRule) Rule {
	return Rule{ScaleType: ScaleType(rule.ScaleType), ShunZiNiu: rule.CardsType["SHUNZINIU"], YinNiu: rule.CardsType["YINNIU"], TongHuaNiu: rule.CardsType["TONGHUANIU"], WuHuaNiu: rule.CardsType["WUHUANIU"], HuLuNiu: rule.CardsType["HULUNIU"], WuXiaoNiu: rule.CardsType["WUXIAONIU"], ZhaDanNiu: rule.CardsType["ZHADANNIU"], YiTiaoLong: rule.CardsType["YITIAOLONG"], TongHuaShun: rule.CardsType["TONGHUASHUN"]}
}

func fill(n, value int) []int {
	result := make([]int, n)
	for i := range result {
		result[i] = value
	}
	return result
}
func (g *GameFrame) GetGameBureauData() any                                { return g.data.Result }
func (g *GameFrame) GetGameVideoData() any                                 { return cloneCards(g.dealt) }
func (g *GameFrame) IsUserEnableLeave(chairID int) bool                    { return g.data.GameStatus == StatusNone }
func (g *GameFrame) OnEventUserEntry(_ *proto.RoomUser, _ *remote.Session) {}
func (g *GameFrame) OnEventUserOffLine(user *proto.RoomUser, session *remote.Session) {
	if g.isPlayingUser(user) {
		g.data.UserTrustArray[user.ChairID] = true
		if user.ChairID == g.currentChair() {
			g.autoOperate(user, session)
		}
	}
}
func (g *GameFrame) OnEventRoomDismiss(_ enums.RoomDismissReason, session *remote.Session) {
	if g.timer != nil {
		g.timer.Stop()
	}
	g.sendAll(proto.ClassicGameEndPushData(GameEndPush, g.r.GetUsers(), g.r.GetCreator(), g.r.GetHongBaoList(), g.completedBureaus > 0), session)
}

func (g *GameFrame) GetEnterGameData(session *remote.Session) any {
	copyData := *g.data
	copyData.HandCards = g.hiddenCards()
	if g.data.GameStatus == StatusResult {
		copyData.HandCards = cloneCards(g.dealt)
	}
	if session != nil {
		if user := g.r.GetUsers()[session.GetUid()]; user != nil && user.ChairID >= 0 && user.ChairID < len(copyData.HandCards) {
			copyData.HandCards[user.ChairID] = append([]int(nil), g.dealt[user.ChairID]...)
		}
	}
	return &copyData
}

func (g *GameFrame) OnEventGameStart(_ *proto.RoomUser, session *remote.Session) {
	if g.data.GameStarted {
		return
	}
	users := g.playingUsers()
	if len(users) < 2 {
		return
	}
	deck := fullDeck()
	rand.New(rand.NewSource(time.Now().UnixNano())).Shuffle(len(deck), func(i, j int) { deck[i], deck[j] = deck[j], deck[i] })
	g.dealt = make([][]int, len(g.data.HandCards))
	g.data.HandCards = make([][]int, len(g.data.HandCards))
	g.data.PourScores = fill(len(g.data.HandCards), 0)
	g.data.RobBankScales = fill(len(g.data.HandCards), -1)
	g.data.ShowCards = fill(len(g.data.HandCards), 0)
	for i, user := range users {
		g.dealt[user.ChairID] = append([]int(nil), deck[i*5:i*5+5]...)
		g.data.HandCards[user.ChairID] = nil
	}
	g.data.GameStarted = true
	g.r.SetCurBureau(g.r.GetCurBureau() + 1)
	g.data.CurBureau = g.r.GetCurBureau()
	g.data.GameStatus = StatusSendCards
	g.data.Tick = 1
	g.sendAll(push(GameBureauPush, map[string]any{"curBureau": g.data.CurBureau}), session)
	g.sendAll(sendCardsPush(g.hiddenCards()), session)
	g.sendAll(statusPush(StatusSendCards, 1), session)
	for _, user := range users {
		g.send(sendCardsPush(g.cardsForUser(user.ChairID)), []string{user.UserInfo.Uid}, session)
	}
	if banker, fixed := g.bankerForMode(users); fixed {
		g.announceBanker(banker, session)
		g.startPour(session)
	} else {
		g.startRob(session)
	}
}

func (g *GameFrame) bankerForMode(users []*proto.RoomUser) (int, bool) {
	if len(users) == 0 {
		return -1, false
	}
	switch g.rule.GameFrameType {
	case gameModeNiuNiu:
		return persistentBanker(users, g.data.BankerChairID), true
	case gameModeRoundBanker:
		return nextBanker(users, g.data.BankerChairID), true
	case gameModeOwnerBanker:
		if creator := g.r.GetCreator(); creator != nil {
			for _, user := range users {
				if roomUserUID(user) == creator.Uid {
					return user.ChairID, true
				}
			}
		}
		return users[0].ChairID, true
	default:
		return -1, false
	}
}

func (g *GameFrame) announceBanker(chairID int, session *remote.Session) {
	g.data.BankerChairID = chairID
	g.data.RobBankScales[chairID] = 1
	g.sendAll(bankerPush(chairID, 1, []int{chairID}), session)
}

func persistentBanker(users []*proto.RoomUser, previous int) int {
	for _, user := range users {
		if user.ChairID == previous {
			return previous
		}
	}
	return users[0].ChairID
}

func nextBanker(users []*proto.RoomUser, previous int) int {
	if previous < 0 {
		return users[0].ChairID
	}
	for _, user := range users {
		if user.ChairID > previous {
			return user.ChairID
		}
	}
	return users[0].ChairID
}

func roomUserUID(user *proto.RoomUser) string {
	if user == nil {
		return ""
	}
	if user.Uid != "" {
		return user.Uid
	}
	if user.UserInfo != nil {
		return user.UserInfo.Uid
	}
	return ""
}

func (g *GameFrame) GameMessageHandle(user *proto.RoomUser, session *remote.Session, msg []byte) {
	if user == nil {
		return
	}
	var req MessageReq
	if json.Unmarshal(msg, &req) != nil {
		return
	}
	if !g.data.GameStarted && req.Type != GameChatNotify && req.Type != GameReviewNotify {
		return
	}
	if req.Type != GameChatNotify && req.Type != GameReviewNotify && !g.isPlayingUser(user) {
		return
	}
	switch req.Type {
	case GameRobBankNotify:
		g.rob(user, req.Data.RobScale, session)
	case GamePourScoreNotify:
		g.pour(user, req.Data.Score, session)
	case GameShowCardsNotify:
		g.show(user, req.Data.Cuopai, session)
	case GameTrustNotify:
		if !g.rule.CanTrust {
			return
		}
		g.data.UserTrustArray[user.ChairID] = req.Data.Trust
		g.sendAll(trustPush(user.ChairID, req.Data.Trust), session)
		if req.Data.Trust {
			g.autoOperate(user, session)
		}
	case GameChatNotify:
		g.sendAll(chatPush(user.ChairID, req.Data.ChatType, req.Data.Msg.Value(), req.Data.RecipientID), session)
	case GameReviewNotify:
		g.send(push(GameReviewPush, map[string]any{"list": g.reviewRecord}), []string{user.UserInfo.Uid}, session)
	}
}

func (g *GameFrame) isPlayingUser(user *proto.RoomUser) bool {
	return user != nil && user.ChairID >= 0 && user.ChairID < g.data.ChairCount && user.UserStatus&enums.Playing > 0
}

func (g *GameFrame) startRob(session *remote.Session) {
	g.data.GameStatus, g.data.Tick = StatusRobBank, 10
	g.sendAll(statusPush(StatusRobBank, 10), session)
	g.schedule(session)
}
func (g *GameFrame) rob(user *proto.RoomUser, scale int, session *remote.Session) {
	if g.data.GameStatus != StatusRobBank || g.data.RobBankScales[user.ChairID] >= 0 {
		return
	}
	if scale < 0 || scale > 4 {
		return
	}
	if g.rule.RobBankLimit > 0 && user.UserInfo != nil && user.UserInfo.Score < g.rule.RobBankLimit {
		return
	}
	g.data.RobBankScales[user.ChairID] = scale
	g.sendAll(robPush(user.ChairID, scale), session)
	if g.allRobbed() {
		g.chooseBanker(session)
	}
}
func (g *GameFrame) chooseBanker(session *remote.Session) {
	users := g.playingUsers()
	best := -1
	banker := users[0].ChairID
	for _, u := range users {
		if g.data.RobBankScales[u.ChairID] > best {
			best = g.data.RobBankScales[u.ChairID]
			banker = u.ChairID
		}
	}
	if best < 0 {
		best = 0
	}
	g.data.BankerChairID = banker
	g.sendAll(bankerPush(banker, best, usersWithRob(g.data.RobBankScales)), session)
	g.startPour(session)
}
func usersWithRob(scales []int) []int {
	result := []int{}
	for i, v := range scales {
		if v > 0 {
			result = append(result, i)
		}
	}
	return result
}
func (g *GameFrame) startPour(session *remote.Session) {
	g.stopTimer()
	g.data.GameStatus, g.data.Tick = StatusPourScore, 10
	g.sendAll(statusPush(StatusPourScore, 10), session)
	banker := g.data.BankerChairID
	for _, u := range g.playingUsers() {
		if u.ChairID == banker {
			continue
		}
		if g.data.UserTrustArray[u.ChairID] {
			g.pour(u, g.minimumPourScore(u.ChairID), session)
		}
	}
	if g.data.GameStatus == StatusPourScore {
		g.schedule(session)
	}
}
func (g *GameFrame) pour(user *proto.RoomUser, score int, session *remote.Session) {
	if g.data.GameStatus != StatusPourScore || user.ChairID == g.data.BankerChairID || g.data.PourScores[user.ChairID] > 0 || !g.canPour(user, score) {
		return
	}
	g.data.PourScores[user.ChairID] = score
	g.sendAll(pourPush(user.ChairID, score), session)
	if g.allPoured() {
		g.startShow(session)
	}
}
func (g *GameFrame) startShow(session *remote.Session) {
	g.stopTimer()
	g.data.GameStatus, g.data.Tick = StatusShowCards, 10
	g.sendAll(statusPush(StatusShowCards, 10), session)
	for _, u := range g.playingUsers() {
		if g.data.UserTrustArray[u.ChairID] {
			g.show(u, false, session)
		}
	}
	g.schedule(session)
}
func (g *GameFrame) show(user *proto.RoomUser, cuopai bool, session *remote.Session) {
	if g.data.GameStatus != StatusShowCards || g.data.ShowCards[user.ChairID] > 0 {
		return
	}
	g.data.ShowCards[user.ChairID] = 1
	g.data.HandCards[user.ChairID] = append([]int(nil), g.dealt[user.ChairID]...)
	g.sendAll(showPush(user.ChairID, g.dealt[user.ChairID], cuopai), session)
	if g.allShown() {
		g.finish(session)
	}
}
func (g *GameFrame) finish(session *remote.Session) {
	g.stopTimer()
	g.data.GameStatus, g.data.GameStarted = StatusResult, false
	result := map[string]any{"winScores": fill(g.data.ChairCount, 0), "curScores": append([]int(nil), g.data.PourScores...), "tuiArr": fill(g.data.ChairCount, 0)}
	wins := result["winScores"].([]int)
	banker := g.dealt[g.data.BankerChairID]
	for _, u := range g.playingUsers() {
		if u.ChairID == g.data.BankerChairID {
			continue
		}
		playerResult := Evaluate(g.dealt[u.ChairID], g.ruleCards)
		cmp := Compare(banker, g.dealt[u.ChairID], g.ruleCards)
		scale := g.data.PourScores[u.ChairID]
		if scale == 0 {
			scale = 1
		}
		bankerScale := g.data.RobBankScales[g.data.BankerChairID]
		if bankerScale < 1 {
			bankerScale = 1
		}
		scale *= bankerScale * playerResult.Scale
		if cmp > 0 {
			wins[g.data.BankerChairID] += scale
			wins[u.ChairID] -= scale
		} else if cmp < 0 {
			wins[g.data.BankerChairID] -= scale
			wins[u.ChairID] += scale
		}
	}
	g.data.Result = result
	review := make([]*BureauReview, 0, len(g.playingUsers()))
	for _, u := range g.playingUsers() {
		cardResult := Evaluate(g.dealt[u.ChairID], g.ruleCards)
		review = append(review, &BureauReview{
			Uid: u.UserInfo.Uid, Nickname: u.UserInfo.Nickname, Avatar: u.UserInfo.Avatar,
			Cards: append([]int(nil), g.dealt[u.ChairID]...), CardType: int(cardResult.Type),
			Rob: g.data.RobBankScales[u.ChairID], PourScore: g.data.PourScores[u.ChairID],
			WinScore: wins[u.ChairID], IsBanker: u.ChairID == g.data.BankerChairID,
		})
	}
	g.reviewRecord = append(g.reviewRecord, review)
	g.completedBureaus++
	g.sendAll(resultPush(result), session)
	end := make([]*proto.EndData, 0)
	for _, u := range g.playingUsers() {
		end = append(end, &proto.EndData{Uid: u.UserInfo.Uid, Score: wins[u.ChairID]})
	}
	g.r.ConcludeGame(end, session)
	if g.r.GetMaxBureau() <= 0 || g.r.GetCurBureau() < g.r.GetMaxBureau() {
		g.scheduleRoundReset(session)
	}
}

func (g *GameFrame) scheduleRoundReset(session *remote.Session) {
	g.stopTimer()
	var reset func()
	reset = func() {
		if g.data.GameStatus != StatusResult {
			return
		}
		if g.r.IsDismissing() {
			g.timer = time.AfterFunc(roundResetRetryDelay, func() { g.r.RunGameAction(reset) })
			return
		}
		g.data.GameStatus = StatusNone
		g.data.GameStarted = false
		g.data.Tick = 0
		g.data.Result = nil
		g.data.HandCards = make([][]int, g.data.ChairCount)
		g.data.PourScores = fill(g.data.ChairCount, 0)
		g.data.RobBankScales = fill(g.data.ChairCount, -1)
		g.data.ShowCards = fill(g.data.ChairCount, 0)
		if g.rule.GameFrameType == gameModeMingPaiQiangZhuang {
			g.data.BankerChairID = -1
		}
		g.sendAll(statusPush(StatusNone, 0), session)
	}
	if g.resultDelay <= 0 {
		reset()
		return
	}
	g.timer = time.AfterFunc(g.resultDelay, func() { g.r.RunGameAction(reset) })
}
func (g *GameFrame) autoOperate(user *proto.RoomUser, session *remote.Session) {
	if g.data.GameStatus == StatusRobBank {
		g.rob(user, 0, session)
	} else if g.data.GameStatus == StatusPourScore && user.ChairID != g.data.BankerChairID {
		g.pour(user, g.minimumPourScore(user.ChairID), session)
	} else if g.data.GameStatus == StatusShowCards {
		g.show(user, false, session)
	}
}

func (g *GameFrame) canPour(user *proto.RoomUser, score int) bool {
	if score < 1 || g.rule.MaxScore > 0 && score > g.rule.MaxScore {
		return false
	}
	allowed := g.allowedPourScores(user.ChairID)
	if len(allowed) > 0 {
		valid := false
		for _, value := range allowed {
			if score == value {
				valid = true
				break
			}
		}
		if !valid {
			return false
		}
	}
	if g.isUnionCreate() && user.UserInfo != nil {
		bankerScale := 1
		if banker := g.data.BankerChairID; banker >= 0 && banker < len(g.data.RobBankScales) && g.data.RobBankScales[banker] > 0 {
			bankerScale = g.data.RobBankScales[banker]
		}
		liabilityScale := bankerScale * g.maximumCardScale()
		if user.UserInfo.Score < 0 || liabilityScale > 0 && score > user.UserInfo.Score/liabilityScale {
			return false
		}
	}
	return true
}

func (g *GameFrame) allowedPourScores(chairID int) []int {
	if len(g.rule.CanPourScores) == 0 {
		return nil
	}
	baseScore := g.rule.BaseScore
	if baseScore < 1 {
		baseScore = 1
	}
	values := make([]int, 0, len(g.rule.CanPourScores)+len(g.rule.TuiScale))
	for _, value := range g.rule.CanPourScores {
		if value > 0 {
			values = append(values, value*baseScore)
		}
	}
	if g.rule.Tuizhu && chairID >= 0 && chairID < len(g.data.TuiArr) && g.data.TuiArr[chairID] > 0 && len(g.rule.CanPourScores) > 0 {
		for _, scale := range g.rule.TuiScale {
			if scale > 0 {
				values = append(values, scale*g.rule.CanPourScores[0]*baseScore)
			}
		}
	}
	return values
}

func (g *GameFrame) minimumPourScore(chairID int) int {
	values := g.allowedPourScores(chairID)
	if len(values) == 0 {
		return 1
	}
	minimum := values[0]
	for _, value := range values[1:] {
		if value < minimum {
			minimum = value
		}
	}
	return minimum
}

func (g *GameFrame) maximumCardScale() int {
	maximum := scaleFor(NiuNiu, 10, g.ruleCards)
	for typ := ShunZiNiu; typ <= TongHuaShun; typ++ {
		enabled := map[CardType]bool{ShunZiNiu: g.ruleCards.ShunZiNiu, YinNiu: g.ruleCards.YinNiu, TongHuaNiu: g.ruleCards.TongHuaNiu, WuHuaNiu: g.ruleCards.WuHuaNiu, HuLuNiu: g.ruleCards.HuLuNiu, WuXiaoNiu: g.ruleCards.WuXiaoNiu, ZhaDanNiu: g.ruleCards.ZhaDanNiu, YiTiaoLong: g.ruleCards.YiTiaoLong, TongHuaShun: g.ruleCards.TongHuaShun}[typ]
		if enabled && scaleFor(typ, 0, g.ruleCards) > maximum {
			maximum = scaleFor(typ, 0, g.ruleCards)
		}
	}
	if maximum < 1 {
		return 1
	}
	return maximum
}

func (g *GameFrame) isUnionCreate() bool {
	creator := g.r.GetCreator()
	return creator != nil && creator.CreatorType == enums.UnionCreatorType
}
func (g *GameFrame) schedule(session *remote.Session) {
	g.stopTimer()
	g.timer = time.AfterFunc(g.timeout, func() {
		g.r.RunGameAction(func() {
			for _, u := range g.playingUsers() {
				if u.ChairID == g.currentChair() {
					g.autoOperate(u, session)
					break
				}
			}
			if g.data.GameStarted && g.currentChair() >= 0 {
				g.schedule(session)
			}
		})
	})
}
func (g *GameFrame) stopTimer() {
	if g.timer != nil {
		g.timer.Stop()
		g.timer = nil
	}
}
func (g *GameFrame) currentChair() int {
	if g.data.GameStatus == StatusRobBank {
		for _, u := range g.playingUsers() {
			if g.data.RobBankScales[u.ChairID] < 0 {
				return u.ChairID
			}
		}
	}
	if g.data.GameStatus == StatusPourScore {
		for _, u := range g.playingUsers() {
			if u.ChairID != g.data.BankerChairID && g.data.PourScores[u.ChairID] == 0 {
				return u.ChairID
			}
		}
	}
	if g.data.GameStatus == StatusShowCards {
		for _, u := range g.playingUsers() {
			if g.data.ShowCards[u.ChairID] == 0 {
				return u.ChairID
			}
		}
	}
	return -1
}
func (g *GameFrame) allRobbed() bool {
	for _, u := range g.playingUsers() {
		if g.data.RobBankScales[u.ChairID] < 0 {
			return false
		}
	}
	return true
}
func (g *GameFrame) allPoured() bool {
	for _, u := range g.playingUsers() {
		if u.ChairID != g.data.BankerChairID && g.data.PourScores[u.ChairID] == 0 {
			return false
		}
	}
	return true
}
func (g *GameFrame) allShown() bool {
	for _, u := range g.playingUsers() {
		if g.data.ShowCards[u.ChairID] == 0 {
			return false
		}
	}
	return true
}
func (g *GameFrame) playingUsers() []*proto.RoomUser {
	result := []*proto.RoomUser{}
	for _, u := range g.r.GetUsers() {
		if u != nil && u.UserInfo != nil && u.ChairID >= 0 && u.ChairID < len(g.data.HandCards) && u.UserStatus&enums.Playing > 0 {
			result = append(result, u)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ChairID < result[j].ChairID })
	return result
}
func (g *GameFrame) hiddenCards() [][]int {
	result := make([][]int, len(g.dealt))
	for i := range result {
		if g.dealt[i] != nil {
			result[i] = make([]int, 5)
		}
	}
	return result
}
func (g *GameFrame) cardsForUser(chairID int) [][]int {
	result := g.hiddenCards()
	if chairID >= 0 && chairID < len(result) {
		result[chairID] = append([]int(nil), g.dealt[chairID]...)
	}
	return result
}
func (g *GameFrame) send(data any, users []string, session *remote.Session) {
	g.r.SendData(session.GetMsg(), users, data)
}
func (g *GameFrame) sendAll(data any, session *remote.Session) {
	g.r.SendDataAll(session.GetMsg(), data)
}
func fullDeck() []int {
	cards := make([]int, 0, 52)
	for suit := 0; suit < 4; suit++ {
		for value := 1; value <= 13; value++ {
			cards = append(cards, suit<<4|value)
		}
	}
	return cards
}
func cloneCards(cards [][]int) [][]int {
	result := make([][]int, len(cards))
	for i := range cards {
		result[i] = append([]int(nil), cards[i]...)
	}
	return result
}
