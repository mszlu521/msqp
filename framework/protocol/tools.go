package protocol

import "sync"

// 预分配的字节缓冲池，用于减少内存分配
var bytesPool = sync.Pool{
	New: func() interface{} {
		return make([]byte, 3)
	},
}

// IntToBytes Encode packet data length to bytes(Big end)
// 优化: 使用对象池减少内存分配
func IntToBytes(n int) []byte {
	buf := bytesPool.Get().([]byte)
	buf[0] = byte((n >> 16) & 0xFF)
	buf[1] = byte((n >> 8) & 0xFF)
	buf[2] = byte(n & 0xFF)

	// 创建一个新的切片返回，以避免对象池中的对象被修改
	result := make([]byte, 3)
	copy(result, buf)

	// 将buf放回对象池
	bytesPool.Put(buf)
	return result
}

// BytesToInt Decode packet data length byte to int(Big end)
// 优化: 直接使用位运算，避免循环
func BytesToInt(b []byte) int {
	if len(b) >= 3 {
		return int(b[0])<<16 | int(b[1])<<8 | int(b[2])
	}

	// 兼容处理，如果长度不足3
	result := 0
	for _, v := range b {
		result = result<<8 + int(v)
	}
	return result
}
