package alg

import (
	"strconv"
)

// Table 字典表
// 1A-9A B C 非字牌 一样 生成一种就行 D生成一种（没有连子的）
// 穷举所有胡牌可能 存入Table0  D TableFeng0
// 有1个鬼 将Table0中的数据 替换一个 然后存入 Table1
// 有2个鬼 将Table0中的数据 替换2个 然后存入 Table2
// 有3个鬼 将Table0中的数据 替换3个 然后存入 Table3
// 有4个鬼 将Table0中的数据 替换4个 然后存入 Table4
// 有5个鬼 将Table0中的数据 替换5个 然后存入 Table5
// 有6个鬼 将Table0中的数据 替换6个 然后存入 Table6
// 有7个鬼 将Table0中的数据 替换7个 然后存入 Table7
type Table struct {
	keyDic        map[string]bool         //非字牌ABC 无鬼 字典
	keyGuiDic     map[int]map[string]bool //非字牌ABC 有鬼 字典
	keyFengDic    map[string]bool         //字牌D 无鬼 字典
	keyFengGuiDic map[int]map[string]bool //字牌D 有鬼 字典
}

func NewTable() *Table {
	t := &Table{
		keyDic:        make(map[string]bool),
		keyGuiDic:     make(map[int]map[string]bool),
		keyFengDic:    make(map[string]bool),
		keyFengGuiDic: make(map[int]map[string]bool),
	}
	t.gen()
	return t
}

// gen 生成字典
func (t *Table) gen() {
	//311100000 一种hu可能
	cards := []int{0, 0, 0, 0, 0, 0, 0, 0, 0}
	level := 0
	jiang := false
	feng := false
	t.genTableNoGui(cards, level, jiang, feng)
	//挨个去替换即可
	t.genTableGui(feng)
	feng = true
	t.genTableNoGui(cards, level, jiang, feng)
	t.genTableGui(feng)
	t.save()
}

func (t *Table) genTableNoGui(cards []int, level int, jiang bool, feng bool) {
	//生成的逻辑 比较简单
	//ABC
	//挨个去放 AAA组合 刻子   14张  3+3+3+2+3 5个层级  最多递归5次就可以
	//ABC 连子
	//DD 将 这个只能有一个 jiang=true
	for i := 0; i < 9; i++ {
		//feng 东南西北中发白 7张牌*4  春夏秋冬梅兰竹菊 没用的牌
		if feng && i > 6 {
			continue
		}
		//1. 需要先将cards中的牌数量计算出来，后续做判断用
		totalCardCount := t.calTotalCardCount(cards)
		//AAA 刻子
		if totalCardCount <= 11 && cards[i] <= 1 {
			cards[i] += 3
			key := t.generateKey(cards)
			if feng {
				t.keyFengDic[key] = true
			} else {
				t.keyDic[key] = true
			}
			if level < 5 {
				t.genTableNoGui(cards, level+1, jiang, feng)
			}
			cards[i] -= 3
		}
		//连子 ABC
		if !feng && totalCardCount <= 11 && i < 7 && cards[i] <= 3 &&
			cards[i+1] <= 3 && cards[i+2] <= 3 {
			cards[i] += 1
			cards[i+1] += 1
			cards[i+2] += 1
			key := t.generateKey(cards)
			t.keyDic[key] = true
			if level < 5 {
				t.genTableNoGui(cards, level+1, jiang, feng)
			}
			cards[i] -= 1
			cards[i+1] -= 1
			cards[i+2] -= 1
		}
		//DD 将
		if !jiang && totalCardCount <= 12 && cards[i] <= 2 {
			cards[i] += 2
			key := t.generateKey(cards)
			if feng {
				t.keyFengDic[key] = true
			} else {
				t.keyDic[key] = true
			}
			if level < 5 {
				t.genTableNoGui(cards, level+1, true, feng)
			}
			cards[i] -= 2
		}
	}
}

func (t *Table) calTotalCardCount(cards []int) int {
	count := 0
	for _, v := range cards {
		count += v
	}
	return count
}

func (t *Table) generateKey(cards []int) string {
	key := ""
	dic := []string{"0", "1", "2", "3", "4"}
	for _, v := range cards {
		key += dic[v]
	}
	return key
}

func (t *Table) save() {
	//TODO
}

func (t *Table) genTableGui(feng bool) {
	dic := t.keyDic
	if feng {
		dic = t.keyFengDic
	}
	//311200000  3 1 1 2 分表 3-1   211200000
	for k := range dic {
		cards := t.toNumberArray(k)
		t.genGui(cards, 1, feng)
	}
}

func (t *Table) toNumberArray(k string) []int {
	cards := make([]int, len(k))
	for i := 0; i < len(k); i++ {
		card, _ := strconv.Atoi(k[i : i+1])
		cards[i] = card
	}
	return cards
}

func (t *Table) genGui(cards []int, guiCount int, feng bool) {
	for i := 0; i < 9; i++ {
		if cards[i] == 0 {
			//此位置没有牌
			continue
		}
		cards[i]--
		//需要判断其有没有 有就不处理 没有就添加到字典中
		if !t.tryAdd(cards, guiCount, feng) {
			cards[i]++
			continue
		}
		if guiCount < 8 {
			t.genGui(cards, guiCount+1, feng)
		}
		cards[i]++
	}
}

func (t *Table) tryAdd(cards []int, guiCount int, feng bool) bool {
	for i := 0; i < 9; i++ {
		if cards[i] < 0 || cards[i] > 4 {
			return false
		}
	}
	key := t.generateKey(cards)
	if feng {
		if t.keyFengGuiDic[guiCount] == nil {
			t.keyFengGuiDic[guiCount] = make(map[string]bool)
		}
		_, ok := t.keyFengGuiDic[guiCount][key]
		if ok {
			return false
		}
		t.keyFengGuiDic[guiCount][key] = true
		return true
	}
	if t.keyGuiDic[guiCount] == nil {
		t.keyGuiDic[guiCount] = make(map[string]bool)
	}
	_, ok := t.keyGuiDic[guiCount][key]
	if ok {
		return false
	}
	t.keyGuiDic[guiCount][key] = true
	return true
}

func (t *Table) findCards(cards []int, guiCount int, feng bool) bool {
	//先编码
	key := t.generateKey(cards)
	if guiCount > 0 {
		if feng {
			_, ok := t.keyFengGuiDic[guiCount][key]
			if ok {
				return true
			}
		} else {
			_, ok := t.keyGuiDic[guiCount][key]
			if ok {
				return true
			}
		}
	} else {
		if feng {
			_, ok := t.keyFengDic[key]
			if ok {
				return true
			}
		} else {
			_, ok := t.keyDic[key]
			if ok {
				return true
			}
		}
	}
	return false
}
