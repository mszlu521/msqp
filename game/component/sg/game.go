package sg

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
}
type GameFrame struct {
	r                base.RoomFrame
	rule             proto.GameRule
	data             *GameData
	dealt            [][]int
	reviewRecord     [][]*BureauReview
	timer            *time.Timer
	timeout          time.Duration
	resultDelay      time.Duration
	completedBureaus int
}

const (
	gameModeBigEatSmall  = 2
	gameModeRoundBanker  = 4
	roundResetRetryDelay = 10 * time.Millisecond
)

func NewGameFrame(rule proto.GameRule, r base.RoomFrame, _ *remote.Session) *GameFrame {
	n := rule.MaxPlayerCount
	if n < 2 {
		n = 2
	}
	if n > 10 {
		n = 10
	}
	return &GameFrame{r: r, rule: rule, dealt: make([][]int, n), reviewRecord: make([][]*BureauReview, 0), timeout: 15 * time.Second, resultDelay: 3 * time.Second, data: &GameData{GameStatus: StatusNone, BankerChairID: -1, ChairCount: n, MaxBureau: rule.Bureau, PourScores: make([]int, n), RobBankScales: fill(n, -1), HandCards: make([][]int, n), ShowCards: make([]int, n), UserTrustArray: make([]bool, n), CurScores: make([]int, n)}}
}
func fill(n, v int) []int {
	a := make([]int, n)
	for i := range a {
		a[i] = v
	}
	return a
}
func (g *GameFrame) GetGameBureauData() any                            { return g.data.Result }
func (g *GameFrame) GetGameVideoData() any                             { return cloneCards(g.dealt) }
func (g *GameFrame) IsUserEnableLeave(int) bool                        { return g.data.GameStatus == StatusNone }
func (g *GameFrame) OnEventUserEntry(*proto.RoomUser, *remote.Session) {}
func (g *GameFrame) OnEventRoomDismiss(_ enums.RoomDismissReason, session *remote.Session) {
	g.stop()
	g.all(proto.ClassicGameEndPushData(EndPush, g.r.GetUsers(), g.r.GetCreator(), g.r.GetHongBaoList(), g.completedBureaus > 0), session)
}
func (g *GameFrame) OnEventUserOffLine(u *proto.RoomUser, s *remote.Session) {
	if g.isPlayingUser(u) {
		g.data.UserTrustArray[u.ChairID] = true
		g.auto(u, s)
	}
}
func (g *GameFrame) GetEnterGameData(s *remote.Session) any {
	d := *g.data
	d.HandCards = g.hidden()
	if g.data.GameStatus == StatusResult {
		d.HandCards = cloneCards(g.dealt)
	}
	if s != nil {
		if u := g.r.GetUsers()[s.GetUid()]; u != nil && u.ChairID >= 0 && u.ChairID < len(d.HandCards) {
			d.HandCards[u.ChairID] = append([]int(nil), g.dealt[u.ChairID]...)
		}
	}
	return &d
}

func cloneCards(cards [][]int) [][]int {
	result := make([][]int, len(cards))
	for i := range cards {
		result[i] = append([]int(nil), cards[i]...)
	}
	return result
}
func (g *GameFrame) OnEventGameStart(_ *proto.RoomUser, s *remote.Session) {
	if g.data.GameStarted || len(g.players()) < 2 {
		return
	}
	deck := Deck()
	rand.New(rand.NewSource(time.Now().UnixNano())).Shuffle(len(deck), func(i, j int) { deck[i], deck[j] = deck[j], deck[i] })
	g.dealt = make([][]int, g.data.ChairCount)
	g.data.HandCards = make([][]int, g.data.ChairCount)
	g.data.PourScores = fill(g.data.ChairCount, 0)
	g.data.RobBankScales = fill(g.data.ChairCount, -1)
	g.data.ShowCards = fill(g.data.ChairCount, 0)
	for i, u := range g.players() {
		g.dealt[u.ChairID] = append([]int(nil), deck[i*3:i*3+3]...)
	}
	g.data.GameStarted = true
	g.r.SetCurBureau(g.r.GetCurBureau() + 1)
	g.data.CurBureau = g.r.GetCurBureau()
	g.data.GameStatus = StatusSend
	g.data.Tick = 1
	g.all(send(BureauPush, map[string]any{"curBureau": g.data.CurBureau}), s)
	g.all(send(SendPush, map[string]any{"handCards": g.hidden()}), s)
	for _, u := range g.players() {
		g.to(u.UserInfo.Uid, send(SendPush, map[string]any{"handCards": g.forUser(u.ChairID)}), s)
	}
	if banker, fixed := g.bankerForMode(g.players()); fixed {
		g.announceBanker(banker, s)
		g.startPour(s)
	} else {
		g.startRob(s)
	}
}

func (g *GameFrame) bankerForMode(users []*proto.RoomUser) (int, bool) {
	if len(users) == 0 {
		return -1, false
	}
	switch g.rule.GameFrameType {
	case gameModeBigEatSmall:
		return persistentBanker(users, g.data.BankerChairID), true
	case gameModeRoundBanker:
		return nextBanker(users, g.data.BankerChairID), true
	default:
		return -1, false
	}
}

func (g *GameFrame) announceBanker(chairID int, s *remote.Session) {
	g.data.BankerChairID = chairID
	g.data.RobBankScales[chairID] = 1
	g.all(send(BankerPush, map[string]any{"bankerChairID": chairID, "robScale": 1, "robChairIDs": []int{chairID}}), s)
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
func send(t int, d any) any { return push(t, d) }
func (g *GameFrame) GameMessageHandle(u *proto.RoomUser, s *remote.Session, b []byte) {
	if u == nil {
		return
	}
	var q MessageReq
	if json.Unmarshal(b, &q) != nil {
		return
	}
	if !g.data.GameStarted && q.Type != ChatNotify && q.Type != ReviewNotify {
		return
	}
	if q.Type != ChatNotify && q.Type != ReviewNotify && !g.isPlayingUser(u) {
		return
	}
	switch q.Type {
	case RobNotify:
		g.rob(u, q.Data.RobScale, s)
	case PourNotify:
		g.pour(u, q.Data.Score, s)
	case ShowNotify:
		g.show(u, q.Data.Cuopai, s)
	case TrustNotify:
		if !g.rule.CanTrust {
			return
		}
		g.data.UserTrustArray[u.ChairID] = q.Data.Trust
		g.all(send(TrustPush, map[string]any{"chairID": u.ChairID, "trust": q.Data.Trust}), s)
		if q.Data.Trust {
			g.auto(u, s)
		}
	case ChatNotify:
		g.all(send(ChatPush, map[string]any{"chairID": u.ChairID, "type": q.Data.ChatType, "msg": q.Data.Msg.Value(), "recipientID": q.Data.RecipientID}), s)
	case ReviewNotify:
		g.to(u.UserInfo.Uid, send(ReviewPush, map[string]any{"list": g.reviewRecord}), s)
	}
}

func (g *GameFrame) isPlayingUser(user *proto.RoomUser) bool {
	return user != nil && user.ChairID >= 0 && user.ChairID < g.data.ChairCount && user.UserStatus&enums.Playing > 0
}
func (g *GameFrame) startRob(s *remote.Session) {
	g.data.GameStatus = StatusRob
	g.data.Tick = 15
	g.all(status(StatusRob, 15), s)
	g.schedule(s)
}
func (g *GameFrame) rob(u *proto.RoomUser, v int, s *remote.Session) {
	if g.data.GameStatus != StatusRob || g.data.RobBankScales[u.ChairID] >= 0 || v < 0 || v > 4 {
		return
	}
	g.data.RobBankScales[u.ChairID] = v
	g.all(send(RobPush, map[string]any{"chairID": u.ChairID, "robScale": v}), s)
	if g.doneRob() {
		g.choose(s)
	}
}
func (g *GameFrame) choose(s *remote.Session) {
	best, chair := -1, g.players()[0].ChairID
	for _, u := range g.players() {
		if g.data.RobBankScales[u.ChairID] > best {
			best = g.data.RobBankScales[u.ChairID]
			chair = u.ChairID
		}
	}
	if best < 0 {
		best = 0
	}
	g.data.BankerChairID = chair
	g.all(send(BankerPush, map[string]any{"bankerChairID": chair, "robScale": best, "robChairIDs": robChairs(g.data.RobBankScales)}), s)
	g.startPour(s)
}

func robChairs(scales []int) []int {
	chairs := make([]int, 0)
	for chairID, scale := range scales {
		if scale > 0 {
			chairs = append(chairs, chairID)
		}
	}
	return chairs
}
func (g *GameFrame) startPour(s *remote.Session) {
	g.stop()
	g.data.GameStatus = StatusPour
	g.data.Tick = 15
	g.all(status(StatusPour, 15), s)
	for _, u := range g.players() {
		if u.ChairID != g.data.BankerChairID && g.data.UserTrustArray[u.ChairID] {
			g.pour(u, g.minimumPourScore(), s)
		}
	}
	if g.data.GameStatus == StatusPour {
		g.schedule(s)
	}
}
func (g *GameFrame) pour(u *proto.RoomUser, v int, s *remote.Session) {
	if g.data.GameStatus != StatusPour || u.ChairID == g.data.BankerChairID || g.data.PourScores[u.ChairID] > 0 || !g.canPour(u, v) {
		return
	}
	g.data.PourScores[u.ChairID] = v
	g.all(send(PourPush, map[string]any{"chairID": u.ChairID, "score": v}), s)
	if g.donePour() {
		g.startShow(s)
	}
}
func (g *GameFrame) startShow(s *remote.Session) {
	g.stop()
	g.data.GameStatus = StatusShow
	g.data.Tick = 15
	g.all(status(StatusShow, 15), s)
	for _, u := range g.players() {
		if g.data.UserTrustArray[u.ChairID] {
			g.show(u, false, s)
		}
	}
	g.schedule(s)
}
func (g *GameFrame) show(u *proto.RoomUser, c bool, s *remote.Session) {
	if g.data.GameStatus != StatusShow || g.data.ShowCards[u.ChairID] > 0 {
		return
	}
	g.data.ShowCards[u.ChairID] = 1
	g.data.HandCards[u.ChairID] = append([]int(nil), g.dealt[u.ChairID]...)
	g.all(send(ShowPush, map[string]any{"chairID": u.ChairID, "cards": g.dealt[u.ChairID], "cuopai": c}), s)
	if g.doneShow() {
		g.finish(s)
	}
}
func (g *GameFrame) finish(s *remote.Session) {
	g.stop()
	g.data.GameStatus = StatusResult
	g.data.GameStarted = false
	wins := fill(g.data.ChairCount, 0)
	bs := g.data.RobBankScales[g.data.BankerChairID]
	if bs < 1 {
		bs = 1
	}
	big := g.rule.ScaleType == 1
	for _, u := range g.players() {
		if u.ChairID == g.data.BankerChairID {
			continue
		}
		v := g.data.PourScores[u.ChairID]
		if v < 1 {
			v = 1
		}
		v *= bs * Evaluate(g.dealt[u.ChairID], big).Scale
		if Compare(g.dealt[g.data.BankerChairID], g.dealt[u.ChairID]) > 0 {
			wins[g.data.BankerChairID] += v
			wins[u.ChairID] -= v
		} else {
			wins[g.data.BankerChairID] -= v
			wins[u.ChairID] += v
		}
	}
	g.data.Result = map[string]any{"winScores": wins, "curScores": append([]int(nil), wins...)}
	review := make([]*BureauReview, 0, len(g.players()))
	for _, u := range g.players() {
		cardResult := Evaluate(g.dealt[u.ChairID], g.rule.ScaleType == 1)
		review = append(review, &BureauReview{
			Uid: u.UserInfo.Uid, Nickname: u.UserInfo.Nickname, Avatar: u.UserInfo.Avatar,
			Cards: append([]int(nil), g.dealt[u.ChairID]...), CardType: int(cardResult.Type),
			Rob: g.data.RobBankScales[u.ChairID], PourScore: g.data.PourScores[u.ChairID],
			WinScore: wins[u.ChairID], IsBanker: u.ChairID == g.data.BankerChairID,
		})
	}
	g.reviewRecord = append(g.reviewRecord, review)
	g.completedBureaus++
	g.all(send(ResultPush, map[string]any{"result": g.data.Result}), s)
	end := make([]*proto.EndData, 0)
	for _, u := range g.players() {
		end = append(end, &proto.EndData{Uid: u.UserInfo.Uid, Score: wins[u.ChairID]})
	}
	g.r.ConcludeGame(end, s)
	if g.r.GetMaxBureau() <= 0 || g.r.GetCurBureau() < g.r.GetMaxBureau() {
		g.scheduleRoundReset(s)
	}
}

func (g *GameFrame) scheduleRoundReset(s *remote.Session) {
	g.stop()
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
		g.all(status(StatusNone, 0), s)
	}
	if g.resultDelay <= 0 {
		reset()
		return
	}
	g.timer = time.AfterFunc(g.resultDelay, func() { g.r.RunGameAction(reset) })
}
func (g *GameFrame) auto(u *proto.RoomUser, s *remote.Session) {
	switch g.data.GameStatus {
	case StatusRob:
		g.rob(u, 0, s)
	case StatusPour:
		if u.ChairID != g.data.BankerChairID {
			g.pour(u, g.minimumPourScore(), s)
		}
	case StatusShow:
		g.show(u, false, s)
	}
}

func (g *GameFrame) canPour(user *proto.RoomUser, score int) bool {
	minimum, maximum := g.pourRange()
	if score < minimum || score > maximum || g.rule.MaxScore > 0 && score > g.rule.MaxScore {
		return false
	}
	if g.isUnionCreate() && user.UserInfo != nil {
		bankerScale := 1
		if banker := g.data.BankerChairID; banker >= 0 && banker < len(g.data.RobBankScales) && g.data.RobBankScales[banker] > 0 {
			bankerScale = g.data.RobBankScales[banker]
		}
		cardScale := 1
		if g.rule.ScaleType == 1 {
			cardScale = 5
		}
		liabilityScale := bankerScale * cardScale
		if user.UserInfo.Score < 0 || liabilityScale > 0 && score > user.UserInfo.Score/liabilityScale {
			return false
		}
	}
	return true
}

func (g *GameFrame) pourRange() (int, int) {
	baseScore := g.rule.BaseScore
	if baseScore < 1 {
		baseScore = 1
	}
	minimum := 1
	if len(g.rule.CanPourScores) > 0 && g.rule.CanPourScores[0] > 0 {
		minimum = g.rule.CanPourScores[0] * baseScore
		for _, value := range g.rule.CanPourScores[1:] {
			if value > 0 && value*baseScore < minimum {
				minimum = value * baseScore
			}
		}
	}
	maximum := minimum
	if g.rule.MaxCanPourGold > 0 {
		maximum = g.rule.MaxCanPourGold * baseScore
	} else if len(g.rule.CanPourScores) > 0 {
		for _, value := range g.rule.CanPourScores {
			if value*baseScore > maximum {
				maximum = value * baseScore
			}
		}
	} else if g.rule.MaxScore > 0 {
		maximum = g.rule.MaxScore
	} else {
		maximum = int(^uint(0) >> 1)
	}
	return minimum, maximum
}

func (g *GameFrame) minimumPourScore() int {
	minimum, _ := g.pourRange()
	return minimum
}

func (g *GameFrame) isUnionCreate() bool {
	creator := g.r.GetCreator()
	return creator != nil && creator.CreatorType == enums.UnionCreatorType
}
func (g *GameFrame) schedule(s *remote.Session) {
	g.stop()
	g.timer = time.AfterFunc(g.timeout, func() {
		g.r.RunGameAction(func() {
			if u := g.current(); u != nil {
				g.auto(u, s)
			}
			if g.data.GameStarted && g.current() != nil {
				g.schedule(s)
			}
		})
	})
}
func (g *GameFrame) stop() {
	if g.timer != nil {
		g.timer.Stop()
		g.timer = nil
	}
}
func (g *GameFrame) current() *proto.RoomUser {
	for _, u := range g.players() {
		if g.data.GameStatus == StatusRob && g.data.RobBankScales[u.ChairID] < 0 {
			return u
		}
		if g.data.GameStatus == StatusPour && u.ChairID != g.data.BankerChairID && g.data.PourScores[u.ChairID] == 0 {
			return u
		}
		if g.data.GameStatus == StatusShow && g.data.ShowCards[u.ChairID] == 0 {
			return u
		}
	}
	return nil
}
func (g *GameFrame) doneRob() bool {
	for _, u := range g.players() {
		if g.data.RobBankScales[u.ChairID] < 0 {
			return false
		}
	}
	return true
}
func (g *GameFrame) donePour() bool {
	for _, u := range g.players() {
		if u.ChairID != g.data.BankerChairID && g.data.PourScores[u.ChairID] == 0 {
			return false
		}
	}
	return true
}
func (g *GameFrame) doneShow() bool {
	for _, u := range g.players() {
		if g.data.ShowCards[u.ChairID] == 0 {
			return false
		}
	}
	return true
}
func (g *GameFrame) players() []*proto.RoomUser {
	a := []*proto.RoomUser{}
	for _, u := range g.r.GetUsers() {
		if u != nil && u.UserInfo != nil && u.ChairID >= 0 && u.ChairID < g.data.ChairCount && u.UserStatus&enums.Playing > 0 {
			a = append(a, u)
		}
	}
	sort.Slice(a, func(i, j int) bool { return a[i].ChairID < a[j].ChairID })
	return a
}
func (g *GameFrame) hidden() [][]int {
	a := make([][]int, len(g.dealt))
	for i := range a {
		if g.dealt[i] != nil {
			a[i] = make([]int, 3)
		}
	}
	return a
}
func (g *GameFrame) forUser(c int) [][]int {
	a := g.hidden()
	if c >= 0 && c < len(a) {
		a[c] = append([]int(nil), g.dealt[c]...)
	}
	return a
}
func (g *GameFrame) all(v any, s *remote.Session) { g.r.SendDataAll(s.GetMsg(), v) }
func (g *GameFrame) to(uid string, v any, s *remote.Session) {
	g.r.SendData(s.GetMsg(), []string{uid}, v)
}
