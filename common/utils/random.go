package utils

import (
	"math"
	"math/rand"
	"time"
)

// RandomRedPacket 生成红包
func RandomRedPacket[T int | int64 | int32](remainMoney T, remainSize int) []T {
	// 初始化随机数生成器
	rand.New(rand.NewSource(time.Now().UnixNano()))
	// 用于存储生成的红包金额
	moneyList := []T{}
	const min = 1 // 最小红包金额为 1 分
	// 生成红包
	for remainSize > 1 {
		// 最大红包金额 = 剩余金额 / 剩余红包数量 * 2
		max := float64(remainMoney) / float64(remainSize) * 2
		// 随机生成红包金额
		money := rand.Float64() * max
		if money < float64(min) {
			money = float64(min)
		}

		// 四舍五入到整数（单位：分）
		moneyInt := T(math.Round(money))

		// 加入到红包列表
		moneyList = append(moneyList, moneyInt)

		// 更新剩余金额和剩余红包个数
		remainSize--
		remainMoney -= moneyInt
	}
	// 最后一个红包金额，直接设置为剩余金额
	moneyList = append(moneyList, remainMoney)

	return moneyList
}
