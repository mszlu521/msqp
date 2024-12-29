package mj

import "game/component/mj/mp"

func IndexOf[T OperateType | mp.CardID | int](data []T, value T) int {
	for index, v := range data {
		if v == value {
			return index
		}
	}
	return -1
}
func Splice[T OperateType | int](slice []T, index int, count int, elements ...T) []T {
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
