package mj

import (
	"common/utils"
	"game/component/mj/alg"
	"game/component/mj/mp"
	"sort"
	"sync"
)

const (
	Wan1  mp.CardID = 1
	Wan2  mp.CardID = 2
	Wan3  mp.CardID = 3
	Wan4  mp.CardID = 4
	Wan5  mp.CardID = 5
	Wan6  mp.CardID = 6
	Wan7  mp.CardID = 7
	Wan8  mp.CardID = 8
	Wan9  mp.CardID = 9
	Tong1 mp.CardID = 11
	Tong2 mp.CardID = 12
	Tong3 mp.CardID = 13
	Tong4 mp.CardID = 14
	Tong5 mp.CardID = 15
	Tong6 mp.CardID = 16
	Tong7 mp.CardID = 17
	Tong8 mp.CardID = 18
	Tong9 mp.CardID = 19
	Tiao1 mp.CardID = 21
	Tiao2 mp.CardID = 22
	Tiao3 mp.CardID = 23
	Tiao4 mp.CardID = 24
	Tiao5 mp.CardID = 25
	Tiao6 mp.CardID = 26
	Tiao7 mp.CardID = 27
	Tiao8 mp.CardID = 28
	Tiao9 mp.CardID = 29
	Dong  mp.CardID = 31
	Nan   mp.CardID = 32
	Xi    mp.CardID = 33
	Bei   mp.CardID = 34
	Zhong mp.CardID = 35
)

type Logic struct {
	sync.RWMutex
	cards    []mp.CardID
	gameType GameType
	qidui    bool
	huLogic  *alg.HuLogic
}

func (l *Logic) washCards() {
	l.Lock()
	defer l.Unlock()
	l.cards = []mp.CardID{
		Wan1, Wan2, Wan3, Wan4, Wan5, Wan6, Wan7, Wan8, Wan9,
		Wan1, Wan2, Wan3, Wan4, Wan5, Wan6, Wan7, Wan8, Wan9,
		Wan1, Wan2, Wan3, Wan4, Wan5, Wan6, Wan7, Wan8, Wan9,
		Wan1, Wan2, Wan3, Wan4, Wan5, Wan6, Wan7, Wan8, Wan9,
		Tong1, Tong2, Tong3, Tong4, Tong5, Tong6, Tong7, Tong8, Tong9,
		Tong1, Tong2, Tong3, Tong4, Tong5, Tong6, Tong7, Tong8, Tong9,
		Tong1, Tong2, Tong3, Tong4, Tong5, Tong6, Tong7, Tong8, Tong9,
		Tong1, Tong2, Tong3, Tong4, Tong5, Tong6, Tong7, Tong8, Tong9,
		Tiao1, Tiao2, Tiao3, Tiao4, Tiao5, Tiao6, Tiao7, Tiao8, Tiao9,
		Tiao1, Tiao2, Tiao3, Tiao4, Tiao5, Tiao6, Tiao7, Tiao8, Tiao9,
		Tiao1, Tiao2, Tiao3, Tiao4, Tiao5, Tiao6, Tiao7, Tiao8, Tiao9,
		Tiao1, Tiao2, Tiao3, Tiao4, Tiao5, Tiao6, Tiao7, Tiao8, Tiao9,
		Zhong, Zhong, Zhong, Zhong,
	}
	if l.gameType == HongZhong8 {
		l.cards = append(l.cards, Zhong, Zhong, Zhong, Zhong)
	}
	for i := 0; i < 300; i++ {
		index := i % len(l.cards)
		random := utils.Rand(len(l.cards))
		l.cards[index], l.cards[random] = l.cards[random], l.cards[index]
	}
}

func (l *Logic) getCards(num int) []mp.CardID {
	//发牌之后 牌就没了
	l.Lock()
	defer l.Unlock()
	if len(l.cards) < num {
		return nil
	}
	cards := l.cards[:num]
	l.cards = l.cards[num:]
	return cards
}

func (l *Logic) getRestCardsCount() int {
	return len(l.cards)
}

func (l *Logic) canHu(cards []mp.CardID, card mp.CardID) bool {
	if l.qidui && l.canHuQidui(cards, card) {
		return true
	}
	//胡牌判断 复杂的一套逻辑
	hu := l.huLogic.CheckHu(cards, []mp.CardID{Zhong}, card)
	return hu
}

func (l *Logic) getOperateArray(cards []mp.CardID, outCard mp.CardID) []OperateType {
	operateArray := make([]OperateType, 0)
	sort.Slice(cards, func(i, j int) bool {
		return cards[i] < cards[j]
	})
	//判断碰 已经有两个相同的了 如果 card和这两个一直 能凑成三个一样的
	sameCount := 0
	for _, v := range cards {
		if v == outCard {
			sameCount++
		}
	}
	if sameCount >= 2 {
		operateArray = append(operateArray, Peng)
	}
	if sameCount >= 3 {
		operateArray = append(operateArray, GangChi)
	}
	if l.canHu(cards, outCard) {
		operateArray = append(operateArray, HuChi)
	}
	if len(operateArray) > 0 {
		operateArray = append(operateArray, Guo)
	}
	return operateArray
}

func (l *Logic) getRestCards() []mp.CardID {
	return l.cards
}

func (l *Logic) getCard(card mp.CardID) mp.CardID {
	indexOf := alg.IndexOf(l.cards, card)
	if indexOf == -1 {
		return 0
	}
	l.cards = append(l.cards[:indexOf], l.cards[indexOf+1:]...)
	return card
}

func (l *Logic) getCardCount(cardIDS []mp.CardID, card mp.CardID) int {
	count := 0
	for _, v := range cardIDS {
		if v == card {
			count++
		}
	}
	return count
}

func (l *Logic) canHuQidui(cards []mp.CardID, card mp.CardID) bool {
	return false
}

// 获取玩家所有马牌
func (l *Logic) getMaCardsByChairID() []mp.CardID {
	return []mp.CardID{
		Wan1, Wan5, Wan9, Tiao1, Tiao5, Tiao9, Tong1, Tong5, Tong9, Zhong,
	}
}

func (l *Logic) getHongZhongCount(cards []mp.CardID) int {
	count := 0
	for _, v := range cards {
		if v == Zhong {
			count++
		}
	}
	return count
}

func NewLogic(gameType GameType, qidui bool) *Logic {
	return &Logic{
		gameType: gameType,
		qidui:    qidui,
		huLogic:  alg.NewHuLogic(),
	}
}
