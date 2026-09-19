package dgn

import "game/component/nn"

type Result = nn.Result

func ruleFromConfig(config map[string]bool, scale int) nn.Rule {
	return nn.Rule{
		ScaleType: nn.ScaleType(scale),
		ShunZiNiu: config["SHUNZINIU"], YinNiu: config["YINNIU"],
		TongHuaNiu: config["TONGHUANIU"], WuHuaNiu: config["WUHUANIU"],
		HuLuNiu: config["HULUNIU"], WuXiaoNiu: config["WUXIAONIU"],
		ZhaDanNiu: config["ZHADANNIU"], YiTiaoLong: config["YITIAOLONG"],
		TongHuaShun: config["TONGHUASHUN"],
	}
}

func Evaluate(cards []int, config map[string]bool, scale int) nn.Result {
	return nn.Evaluate(cards, ruleFromConfig(config, scale))
}

func Compare(left, right []int, config map[string]bool, scale int) int {
	return nn.Compare(left, right, ruleFromConfig(config, scale))
}

func deck() []int { return nnDeck() }
func nnDeck() []int {
	cards := make([]int, 0, 52)
	for suit := 0; suit < 4; suit++ {
		for value := 1; value <= 13; value++ {
			cards = append(cards, suit<<4|value)
		}
	}
	return cards
}
