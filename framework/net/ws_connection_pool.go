package net

import (
	"fmt"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"sync"
	"sync/atomic"
)

// WsConnectionPool 实现WebSocket连接对象池
type WsConnectionPool struct {
	pool      sync.Pool
	count     int32 // 当前池中对象数量
	maxSize   int32 // 最大池大小
	created   int64 // 总共创建的对象数
	reused    int64 // 复用的对象数
	discarded int64 // 丢弃的对象数
}

var (
	// 全局连接池实例
	globalWsConnectionPool *WsConnectionPool
	poolOnce               sync.Once
)

// GetWsConnectionPool 获取全局连接池实例
func GetWsConnectionPool() *WsConnectionPool {
	poolOnce.Do(func() {
		globalWsConnectionPool = NewWsConnectionPool(10000) // 默认最大池大小为10000
	})
	return globalWsConnectionPool
}

// NewWsConnectionPool 创建一个新的连接池
func NewWsConnectionPool(maxSize int32) *WsConnectionPool {
	p := &WsConnectionPool{
		maxSize: maxSize,
	}

	p.pool = sync.Pool{
		New: func() interface{} {
			atomic.AddInt64(&p.created, 1)
			atomic.AddInt32(&p.count, 1)
			return &WsConnection{}
		},
	}

	return p
}

// Get 从池中获取一个连接对象
func (p *WsConnectionPool) Get(conn *websocket.Conn, manager *Manager) *WsConnection {
	wsConn := p.pool.Get().(*WsConnection)
	atomic.AddInt64(&p.reused, 1)

	// 初始化连接对象
	cid := fmt.Sprintf("%s-%s-%d", uuid.New().String(), manager.ServerId, atomic.AddUint64(&cidBase, 1))

	wsConn.Conn = conn
	wsConn.manager = manager
	wsConn.Cid = cid
	wsConn.WriteChan = make(chan []byte, 1024)
	wsConn.ReadChan = manager.ClientReadChan
	wsConn.Session = NewSession(cid, manager)
	wsConn.closeChan = make(chan struct{})

	// 重置同步对象
	wsConn.closeOnce = sync.Once{}
	wsConn.readChanOnce = sync.Once{}
	wsConn.writeChanOnce = sync.Once{}

	return wsConn
}

// Put 将连接对象放回池中
func (p *WsConnectionPool) Put(wsConn *WsConnection) {
	if wsConn == nil {
		return
	}

	// 如果池已满，直接丢弃
	if atomic.LoadInt32(&p.count) > p.maxSize {
		atomic.AddInt64(&p.discarded, 1)
		atomic.AddInt32(&p.count, -1)
		return
	}

	// 重置连接状态
	wsConn.reset()
	p.pool.Put(wsConn)
}

// Stats 获取池统计信息
func (p *WsConnectionPool) Stats() map[string]interface{} {
	return map[string]interface{}{
		"current":   atomic.LoadInt32(&p.count),
		"max":       p.maxSize,
		"created":   atomic.LoadInt64(&p.created),
		"reused":    atomic.LoadInt64(&p.reused),
		"discarded": atomic.LoadInt64(&p.discarded),
	}
}

// reset 重置WsConnection对象的状态
// 这是一个内部方法，不暴露给外部使用
func (c *WsConnection) reset() {
	c.Cid = ""
	c.Conn = nil
	c.manager = nil
	c.ReadChan = nil
	c.WriteChan = nil
	c.Session = nil
	c.pingTicker = nil
	c.closeChan = nil
}
