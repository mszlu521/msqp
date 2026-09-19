package dgn

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
	GameStatus        int     `json:"gameStatus"`
	GameStarted       bool    `json:"gameStarted"`
	Tick              int     `json:"tick"`
	BankerChairID     int     `json:"bankerChairID"`
	PourScores        []int   `json:"pourScores"`
	HandCards         [][]int `json:"handCards"`
	CurBureau         int     `json:"curBureau"`
	Turn              int     `json:"turn"`
	BankerTurn        int     `json:"bankerTurn"`
	BankerPourTurn    int     `json:"bankerPourTurn"`
	PoolScore         int     `json:"poolScore"`
	MaxBureau         int     `json:"maxBureau"`
	ChairCount        int     `json:"chairCount"`
	ShowCards         []bool  `json:"showCards"`
	UserTrustArray    []bool  `json:"userTrustArray"`
	ShemenArray       []bool  `json:"shemenArray"`
	NextBankerChairID int     `json:"nextBankerChairID"`
	Result            any     `json:"result"`
}
type GameFrame struct {
	r                 base.RoomFrame
	rule              proto.GameRule
	data              *GameData
	dealt             [][]int
	reviewRecord      [][]*BureauReview
	choices           []int
	pendingBanker     int
	pendingBankerTurn int
	timer             *time.Timer
	timeout           time.Duration
	resultDelay       time.Duration
	completedBureaus  int
}

const roundResetRetryDelay = 10 * time.Millisecond

func NewGameFrame(rule proto.GameRule, r base.RoomFrame, _ *remote.Session) *GameFrame {
	n := rule.MaxPlayerCount
	if n < 2 {
		n = 2
	}
	if n > 10 {
		n = 10
	}
	return &GameFrame{r: r, rule: rule, dealt: make([][]int, n), reviewRecord: make([][]*BureauReview, 0), choices: fill(n, -1), pendingBanker: -1, timeout: 5 * time.Second, resultDelay: 3 * time.Second, data: &GameData{GameStatus: StatusNone, BankerChairID: -1, NextBankerChairID: -1, ChairCount: n, MaxBureau: rule.Bureau, PourScores: make([]int, n), HandCards: make([][]int, n), ShowCards: make([]bool, n), UserTrustArray: make([]bool, n), ShemenArray: make([]bool, n)}}
}
func fill(n, v int) []int {
	a := make([]int, n)
	for i := range a {
		a[i] = v
	}
	return a
}
func (g *GameFrame) GetGameBureauData() any                            { return clone(g.dealt) }
func (g *GameFrame) GetGameVideoData() any                             { return clone(g.dealt) }
func (g *GameFrame) IsUserEnableLeave(int) bool                        { return g.data.GameStatus == StatusNone }
func (g *GameFrame) OnEventUserEntry(*proto.RoomUser, *remote.Session) {}
func (g *GameFrame) OnEventRoomDismiss(_ enums.RoomDismissReason, session *remote.Session) {
	g.stop()
	g.all(proto.ClassicGameEndPushData(EndPush, g.r.GetUsers(), g.r.GetCreator(), g.r.GetHongBaoList(), g.completedBureaus > 0), session)
}
func (g *GameFrame) OnEventUserOffLine(u *proto.RoomUser, s *remote.Session) {
	if g.isPlayingUser(u) {
		g.data.UserTrustArray[u.ChairID] = true
		if g.data.GameStatus == StatusChooseBank {
			g.choose(u, false, s)
		} else {
			g.auto(u, s)
		}
	}
}
func (g *GameFrame) GetEnterGameData(s *remote.Session) any {
	d := *g.data
	d.HandCards = g.hidden()
	if g.data.GameStatus == StatusResult {
		d.HandCards = clone(g.dealt)
	}
	if s != nil {
		if u := g.r.GetUsers()[s.GetUid()]; u != nil && u.ChairID >= 0 && u.ChairID < len(d.HandCards) {
			d.HandCards[u.ChairID] = append([]int(nil), g.dealt[u.ChairID]...)
		}
	}
	return &d
}
func (g *GameFrame) OnEventGameStart(_ *proto.RoomUser, s *remote.Session) {
	users := g.players()
	if g.data.GameStarted || len(users) < 2 {
		return
	}
	pendingBanker, pendingBankerTurn, hasPendingBanker := g.pendingBankerFor(users)
	g.pendingBanker = -1
	g.pendingBankerTurn = 0
	deck := deck()
	rand.New(rand.NewSource(time.Now().UnixNano())).Shuffle(len(deck), func(i, j int) { deck[i], deck[j] = deck[j], deck[i] })
	g.dealt = make([][]int, g.data.ChairCount)
	g.data.HandCards = make([][]int, g.data.ChairCount)
	g.data.PourScores = fill(g.data.ChairCount, 0)
	g.data.ShowCards = make([]bool, g.data.ChairCount)
	g.data.ShemenArray = make([]bool, g.data.ChairCount)
	g.choices = fill(g.data.ChairCount, -1)
	for i, u := range users {
		g.dealt[u.ChairID] = append([]int(nil), deck[i*5:i*5+5]...)
	}
	g.data.GameStarted = true
	g.r.SetCurBureau(g.r.GetCurBureau() + 1)
	g.data.CurBureau = g.r.GetCurBureau()
	g.data.GameStatus = StatusChooseBank
	g.data.Tick = 5
	bankerTurn := 1
	if hasPendingBanker && pendingBankerTurn > 0 {
		bankerTurn = pendingBankerTurn
	}
	g.all(push(BureauPush, map[string]any{"curBureau": g.data.CurBureau, "turn": 1, "bankerTurn": bankerTurn, "bankerPourTurn": 0}), s)
	g.all(status(StatusChooseBank, 5), s)
	for _, u := range users {
		g.to(u.UserInfo.Uid, push(CardsPush, map[string]any{"handCards": g.forUser(u.ChairID)}), s)
	}
	if hasPendingBanker {
		g.startWithBanker(pendingBanker, pendingBankerTurn, true, s)
	} else {
		g.promptBankerChoice(s)
		g.schedule(s)
	}
}
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
	case ChooseBankNotify:
		g.choose(u, q.Data.BeBanker, s)
	case PourNotify:
		g.pour(u, q.Data.Score, q.Data.Shemen, s)
	case ShowNotify:
		g.show(u, s)
	case TrustNotify:
		if !g.rule.CanTrust {
			return
		}
		g.data.UserTrustArray[u.ChairID] = q.Data.Trust
		g.all(push(TrustPush, map[string]any{"chairID": u.ChairID, "trust": q.Data.Trust}), s)
		if q.Data.Trust {
			g.auto(u, s)
		}
	case ChatNotify:
		g.all(push(ChatPush, map[string]any{"chairID": u.ChairID, "type": q.Data.ChatType, "msg": q.Data.Msg.Value(), "recipientID": q.Data.RecipientID}), s)
	case ReviewNotify:
		g.to(u.UserInfo.Uid, push(ReviewPush, map[string]any{"list": g.reviewRecord}), s)
	}
}

func (g *GameFrame) isPlayingUser(user *proto.RoomUser) bool {
	return user != nil && user.ChairID >= 0 && user.ChairID < g.data.ChairCount && user.UserStatus&enums.Playing > 0
}
func (g *GameFrame) choose(u *proto.RoomUser, be bool, s *remote.Session) {
	current := g.current()
	if g.data.GameStatus != StatusChooseBank || current == nil || current.ChairID != u.ChairID {
		return
	}
	if be {
		g.choices[u.ChairID] = 1
	} else {
		g.choices[u.ChairID] = 0
	}
	ok := true
	for _, p := range g.players() {
		if g.choices[p.ChairID] < 0 {
			ok = false
		}
	}
	if ok {
		g.selectBank(s)
		return
	}
	g.promptBankerChoice(s)
	g.schedule(s)
}

func (g *GameFrame) promptBankerChoice(s *remote.Session) {
	if u := g.current(); u != nil {
		g.data.NextBankerChairID = u.ChairID
		g.all(push(ChooseBankPush, map[string]any{"chairID": u.ChairID}), s)
	}
}
func (g *GameFrame) selectBank(s *remote.Session) {
	ps := g.players()
	chair := ps[0].ChairID
	for _, p := range ps {
		if g.choices[p.ChairID] > 0 {
			chair = p.ChairID
			break
		}
	}
	g.startWithBanker(chair, 1, false, s)
}

func (g *GameFrame) startWithBanker(chair, turn int, preservePool bool, s *remote.Session) {
	g.data.BankerChairID = chair
	g.data.NextBankerChairID = -1
	g.data.BankerTurn = turn
	g.data.BankerPourTurn = 0
	g.data.Turn = 1
	if !preservePool || g.data.PoolScore <= 0 {
		g.data.PoolScore = g.initialPool()
	}
	g.all(push(BankerPush, map[string]any{"bankerChairID": chair, "poolScore": g.data.PoolScore, "bankerTurn": turn, "bankerPourTurn": 0}), s)
	g.startPour(s)
}

func (g *GameFrame) pendingBankerFor(users []*proto.RoomUser) (int, int, bool) {
	if g.pendingBanker < 0 {
		return -1, 0, false
	}
	for _, user := range users {
		if user.ChairID == g.pendingBanker {
			return g.pendingBanker, g.pendingBankerTurn, true
		}
	}
	return -1, 0, false
}

func nextValidChair(users []*proto.RoomUser, previous int) int {
	if len(users) == 0 {
		return -1
	}
	for _, user := range users {
		if user.ChairID > previous {
			return user.ChairID
		}
	}
	return users[0].ChairID
}
func (g *GameFrame) initialPool() int {
	v := g.rule.Shouzhuang
	if v < 1 {
		v = 1
	}
	return v
}
func (g *GameFrame) startPour(s *remote.Session) {
	g.stop()
	g.data.GameStatus = StatusPour
	g.data.Tick = 5
	g.data.PourScores[g.data.BankerChairID] = g.data.PoolScore
	g.all(status(StatusPour, 5), s)
	for _, u := range g.players() {
		if u.ChairID != g.data.BankerChairID && g.data.UserTrustArray[u.ChairID] {
			g.pour(u, g.minimumPourScore(), false, s)
		}
	}
	g.schedule(s)
}
func (g *GameFrame) pour(u *proto.RoomUser, v int, shemen bool, s *remote.Session) {
	if g.data.GameStatus != StatusPour || u.ChairID == g.data.BankerChairID || v < 1 || g.data.PourScores[u.ChairID] > 0 {
		return
	}
	min := g.minimumPourScore()
	if v < min {
		return
	}
	max := g.data.PoolScore / 3
	if max > 0 && v > max {
		return
	}
	g.data.PourScores[u.ChairID] = v
	g.data.ShemenArray[u.ChairID] = shemen
	g.data.BankerPourTurn++
	g.all(push(PourPush, map[string]any{"chairID": u.ChairID, "score": v, "shemen": shemen}), s)
	if g.donePour() {
		g.startShow(s)
	}
}
func (g *GameFrame) startShow(s *remote.Session) {
	g.stop()
	g.data.GameStatus = StatusShow
	g.data.Tick = 5
	g.all(status(StatusShow, 5), s)
	for _, u := range g.players() {
		if g.data.UserTrustArray[u.ChairID] {
			g.show(u, s)
		}
	}
	g.schedule(s)
}
func (g *GameFrame) show(u *proto.RoomUser, s *remote.Session) {
	if g.data.GameStatus != StatusShow || g.data.ShowCards[u.ChairID] {
		return
	}
	g.data.ShowCards[u.ChairID] = true
	g.data.HandCards[u.ChairID] = append([]int(nil), g.dealt[u.ChairID]...)
	g.all(push(ShowPush, map[string]any{"chairID": u.ChairID, "cardArray": g.dealt[u.ChairID]}), s)
	if g.doneShow() {
		g.finish(s)
	}
}
func (g *GameFrame) finish(s *remote.Session) {
	g.stop()
	g.data.GameStatus = StatusResult
	g.data.GameStarted = false
	wins := fill(g.data.ChairCount, 0)
	banker := g.data.BankerChairID
	for _, u := range g.players() {
		if u.ChairID == banker {
			continue
		}
		v := g.data.PourScores[u.ChairID]
		if v < 1 {
			v = 1
		}
		baseScore := g.rule.BaseScore
		if baseScore < 1 {
			baseScore = 1
		}
		v *= baseScore * Evaluate(g.dealt[u.ChairID], g.rule.CardsType, g.rule.ScaleType).Scale
		if Compare(g.dealt[banker], g.dealt[u.ChairID], g.rule.CardsType, g.rule.ScaleType) > 0 {
			wins[banker] += v
			wins[u.ChairID] -= v
		} else {
			wins[banker] -= v
			wins[u.ChairID] += v
		}
	}
	g.data.PoolScore += wins[banker]
	clearPool := g.rule.Xiazhuangfen > 0 && g.data.PoolScore < g.rule.Xiazhuangfen
	if clearPool {
		g.data.PoolScore = 0
	}
	g.pendingBanker, g.pendingBankerTurn = g.nextBureauBanker(banker, wins[banker], clearPool)
	result := map[string]any{"poolScore": g.data.PoolScore, "winScores": wins, "shemenArray": append([]bool(nil), g.data.ShemenArray...)}
	g.data.Result = result
	review := make([]*BureauReview, 0, len(g.players()))
	for _, u := range g.players() {
		cardResult := Evaluate(g.dealt[u.ChairID], g.rule.CardsType, g.rule.ScaleType)
		review = append(review, &BureauReview{
			Uid: u.UserInfo.Uid, Nickname: u.UserInfo.Nickname, Avatar: u.UserInfo.Avatar,
			Cards: append([]int(nil), g.dealt[u.ChairID]...), CardType: int(cardResult.Type),
			PourScore: g.data.PourScores[u.ChairID], WinScore: wins[u.ChairID],
			IsBanker: u.ChairID == banker, CurBureau: g.data.CurBureau,
			BankerTurn: g.data.BankerTurn, BankerPourTurn: g.data.BankerPourTurn,
		})
	}
	g.reviewRecord = append(g.reviewRecord, review)
	g.completedBureaus++
	g.data.PourScores = fill(g.data.ChairCount, 0)
	if banker >= 0 && banker < len(g.data.PourScores) {
		g.data.PourScores[banker] = g.data.PoolScore
	}
	g.all(push(ResultPush, map[string]any{"result": result}), s)
	if clearPool {
		g.all(push(ClearPoolPush, map[string]any{"bankerChairID": banker, "poolScore": 0}), s)
	}
	end := make([]*proto.EndData, 0)
	for _, u := range g.players() {
		end = append(end, &proto.EndData{Uid: u.UserInfo.Uid, Score: wins[u.ChairID]})
	}
	g.r.ConcludeGame(end, s)
	if g.r.GetMaxBureau() <= 0 || g.r.GetCurBureau() < g.r.GetMaxBureau() {
		g.scheduleRoundReset(s)
	} else {
		g.scheduleRoomDismiss(s)
	}
}

func (g *GameFrame) scheduleRoomDismiss(s *remote.Session) {
	g.stop()
	var dismiss func()
	dismiss = func() {
		if g.data.GameStatus != StatusResult {
			return
		}
		if g.r.IsDismissing() {
			g.timer = time.AfterFunc(roundResetRetryDelay, func() { g.r.RunGameAction(dismiss) })
			return
		}
		g.r.DismissRoom(s, enums.BureauFinished)
	}
	if g.resultDelay <= 0 {
		dismiss()
		return
	}
	g.timer = time.AfterFunc(g.resultDelay, func() { g.r.RunGameAction(dismiss) })
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
		g.data.ShowCards = make([]bool, g.data.ChairCount)
		g.data.ShemenArray = make([]bool, g.data.ChairCount)
		g.data.NextBankerChairID = -1
		g.dealt = make([][]int, g.data.ChairCount)
		g.choices = fill(g.data.ChairCount, -1)
		g.all(status(StatusNone, 0), s)
	}
	if g.resultDelay <= 0 {
		reset()
		return
	}
	g.timer = time.AfterFunc(g.resultDelay, func() { g.r.RunGameAction(reset) })
}

func (g *GameFrame) nextBureauBanker(current, bankerScore int, poolCleared bool) (int, int) {
	if poolCleared {
		return -1, 0
	}
	users := g.players()
	if len(users) == 0 {
		return -1, 0
	}
	if g.rule.LianzhuangType == 2 {
		return nextValidChair(users, current), 1
	}
	if g.rule.LianzhuangType == 1 && bankerScore > 0 && g.data.BankerTurn < g.rule.LianzhuangCount {
		return current, g.data.BankerTurn + 1
	}
	return -1, 0
}
func (g *GameFrame) auto(u *proto.RoomUser, s *remote.Session) {
	if g.data.GameStatus == StatusPour {
		g.pour(u, g.minimumPourScore(), false, s)
	} else if g.data.GameStatus == StatusShow {
		g.show(u, s)
	}
}

func (g *GameFrame) minimumPourScore() int {
	rate := g.rule.FirstBureauRate
	if g.data.BankerPourTurn > 1 {
		rate = g.rule.BureauRate
	}
	minimum := int(float64(g.initialPool()) * rate)
	if minimum < 1 {
		minimum = 1
	}
	return minimum
}
func (g *GameFrame) schedule(s *remote.Session) {
	g.stop()
	g.timer = time.AfterFunc(g.timeout, func() {
		g.r.RunGameAction(func() {
			if u := g.current(); u != nil {
				if g.data.GameStatus == StatusChooseBank {
					g.choose(u, false, s)
				} else {
					g.auto(u, s)
				}
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
		if g.data.GameStatus == StatusChooseBank && g.choices[u.ChairID] < 0 {
			return u
		}
		if g.data.GameStatus == StatusPour && u.ChairID != g.data.BankerChairID && g.data.PourScores[u.ChairID] == 0 {
			return u
		}
		if g.data.GameStatus == StatusShow && !g.data.ShowCards[u.ChairID] {
			return u
		}
	}
	return nil
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
		if !g.data.ShowCards[u.ChairID] {
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
			a[i] = make([]int, 5)
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
func clone(a [][]int) [][]int {
	r := make([][]int, len(a))
	for i := range a {
		r[i] = append([]int(nil), a[i]...)
	}
	return r
}
func sum(a []int) int {
	n := 0
	for _, v := range a {
		n += v
	}
	return n
}
func (g *GameFrame) all(v any, s *remote.Session) { g.r.SendDataAll(s.GetMsg(), v) }
func (g *GameFrame) to(uid string, v any, s *remote.Session) {
	g.r.SendData(s.GetMsg(), []string{uid}, v)
}
