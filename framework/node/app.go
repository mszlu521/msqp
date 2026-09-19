package node

import (
	"common/logs"
	"encoding/json"
	"errors"
	"framework/pusher"
	"framework/remote"
	"framework/stream"
)

// App 就是nats的客户端 处理实际游戏逻辑的服务
type App struct {
	remoteCli remote.Client
	readChan  chan []byte
	writeChan chan *stream.Msg
	handlers  LogicHandler
}

func Default() *App {
	return &App{
		readChan:  make(chan []byte, 1024),
		writeChan: make(chan *stream.Msg, 1024),
		handlers:  make(LogicHandler),
	}
}

func (a *App) Run(serverId string) error {
	a.remoteCli = remote.NewNatsClient(serverId, a.readChan)
	err := a.remoteCli.Run()
	if err != nil {
		return err
	}
	pusher.NewPusher(a.remoteCli)
	go a.readChanMsg(serverId)
	go a.writeChanMsg()
	return nil
}

func (a *App) readChanMsg(serverId string) {
	//收到的是 其他nas client发送的消息
	for {
		select {
		case msg := <-a.readChan:
			remoteMsg, err := decodeRemoteMessage(msg)
			if err != nil {
				logs.Error("app decode remote message failed,err:%v", err)
				continue
			}
			logs.Warn("app readChanMsg:%v", string(remoteMsg.Body.Data))
			session := remote.NewSession(a.remoteCli, &remoteMsg)
			session.SetServerId(serverId)
			session.SetData(remoteMsg.SessionData)
			//根据路由消息 发送给对应的handler进行处理
			router := remoteMsg.Router
			if handlerFunc := a.handlers[router]; handlerFunc != nil {
				go func() {
					result := handlerFunc(session, remoteMsg.Body.Data)
					message := remoteMsg.Body
					var body []byte
					if result == nil {
						return
					}
					body, _ = json.Marshal(result)
					message.Data = body
					//得到结果了 发送给connector
					responseMsg := &stream.Msg{
						Src:  remoteMsg.Dst,
						Dst:  remoteMsg.Src,
						Body: message,
						Uid:  remoteMsg.Uid,
						Cid:  remoteMsg.Cid,
					}
					a.writeChan <- responseMsg
				}()
			}
		}
	}

}

func decodeRemoteMessage(data []byte) (stream.Msg, error) {
	var msg stream.Msg
	if err := json.Unmarshal(data, &msg); err != nil {
		return stream.Msg{}, err
	}
	if msg.Body == nil {
		return stream.Msg{}, errors.New("remote message body is required")
	}
	return msg, nil
}

func (a *App) writeChanMsg() {
	for {
		select {
		case msg, ok := <-a.writeChan:
			if ok {
				marshal, _ := json.Marshal(msg)
				err := a.remoteCli.SendMsg(msg.Dst, marshal)
				if err != nil {
					logs.Error("app remote send stream err:%v", err)
				}
			}
		}
	}
}

func (a *App) Close() {
	if a.remoteCli != nil {
		a.remoteCli.Close()
	}
}

func (a *App) RegisterHandler(handler LogicHandler) {
	a.handlers = handler
}
