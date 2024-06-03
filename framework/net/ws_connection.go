package net

import (
	"common/logs"
	"fmt"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"sync/atomic"
	"time"
)

var cidBase uint64 = 10000

var (
	pongWait             = 10 * time.Second
	writeWait            = 10 * time.Second
	pingInterval         = (pongWait * 9) / 10
	maxMessageSize int64 = 1024
)

type WsConnection struct {
	Cid        string
	Conn       *websocket.Conn
	manager    *Manager
	ReadChan   chan *MsgPack
	WriteChan  chan []byte
	Session    *Session
	pingTicker *time.Ticker
}

func (c *WsConnection) GetSession() *Session {
	return c.Session
}

func (c *WsConnection) SendMessage(buf []byte) error {
	c.WriteChan <- buf
	return nil
}

func (c *WsConnection) Close() {
	if c.Conn != nil {
		c.Conn.Close()
	}
	if c.pingTicker != nil {
		c.pingTicker.Stop()
	}
}

func (c *WsConnection) Run() {
	go c.readMessage()
	go c.writeMessage()
	//做一些心跳检测 websocket中 ping pong机制
	c.Conn.SetPongHandler(c.PongHandler)
}

func (c *WsConnection) writeMessage() {

	c.pingTicker = time.NewTicker(pingInterval)
	for {
		select {
		case message, ok := <-c.WriteChan:
			if !ok {
				if err := c.Conn.WriteMessage(websocket.CloseMessage, nil); err != nil {
					logs.Error("connection closed, %v", err)
				}
				return
			}
			//logs.Error("%v", stream)
			if err := c.Conn.WriteMessage(websocket.BinaryMessage, message); err != nil {
				logs.Error("client[%s] write stream err :%v", c.Cid, err)
			}
		case <-c.pingTicker.C:
			if err := c.Conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
				logs.Error("client[%s] ping SetWriteDeadline err :%v", c.Cid, err)
			}
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				logs.Error("client[%s] ping  err :%v", c.Cid, err)
				c.Close()
			}
		}
	}
}

func (c *WsConnection) readMessage() {
	defer func() {
		c.manager.removeClient(c)
	}()
	c.Conn.SetReadLimit(maxMessageSize)
	if err := c.Conn.SetReadDeadline(time.Now().Add(pongWait)); err != nil {
		logs.Error("SetReadDeadline err:%v", err)
		return
	}
	for {
		messageType, message, err := c.Conn.ReadMessage()
		if err != nil {
			break
		}
		//客户端发来的消息是二进制消息
		if messageType == websocket.BinaryMessage {
			if c.ReadChan != nil {
				c.ReadChan <- &MsgPack{
					Cid:  c.Cid,
					Body: message,
				}
			}
		} else {
			logs.Error("unsupported stream type : %d", messageType)
		}
	}
}

func (c *WsConnection) PongHandler(data string) error {
	if err := c.Conn.SetReadDeadline(time.Now().Add(pongWait)); err != nil {
		return err
	}
	return nil
}

func NewWsConnection(conn *websocket.Conn, manager *Manager) *WsConnection {
	cid := fmt.Sprintf("%s-%s-%d", uuid.New().String(), manager.ServerId, atomic.AddUint64(&cidBase, 1))
	return &WsConnection{
		Conn:      conn,
		manager:   manager,
		Cid:       cid,
		WriteChan: make(chan []byte, 1024),
		ReadChan:  manager.ClientReadChan,
		Session:   NewSession(cid),
	}
}
