package utils

import (
	"errors"
	"math/rand"
	"time"
)

func Contains[T int | string](data []T, value T) bool {
	for _, v := range data {
		if v == value {
			return true
		}
	}
	return false
}

func Rand(n int) int {
	rand.New(rand.NewSource(time.Now().UnixNano()))
	return rand.Intn(n)
}

func IndexOf[T int | string](data []T, value T) int {
	for index, v := range data {
		if v == value {
			return index
		}
	}
	return -1
}

// Splice splice 函数：
// 用于删除或插入切片中的元素
// splice(slice, index, count, elements) -> 返回修改后的切片
func Splice(slice []int, index int, count int, elements ...int) []int {
	// 删除 count 个元素
	if count > 0 {
		// 判断 index 是否越界
		if index < 0 || index >= len(slice) {
			return slice
		}
		// 删除元素：将切片前后两部分合并
		slice = append(slice[:index], slice[index+count:]...)
	}

	// 插入元素
	if len(elements) > 0 {
		// 判断 index 是否越界
		if index < 0 || index > len(slice) {
			return slice
		}
		// 将插入的元素插入到切片指定位置
		slice = append(slice[:index], append(elements, slice[index:]...)...)
	}

	return slice
}

// Pop 函数：返回切片的最后一个元素，并删除它
func Pop[T int | string](slice []T) T {
	if len(slice) == 0 {
		var zeroValue T
		// 如果切片为空，返回零值和空切片
		return zeroValue
	}
	// 获取最后一个元素并返回，同时更新切片
	lastElement := slice[len(slice)-1]
	slice = slice[:len(slice)-1]
	return lastElement
}

// Shift 函数移除切片中的第一个元素并返回它，返回更新后的切片
func Shift[T int | string | int32](slice []T) (T, []T, error) {
	// 检查切片是否为空
	if len(slice) == 0 {
		var zeroValue T
		return zeroValue, slice, errors.New("slice is empty")
	}
	// 获取第一个元素
	shiftedElement := slice[0]
	// 返回去掉第一个元素后的切片
	remainingSlice := slice[1:]

	return shiftedElement, remainingSlice, nil
}

// GetTimeTodayStart 返回当天零点的时间戳（毫秒）
func GetTimeTodayStart() int64 {
	now := time.Now()
	// 获取当天零点的时间
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	// 返回时间戳，单位为毫秒
	return todayStart.UnixNano() / int64(time.Millisecond)
}
