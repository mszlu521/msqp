package net

import (
	"common/logs"
	"common/utils"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"framework/game"
	"framework/protocol"
	"framework/remote"
	"framework/stream"
	"github.com/gorilla/websocket"
	"hash/fnv"
	"math/rand"
	"net/http"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var (
	// 优化websocket连接配置
	websocketUpgrade = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
		ReadBufferSize:    4096, // 增加缓冲区大小
		WriteBufferSize:   4096, // 增加缓冲区大小
		EnableCompression: true, // 启用压缩
	}

	// 连接限流配置
	connectionRateLimiter = utils.NewRateLimiter(100, 1) // 每秒最多100个新连接
)

// 用于计算哈希值的辅助函数
func fnv32(key string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(key))
	return h.Sum32()
}

// ClientBucket 客户端连接分片
type ClientBucket struct {
	sync.RWMutex
	clients map[string]Connection
}

// NewClientBucket 创建新的客户端连接分片
func NewClientBucket() *ClientBucket {
	return &ClientBucket{
		clients: make(map[string]Connection),
	}
}

type CheckOriginHandler func(r *http.Request) bool
type Manager struct {
	// 移除全局锁，使用分片锁
	dataLock           sync.RWMutex // 仅用于保护data字段
	websocketUpgrade   *websocket.Upgrader
	ServerId           string
	CheckOriginHandler CheckOriginHandler

	// 分片存储客户端连接
	clientBuckets []*ClientBucket
	bucketMask    uint32

	// 工作池相关
	ClientReadChan chan *MsgPack
	clientWorkers  []chan *MsgPack // 工作协程池
	workerCount    int             // 工作协程数量

	handlers          map[protocol.PackageType]EventHandler
	ConnectorHandlers LogicHandler

	// 远程消息处理
	RemoteReadChan chan []byte
	RemoteCli      remote.Client
	RemotePushChan chan *stream.Msg

	// 共享数据
	data map[string]any

	// 连接限制
	maxConnections int           // 最大连接数
	connSemaphore  chan struct{} // 连接信号量

	// 性能统计
	stats struct {
		messageProcessed   int64
		messageErrors      int64
		avgProcessingTime  int64
		currentConnections int32
	}

	// 负载均衡状态
	lbState loadBalanceState
}
type HandlerFunc func(session *Session, body []byte) (any, error)
type LogicHandler map[string]HandlerFunc
type EventHandler func(packet *protocol.Packet, c Connection) error

func (m *Manager) Run(addr string) {
	// 启动工作协程池
	for i := 0; i < m.workerCount; i++ {
		go m.clientWorkerRoutine(i)
	}

	go m.clientReadChanHandler()
	go m.remoteReadChanHandler()
	go m.remotePushChanHandler()

	// 启动性能监控
	go m.monitorPerformance()

	http.HandleFunc("/", m.serveWS)
	//设置不同的消息处理器
	m.setupEventHandlers()
	logs.Info("WebSocket manager started with %d worker goroutines and %d connection buckets",
		m.workerCount, len(m.clientBuckets))
	logs.Fatal("connector listen serve err:%v", http.ListenAndServe(addr, nil))
}

// 工作协程处理消息
func (m *Manager) clientWorkerRoutine(workerID int) {
	for msg := range m.clientWorkers[workerID] {
		startTime := time.Now()
		m.decodeClientPack(msg)
		processingTime := time.Since(startTime).Microseconds()

		// 更新统计信息
		atomic.AddInt64(&m.stats.messageProcessed, 1)
		// 使用指数移动平均更新处理时间
		oldAvg := atomic.LoadInt64(&m.stats.avgProcessingTime)
		newAvg := (oldAvg*9 + processingTime) / 10 // 90%旧值，10%新值
		atomic.StoreInt64(&m.stats.avgProcessingTime, newAvg)
	}
}

// 性能监控
func (m *Manager) monitorPerformance() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		logs.Info("Performance stats: connections=%d, messages_processed=%d, avg_processing_time=%dμs, errors=%d",
			atomic.LoadInt32(&m.stats.currentConnections),
			atomic.LoadInt64(&m.stats.messageProcessed),
			atomic.LoadInt64(&m.stats.avgProcessingTime),
			atomic.LoadInt64(&m.stats.messageErrors))
	}
}

func (m *Manager) serveWS(w http.ResponseWriter, r *http.Request) {
	// 连接限流
	if !connectionRateLimiter.Allow() {
		http.Error(w, "Too many connections", http.StatusTooManyRequests)
		logs.Warn("Connection rate limit exceeded from %s", r.RemoteAddr)
		return
	}

	// 检查当前连接数是否已达上限
	if atomic.LoadInt32(&m.stats.currentConnections) >= int32(m.maxConnections) {
		http.Error(w, "Server is at capacity", http.StatusServiceUnavailable)
		logs.Warn("Connection limit reached, rejecting connection from %s", r.RemoteAddr)
		return
	}

	// 设置连接超时
	var upgrader *websocket.Upgrader
	if m.websocketUpgrade == nil {
		// 创建一个带有超时设置的upgrader
		upgrader = &websocketUpgrade
	} else {
		upgrader = m.websocketUpgrade
	}

	// 设置响应头
	header := w.Header()
	header.Add("Server", "MSQP-WebSocket-Server")

	// 记录连接信息
	logs.Debug("WebSocket connection attempt from %s, User-Agent: %s",
		r.RemoteAddr, r.UserAgent())

	// 升级HTTP连接为WebSocket
	wsConn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logs.Error("websocketUpgrade.Upgrade err:%v from %s", err, r.RemoteAddr)
		return
	}

	// 设置读写超时
	wsConn.SetReadDeadline(time.Now().Add(120 * time.Second))
	wsConn.SetWriteDeadline(time.Now().Add(10 * time.Second))

	// 创建客户端连接
	client := NewWsConnection(wsConn, m)

	// 记录连接成功
	logs.Debug("WebSocket connection established: %s from %s", client.Cid, r.RemoteAddr)

	// 添加客户端并启动
	m.addClient(client)
	client.Run()
}

// SetConnectionRateLimit 设置连接速率限制
func (m *Manager) SetConnectionRateLimit(connectionsPerSecond int) {
	connectionRateLimiter = utils.NewRateLimiter(connectionsPerSecond, 1)
	logs.Info("Connection rate limit set to %d per second", connectionsPerSecond)
}

// BroadcastToAll 向所有连接的客户端广播消息
func (m *Manager) BroadcastToAll(messageType protocol.PackageType, data []byte) {
	// 编码消息
	res, err := protocol.Encode(messageType, data)
	if err != nil {
		logs.Error("BroadcastToAll encode error: %v", err)
		return
	}

	// 并行处理每个分片
	var wg sync.WaitGroup
	for _, bucket := range m.clientBuckets {
		wg.Add(1)
		go func(b *ClientBucket) {
			defer wg.Done()

			// 获取分片中的所有连接
			b.RLock()
			connections := make([]Connection, 0, len(b.clients))
			for _, conn := range b.clients {
				connections = append(connections, conn)
			}
			b.RUnlock()

			// 向每个连接发送消息
			for _, conn := range connections {
				conn.SendMessage(res)
			}
		}(bucket)
	}

	// 等待所有分片处理完成
	wg.Wait()
	logs.Info("Broadcast message sent to all clients")
}

// 获取连接所在的分片
func (m *Manager) getBucket(cid string) *ClientBucket {
	hash := fnv32(cid)
	index := hash & m.bucketMask
	return m.clientBuckets[index]
}

func (m *Manager) addClient(client *WsConnection) {
	// 使用分片锁
	bucket := m.getBucket(client.Cid)

	select {
	case m.connSemaphore <- struct{}{}:
		// 允许新连接
		bucket.Lock()
		bucket.clients[client.Cid] = client
		bucket.Unlock()

		// 设置会话数据
		m.dataLock.RLock()
		client.GetSession().SetAll(m.data)
		m.dataLock.RUnlock()

		// 更新统计信息
		atomic.AddInt32(&m.stats.currentConnections, 1)
		return
	default:
		// 连接数已达上限
		logs.Warn("Connection limit reached, rejecting new connection")
		client.Close()
		return
	}
}

func (m *Manager) removeClient(wc *WsConnection) {
	bucket := m.getBucket(wc.Cid)

	bucket.Lock()
	if _, exists := bucket.clients[wc.Cid]; exists {
		// 先从map中删除，避免其他地方再次访问
		delete(bucket.clients, wc.Cid)
		bucket.Unlock()

		// 关闭连接
		wc.Close()

		// 释放连接槽位
		<-m.connSemaphore

		// 更新统计信息
		atomic.AddInt32(&m.stats.currentConnections, -1)
	} else {
		bucket.Unlock()
	}
}

func (m *Manager) clientReadChanHandler() {
	for body := range m.ClientReadChan {
		// 根据连接ID分配到特定工作协程
		hash := fnv32(body.Cid)
		workerID := hash % uint32(m.workerCount)
		select {
		case m.clientWorkers[workerID] <- body:
			// 消息已分发到工作协程
		default:
			// 工作协程队列已满，记录错误并尝试直接处理
			atomic.AddInt64(&m.stats.messageErrors, 1)
			logs.Warn("Worker queue %d full, processing message in main goroutine", workerID)
			go m.decodeClientPack(body) // 使用新的goroutine避免阻塞
		}
	}
}

func (m *Manager) decodeClientPack(body *MsgPack) {
	//解析协议
	packet, err := protocol.Decode(body.Body)
	if err != nil {
		atomic.AddInt64(&m.stats.messageErrors, 1)
		logs.Error("decode stream err:%v", err)
		return
	}
	if err := m.routeEvent(packet, body.Cid); err != nil {
		atomic.AddInt64(&m.stats.messageErrors, 1)
		logs.Error("routeEvent err:%v", err)
	}
}

func (m *Manager) Close() {
	// 使用多个goroutine并行关闭连接
	var wg sync.WaitGroup

	for i, bucket := range m.clientBuckets {
		wg.Add(1)
		go func(b *ClientBucket, bucketID int) {
			defer wg.Done()

			b.Lock()
			clients := make([]Connection, 0, len(b.clients))
			for _, client := range b.clients {
				clients = append(clients, client)
			}
			// 清空map
			for cid := range b.clients {
				delete(b.clients, cid)
			}
			b.Unlock()

			// 关闭连接
			for _, client := range clients {
				client.Close()
				// 不需要从connSemaphore中取出，因为整个Manager都要关闭了
			}

			logs.Info("Closed %d connections in bucket %d", len(clients), bucketID)
		}(bucket, i)
	}

	wg.Wait()
	logs.Info("All connections closed")
}

func (m *Manager) routeEvent(packet *protocol.Packet, cid string) error {
	// 根据packet.type来做不同的处理
	bucket := m.getBucket(cid)

	bucket.RLock()
	conn, ok := bucket.clients[cid]
	bucket.RUnlock()

	if !ok {
		return errors.New("no client found")
	}

	handler, ok := m.handlers[packet.Type]
	if !ok {
		return errors.New("no packetType found")
	}

	return handler(packet, conn)
}

func (m *Manager) setupEventHandlers() {
	m.handlers[protocol.Handshake] = m.HandshakeHandler
	m.handlers[protocol.HandshakeAck] = m.HandshakeAckHandler
	m.handlers[protocol.Heartbeat] = m.HeartbeatHandler
	m.handlers[protocol.Data] = m.MessageHandler
	m.handlers[protocol.Kick] = m.KickHandler
}

func (m *Manager) HandshakeHandler(packet *protocol.Packet, c Connection) error {
	res := protocol.HandshakeResponse{
		Code: 200,
		Sys: protocol.Sys{
			Heartbeat: 3,
		},
	}
	data, _ := json.Marshal(res)
	buf, err := protocol.Encode(packet.Type, data)
	if err != nil {
		logs.Error("encode packet err:%v", err)
		return err
	}
	return c.SendMessage(buf)
}

func (m *Manager) HandshakeAckHandler(packet *protocol.Packet, c Connection) error {
	return nil
}

func (m *Manager) HeartbeatHandler(packet *protocol.Packet, c Connection) error {
	var res []byte
	data, _ := json.Marshal(res)
	buf, err := protocol.Encode(packet.Type, data)
	if err != nil {
		logs.Error("encode packet err:%v", err)
		return err
	}
	return c.SendMessage(buf)
}

func (m *Manager) MessageHandler(packet *protocol.Packet, c Connection) error {
	message := packet.MessageBody()
	//connector.entryHandler.entry
	routeStr := message.Route
	routers := strings.Split(routeStr, ".")
	if len(routers) != 3 {
		return errors.New("router unsupported")
	}
	serverType := routers[0]
	handlerMethod := fmt.Sprintf("%s.%s", routers[1], routers[2])
	connectorConfig := game.Conf.GetConnectorByServerType(serverType)
	if connectorConfig != nil {
		//本地connector服务器处理
		handler, ok := m.ConnectorHandlers[handlerMethod]
		if ok {
			data, err := handler(c.GetSession(), message.Data)
			if err != nil {
				return err
			}
			marshal, _ := json.Marshal(data)
			message.Type = protocol.Response
			message.Data = marshal
			encode, err := protocol.MessageEncode(message)
			if err != nil {
				return err
			}
			res, err := protocol.Encode(packet.Type, encode)
			if err != nil {
				return err
			}
			return c.SendMessage(res)
		}
	} else {
		//nats 远端调用处理 hall.userHandler.updateUserAddress
		dst, err := m.selectDst(serverType)
		if err != nil {
			logs.Error("remote send stream selectDst err:%v", err)
			return err
		}
		msg := &stream.Msg{
			Cid:         c.GetSession().Cid,
			Uid:         c.GetSession().Uid,
			Src:         m.ServerId,
			ConnectorId: m.ServerId,
			Dst:         dst,
			Router:      handlerMethod,
			Body:        message,
			SessionData: &stream.SessionData{
				SingleData: c.GetSession().data,
				AllData:    c.GetSession().all,
			},
		}
		data, _ := json.Marshal(msg)
		logs.Warn("remote send stream:%s", string(msg.Body.Data))
		err = m.RemoteCli.SendMsg(dst, data)
		if err != nil {
			logs.Error("remote send stream err：%v", err)
			return err
		}
	}
	return nil
}

func (m *Manager) KickHandler(packet *protocol.Packet, c Connection) error {
	return nil
}

func (m *Manager) remoteReadChanHandler() {
	const batchSize = 32
	batch := make([][]byte, 0, batchSize)

	processBatch := func() {
		if len(batch) == 0 {
			return
		}

		// 并行处理批次中的消息
		var wg sync.WaitGroup
		for _, body := range batch {
			wg.Add(1)
			go func(msgBody []byte) {
				defer wg.Done()

				var msg stream.Msg
				if err := json.Unmarshal(msgBody, &msg); err != nil {
					logs.Error("nats remote stream format err:%v", err)
					return
				}

				if msg.SessionType == stream.Session {
					//需要特出处理，session类型是存储在connection中的session 并不 推送客户端
					m.setSessionData(msg)
					return
				}

				if msg.Body != nil {
					if msg.Body.Type == protocol.Request || msg.Body.Type == protocol.Response {
						//给客户端回信息 都是 response
						msg.Body.Type = protocol.Response
						m.Response(&msg)
					}
					if msg.Body.Type == protocol.Push {
						select {
						case m.RemotePushChan <- &msg:
							// 成功发送到推送通道
						default:
							// 通道已满，直接处理
							if msg.Body.Type == protocol.Push {
								m.Response(&msg)
							}
						}
					}
				}
			}(body)
		}

		// 等待所有消息处理完成
		wg.Wait()
		batch = batch[:0] // 清空批次
	}

	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case body, ok := <-m.RemoteReadChan:
			if !ok {
				// 通道已关闭
				processBatch() // 处理剩余消息
				return
			}

			batch = append(batch, body)
			if len(batch) >= batchSize {
				processBatch()
			}

		case <-ticker.C:
			processBatch()
		}
	}
}

// LoadBalanceStrategy 定义负载均衡策略类型
type LoadBalanceStrategy int

const (
	// Random 随机选择策略
	Random LoadBalanceStrategy = iota
	// RoundRobin 轮询策略
	RoundRobin
	// WeightedRoundRobin 加权轮询策略
	WeightedRoundRobin
	// LeastConnection 最少连接策略
	LeastConnection
	// ConsistentHash 一致性哈希策略
	ConsistentHash
	// IPHash IP哈希策略
	IPHash
)

// 负载均衡状态
type loadBalanceState struct {
	strategy      LoadBalanceStrategy
	roundRobinIdx map[string]int             // 每种服务器类型的轮询索引
	hashRing      map[string]*consistentHash // 每种服务器类型的一致性哈希环
	serverLoads   map[string]int             // 服务器负载计数
	mu            sync.RWMutex               // 保护状态的互斥锁
}

// consistentHash 简单的一致性哈希实现
type consistentHash struct {
	hashRing []uint32          // 排序的哈希环
	mapping  map[uint32]string // 哈希值到服务器ID的映射
}

func (m *Manager) selectDst(serverType string) (string, error) {
	serversConfigs, ok := game.Conf.ServersConf.TypeServer[serverType]
	if !ok {
		return "", errors.New("no server found")
	}

	if len(serversConfigs) == 0 {
		return "", errors.New("no available servers")
	}

	// 如果只有一个服务器，直接返回
	if len(serversConfigs) == 1 {
		return serversConfigs[0].ID, nil
	}

	// 根据选择的负载均衡策略选择服务器
	m.lbState.mu.RLock()
	strategy := m.lbState.strategy
	m.lbState.mu.RUnlock()

	var serverID string
	//var err error

	switch strategy {
	case Random:
		serverID = m.selectRandomServer(serversConfigs)
	case RoundRobin:
		serverID = m.selectRoundRobinServer(serverType, serversConfigs)
	case WeightedRoundRobin:
		serverID = m.selectWeightedRoundRobinServer(serversConfigs)
	case LeastConnection:
		serverID = m.selectLeastConnectionServer(serversConfigs)
	case ConsistentHash:
		// 对于一致性哈希，我们需要一个键（通常是用户ID）
		// 这里我们尝试从当前上下文获取用户ID，如果没有则回退到随机选择
		uid := m.getCurrentUserID()
		if uid != "" {
			serverID = m.selectConsistentHashServer(serverType, uid, serversConfigs)
		} else {
			serverID = m.selectRandomServer(serversConfigs)
			logs.Debug("No user ID available for consistent hash, falling back to random selection")
		}
	case IPHash:
		// 对于IP哈希，我们需要客户端IP
		// 这里我们尝试从当前上下文获取客户端IP，如果没有则回退到随机选择
		clientIP := m.getCurrentClientIP()
		if clientIP != "" {
			serverID = m.selectIPHashServer(serverType, clientIP, serversConfigs)
		} else {
			serverID = m.selectRandomServer(serversConfigs)
			logs.Debug("No client IP available for IP hash, falling back to random selection")
		}
	default:
		// 默认使用随机选择
		serverID = m.selectRandomServer(serversConfigs)
	}

	// 记录服务器选择信息，便于后续分析
	logs.Debug("Selected server %s for type %s using strategy %v", serverID, serverType, strategy)

	return serverID, nil
}

// selectRandomServer 随机选择服务器
func (m *Manager) selectRandomServer(servers []*game.ServersConfig) string {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	index := r.Intn(len(servers))
	return servers[index].ID
}

// selectRoundRobinServer 轮询选择服务器
func (m *Manager) selectRoundRobinServer(serverType string, servers []*game.ServersConfig) string {
	m.lbState.mu.Lock()
	defer m.lbState.mu.Unlock()

	// 初始化该服务器类型的轮询索引（如果不存在）
	if _, exists := m.lbState.roundRobinIdx[serverType]; !exists {
		m.lbState.roundRobinIdx[serverType] = 0
	}

	// 获取当前索引并更新
	index := m.lbState.roundRobinIdx[serverType]
	m.lbState.roundRobinIdx[serverType] = (index + 1) % len(servers)

	return servers[index].ID
}

// selectWeightedRoundRobinServer 加权轮询选择服务器
func (m *Manager) selectWeightedRoundRobinServer(servers []*game.ServersConfig) string {
	// 这里我们假设ServerConfig中有一个Weight字段
	// 如果没有，我们可以根据服务器的其他属性计算权重

	// 简单实现：使用服务器配置中的某个属性作为权重
	// 这里我们假设所有服务器权重相等，实际应用中可以根据需要修改
	totalWeight := len(servers)
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	randomWeight := r.Intn(totalWeight)

	// 选择权重对应的服务器
	currentWeight := 0
	for _, server := range servers {
		currentWeight++
		if currentWeight > randomWeight {
			return server.ID
		}
	}

	// 默认返回第一个服务器
	return servers[0].ID
}

// selectLeastConnectionServer 选择连接数最少的服务器
func (m *Manager) selectLeastConnectionServer(servers []*game.ServersConfig) string {
	m.lbState.mu.RLock()
	defer m.lbState.mu.RUnlock()

	minLoad := -1
	var selectedServer string

	for _, server := range servers {
		load, exists := m.lbState.serverLoads[server.ID]
		if !exists || (minLoad == -1 || load < minLoad) {
			minLoad = load
			selectedServer = server.ID
		}
	}

	return selectedServer
}

// selectConsistentHashServer 使用一致性哈希选择服务器
func (m *Manager) selectConsistentHashServer(serverType string, key string, servers []*game.ServersConfig) string {
	m.lbState.mu.Lock()
	defer m.lbState.mu.Unlock()

	// 初始化该服务器类型的哈希环（如果不存在）
	if _, exists := m.lbState.hashRing[serverType]; !exists {
		m.initConsistentHash(serverType, servers)
	}

	// 计算键的哈希值
	h := fnv.New32a()
	h.Write([]byte(key))
	keyHash := h.Sum32()

	// 在哈希环上查找服务器
	hashRing := m.lbState.hashRing[serverType]
	if hashRing == nil || len(hashRing.hashRing) == 0 {
		// 如果哈希环为空，回退到随机选择
		return m.selectRandomServer(servers)
	}

	// 二分查找大于等于keyHash的第一个点
	idx := sort.Search(len(hashRing.hashRing), func(i int) bool {
		return hashRing.hashRing[i] >= keyHash
	})

	// 如果没有找到，则使用第一个点（环状结构）
	if idx == len(hashRing.hashRing) {
		idx = 0
	}

	// 返回对应的服务器ID
	return hashRing.mapping[hashRing.hashRing[idx]]
}

// selectIPHashServer 使用IP哈希选择服务器
func (m *Manager) selectIPHashServer(serverType string, ip string, servers []*game.ServersConfig) string {
	// IP哈希实际上是一致性哈希的特例，使用IP作为键
	return m.selectConsistentHashServer(serverType, ip, servers)
}

// initConsistentHash 初始化一致性哈希环
func (m *Manager) initConsistentHash(serverType string, servers []*game.ServersConfig) {
	// 为每个服务器创建多个虚拟节点以提高均衡性
	const virtualNodes = 100
	hashRing := &consistentHash{
		hashRing: make([]uint32, 0, len(servers)*virtualNodes),
		mapping:  make(map[uint32]string),
	}

	for _, server := range servers {
		for i := 0; i < virtualNodes; i++ {
			// 为每个服务器创建多个虚拟节点
			key := fmt.Sprintf("%s-%d", server.ID, i)
			h := fnv.New32a()
			h.Write([]byte(key))
			hash := h.Sum32()

			hashRing.hashRing = append(hashRing.hashRing, hash)
			hashRing.mapping[hash] = server.ID
		}
	}

	// 排序哈希环
	sort.Slice(hashRing.hashRing, func(i, j int) bool {
		return hashRing.hashRing[i] < hashRing.hashRing[j]
	})

	// 保存哈希环
	m.lbState.hashRing[serverType] = hashRing
}

// 上下文键，用于在请求上下文中存储用户ID和客户端IP
var (
	userIDContextKey   = struct{}{}
	clientIPContextKey = struct{}{}
	currentContext     context.Context // 当前请求上下文
)

// getCurrentUserID 获取当前上下文中的用户ID
func (m *Manager) getCurrentUserID() string {
	// 尝试从当前上下文中获取用户ID
	if currentContext != nil {
		if uid, ok := currentContext.Value(userIDContextKey).(string); ok && uid != "" {
			return uid
		}
	}

	// 如果无法从上下文中获取，尝试从当前处理的消息中获取
	// 这需要在消息处理过程中设置一个线程本地变量或全局变量
	// 在实际应用中，可能需要根据具体的消息处理流程来实现

	// 这里我们可以尝试从当前正在处理的连接中获取用户ID
	// 注意：这种方法只在处理特定连接的消息时有效
	// 在其他情况下（如定时任务、系统事件等），可能无法获取用户ID

	return ""
}

// getCurrentClientIP 获取当前上下文中的客户端IP
func (m *Manager) getCurrentClientIP() string {
	// 尝试从当前上下文中获取客户端IP
	if currentContext != nil {
		if ip, ok := currentContext.Value(clientIPContextKey).(string); ok && ip != "" {
			return ip
		}
	}

	// 如果无法从上下文中获取，尝试从当前处理的连接中获取
	// 在实际应用中，WebSocket连接通常会记录客户端的IP地址

	return ""
}

// SetRequestContext 设置当前请求上下文
// 这个方法应该在处理每个请求之前调用，以便在负载均衡过程中使用上下文信息
func (m *Manager) SetRequestContext(ctx context.Context) {
	currentContext = ctx
}

// WithUserID 创建一个包含用户ID的上下文
func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDContextKey, userID)
}

// WithClientIP 创建一个包含客户端IP的上下文
func WithClientIP(ctx context.Context, clientIP string) context.Context {
	return context.WithValue(ctx, clientIPContextKey, clientIP)
}

// SetLoadBalanceStrategy 设置负载均衡策略
func (m *Manager) SetLoadBalanceStrategy(strategy LoadBalanceStrategy) {
	m.lbState.mu.Lock()
	defer m.lbState.mu.Unlock()

	m.lbState.strategy = strategy
	logs.Info("Load balance strategy set to %v", strategy)
}

// SetLoadBalanceStrategyByName 通过策略名称设置负载均衡策略
func (m *Manager) SetLoadBalanceStrategyByName(strategyName string) error {
	strategy, err := ParseLoadBalanceStrategy(strategyName)
	if err != nil {
		return err
	}

	m.SetLoadBalanceStrategy(strategy)
	return nil
}

// GetCurrentLoadBalanceStrategy 获取当前使用的负载均衡策略
func (m *Manager) GetCurrentLoadBalanceStrategy() LoadBalanceStrategy {
	m.lbState.mu.RLock()
	defer m.lbState.mu.RUnlock()

	return m.lbState.strategy
}

// GetCurrentLoadBalanceStrategyName 获取当前使用的负载均衡策略名称
func (m *Manager) GetCurrentLoadBalanceStrategyName() string {
	strategy := m.GetCurrentLoadBalanceStrategy()
	return strategy.String()
}

// GetAvailableLoadBalanceStrategies 获取所有可用的负载均衡策略
func (m *Manager) GetAvailableLoadBalanceStrategies() []string {
	return []string{
		"random",
		"round_robin",
		"weighted_round_robin",
		"least_connection",
		"consistent_hash",
		"ip_hash",
	}
}

// String 返回负载均衡策略的字符串表示
func (s LoadBalanceStrategy) String() string {
	switch s {
	case Random:
		return "random"
	case RoundRobin:
		return "round_robin"
	case WeightedRoundRobin:
		return "weighted_round_robin"
	case LeastConnection:
		return "least_connection"
	case ConsistentHash:
		return "consistent_hash"
	case IPHash:
		return "ip_hash"
	default:
		return "unknown"
	}
}

// ParseLoadBalanceStrategy 将字符串解析为负载均衡策略
func ParseLoadBalanceStrategy(s string) (LoadBalanceStrategy, error) {
	switch strings.ToLower(s) {
	case "random":
		return Random, nil
	case "round_robin", "roundrobin":
		return RoundRobin, nil
	case "weighted_round_robin", "weightedroundrobin":
		return WeightedRoundRobin, nil
	case "least_connection", "leastconnection":
		return LeastConnection, nil
	case "consistent_hash", "consistenthash":
		return ConsistentHash, nil
	case "ip_hash", "iphash":
		return IPHash, nil
	default:
		return Random, fmt.Errorf("unknown load balance strategy: %s", s)
	}
}

// UpdateServerLoad 更新服务器负载
func (m *Manager) UpdateServerLoad(serverID string, load int) {
	m.lbState.mu.Lock()
	defer m.lbState.mu.Unlock()

	m.lbState.serverLoads[serverID] = load
}

// GetAllClients 获取所有客户端连接
func (m *Manager) GetAllClients() map[string]Connection {
	result := make(map[string]Connection)

	// 从所有分片中收集客户端
	for _, bucket := range m.clientBuckets {
		bucket.RLock()
		for cid, conn := range bucket.clients {
			result[cid] = conn
		}
		bucket.RUnlock()
	}

	return result
}

// GetConnectionCount 获取当前连接数量
func (m *Manager) GetConnectionCount() int {
	return int(atomic.LoadInt32(&m.stats.currentConnections))
}

// GetStats 获取性能统计信息
func (m *Manager) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"connections":            atomic.LoadInt32(&m.stats.currentConnections),
		"messages_processed":     atomic.LoadInt64(&m.stats.messageProcessed),
		"message_errors":         atomic.LoadInt64(&m.stats.messageErrors),
		"avg_processing_time_us": atomic.LoadInt64(&m.stats.avgProcessingTime),
		"worker_count":           m.workerCount,
		"bucket_count":           len(m.clientBuckets),
	}
}

// FindClientByUID 根据用户ID查找客户端连接
func (m *Manager) FindClientByUID(uid string) Connection {
	if uid == "" {
		return nil
	}

	// 并行搜索所有分片
	type result struct {
		conn  Connection
		found bool
	}

	results := make(chan result, len(m.clientBuckets))

	for _, bucket := range m.clientBuckets {
		go func(b *ClientBucket) {
			b.RLock()
			defer b.RUnlock()

			for _, conn := range b.clients {
				if conn.GetSession().Uid == uid {
					results <- result{conn: conn, found: true}
					return
				}
			}

			results <- result{found: false}
		}(bucket)
	}

	// 收集结果
	for i := 0; i < len(m.clientBuckets); i++ {
		if r := <-results; r.found {
			return r.conn
		}
	}

	return nil
}

func (m *Manager) Response(msg *stream.Msg) {
	// 编码消息（只编码一次）
	buf, err := protocol.MessageEncode(msg.Body)
	if err != nil {
		logs.Error("Response MessageEncode err:%v", err)
		return
	}
	res, err := protocol.Encode(protocol.Data, buf)
	if err != nil {
		logs.Error("Response Encode err:%v", err)
		return
	}

	if msg.Body.Type == protocol.Push {
		// 推送消息给多个用户
		if len(msg.PushUser) > 0 {
			// 创建用户ID到连接的映射
			userConnections := make(map[string][]Connection)

			// 并行收集每个分片中的目标连接
			var wg sync.WaitGroup
			var mu sync.Mutex // 保护userConnections

			for _, bucket := range m.clientBuckets {
				wg.Add(1)
				go func(b *ClientBucket) {
					defer wg.Done()

					b.RLock()
					for _, conn := range b.clients {
						uid := conn.GetSession().Uid
						if utils.Contains(msg.PushUser, uid) {
							mu.Lock()
							userConnections[uid] = append(userConnections[uid], conn)
							mu.Unlock()
						}
					}
					b.RUnlock()
				}(bucket)
			}

			wg.Wait()

			// 并行发送消息
			var sendWg sync.WaitGroup
			for _, connections := range userConnections {
				for _, conn := range connections {
					sendWg.Add(1)
					go func(c Connection) {
						defer sendWg.Done()
						c.SendMessage(res)
					}(conn)
				}
			}

			// 可选：等待所有消息发送完成
			// sendWg.Wait()
		}
	} else if msg.Cid != "" {
		// 发送消息给单个客户端
		bucket := m.getBucket(msg.Cid)
		bucket.RLock()
		connection, ok := bucket.clients[msg.Cid]
		bucket.RUnlock()

		if ok {
			connection.SendMessage(res)
		}
	}
}

func (m *Manager) remotePushChanHandler() {
	const batchSize = 32
	batch := make([]*stream.Msg, 0, batchSize)

	processBatch := func() {
		if len(batch) == 0 {
			return
		}

		// 并行处理批次中的消息
		var wg sync.WaitGroup
		for _, msg := range batch {
			if msg.Body.Type == protocol.Push {
				wg.Add(1)
				go func(pushMsg *stream.Msg) {
					defer wg.Done()
					m.Response(pushMsg)
				}(msg)
			}
		}

		// 等待所有消息处理完成
		wg.Wait()
		batch = batch[:0] // 清空批次
	}

	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case msg, ok := <-m.RemotePushChan:
			if !ok {
				// 通道已关闭
				processBatch() // 处理剩余消息
				return
			}

			batch = append(batch, msg)
			if len(batch) >= batchSize {
				processBatch()
			}

		case <-ticker.C:
			processBatch()
		}
	}
}

func (m *Manager) setSessionData(msg stream.Msg) {
	if msg.SessionData == nil {
		return
	}

	// 处理单个连接的数据
	if msg.Cid != "" && msg.SessionData.SingleData != nil {
		bucket := m.getBucket(msg.Cid)
		bucket.RLock()
		connection, ok := bucket.clients[msg.Cid]
		bucket.RUnlock()

		if ok {
			connection.GetSession().SetData(msg.Uid, msg.SessionData.SingleData)
		}
	}

	// 处理全局数据
	if len(msg.SessionData.AllData) > 0 {
		// 先更新Manager的数据
		m.dataLock.Lock()
		for k, v := range msg.SessionData.AllData {
			m.data[k] = v
		}
		m.dataLock.Unlock()

		// 使用工作池更新所有连接的数据
		allData := msg.SessionData.AllData

		// 并行处理每个分片
		var wg sync.WaitGroup
		for _, bucket := range m.clientBuckets {
			wg.Add(1)
			go func(b *ClientBucket) {
				defer wg.Done()

				// 获取分片中的所有连接
				b.RLock()
				connections := make([]Connection, 0, len(b.clients))
				for _, conn := range b.clients {
					connections = append(connections, conn)
				}
				b.RUnlock()

				// 更新每个连接的会话数据
				for _, conn := range connections {
					conn.GetSession().SetAll(allData)
				}
			}(bucket)
		}

		// 等待所有分片处理完成
		wg.Wait()
	}
}

// NewManager 创建一个新的连接管理器
func NewManager(maxConn int) *Manager {
	// 确定分片数量，使用2的幂次方以便位运算
	bucketCount := 32
	bucketMask := uint32(bucketCount - 1)

	// 确定工作协程数量，默认为CPU核心数的2倍
	workerCount := runtime.NumCPU() * 2

	m := &Manager{
		ClientReadChan: make(chan *MsgPack, 2048), // 增大缓冲区
		handlers:       make(map[protocol.PackageType]EventHandler),
		RemoteReadChan: make(chan []byte, 2048),      // 增大缓冲区
		RemotePushChan: make(chan *stream.Msg, 2048), // 增大缓冲区
		data:           make(map[string]any),
		maxConnections: maxConn,
		connSemaphore:  make(chan struct{}, maxConn),
		bucketMask:     bucketMask,
		workerCount:    workerCount,
		// 初始化负载均衡状态
		lbState: loadBalanceState{
			strategy:      Random, // 默认使用随机策略
			roundRobinIdx: make(map[string]int),
			hashRing:      make(map[string]*consistentHash),
			serverLoads:   make(map[string]int),
		},
	}

	// 初始化客户端分片
	m.clientBuckets = make([]*ClientBucket, bucketCount)
	for i := 0; i < bucketCount; i++ {
		m.clientBuckets[i] = NewClientBucket()
	}

	// 初始化工作协程池
	m.clientWorkers = make([]chan *MsgPack, workerCount)
	for i := 0; i < workerCount; i++ {
		m.clientWorkers[i] = make(chan *MsgPack, 256)
	}

	// 设置默认的CheckOriginHandler
	m.CheckOriginHandler = func(r *http.Request) bool {
		return true
	}

	logs.Info("WebSocket manager initialized with %d worker goroutines and %d connection buckets",
		workerCount, bucketCount)

	return m
}

// SetMaxConnections 设置最大连接数
func (m *Manager) SetMaxConnections(maxConn int) {
	// 只能在启动前调用
	if m.connSemaphore == nil {
		m.maxConnections = maxConn
		m.connSemaphore = make(chan struct{}, maxConn)
		logs.Info("Max connections set to %d", maxConn)
	} else {
		logs.Warn("Cannot change max connections after manager has started")
	}
}

// SetWorkerCount 设置工作协程数量
func (m *Manager) SetWorkerCount(count int) {
	// 只能在启动前调用
	if m.clientWorkers == nil {
		m.workerCount = count
		logs.Info("Worker count set to %d", count)
	} else {
		logs.Warn("Cannot change worker count after manager has started")
	}
}

// SetBucketCount 设置分片数量
func (m *Manager) SetBucketCount(count int) {
	// 只能在启动前调用
	if m.clientBuckets == nil {
		// 确保是2的幂次方
		bucketCount := 1
		for bucketCount < count {
			bucketCount *= 2
		}
		m.bucketMask = uint32(bucketCount - 1)
		logs.Info("Bucket count set to %d (rounded to power of 2)", bucketCount)
	} else {
		logs.Warn("Cannot change bucket count after manager has started")
	}
}
