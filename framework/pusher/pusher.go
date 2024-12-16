package pusher

import (
	"common/logs"
	"encoding/json"
	"framework/protocol"
	"framework/remote"
	"framework/stream"
)

var _pusher *Pusher

type Pusher struct {
	client   remote.Client
	pushChan chan *stream.PushMessage
}

func GetPusher() *Pusher {
	return _pusher
}

func (p *Pusher) Push(m *stream.Msg, users []stream.PushUser, data any, router string) {
	msgData, _ := json.Marshal(data)
	pm := stream.PushData{
		Data:   msgData,
		Router: router,
	}
	upm := &stream.PushMessage{
		Users:    users,
		PushData: pm,
		Msg:      m,
	}
	p.pushChan <- upm
}

func (p *Pusher) pushChanRead() {
	for {
		select {
		case data := <-p.pushChan:
			logs.Info("push stream11111 %v", string(data.PushData.Data))
			pushMessage := protocol.Message{
				Type:  protocol.Push,
				ID:    data.Msg.Body.ID,
				Route: data.PushData.Router,
				Data:  data.PushData.Data,
			}
			logs.Info("push stream222222 %v", data.Users)
			userMap := make(map[string][]string)
			for _, v := range data.Users {
				//将同一个目的地的到一起
				users, ok := userMap[v.ConnectorId]
				if !ok {
					users = make([]string, 0)
				}
				userMap[v.ConnectorId] = append(users, v.Uid)
			}
			for dst, uids := range userMap {
				msgData := stream.Msg{
					Dst:         dst,
					Src:         data.Msg.Dst,
					Body:        &pushMessage,
					Cid:         data.Msg.Cid,
					Uid:         data.Msg.Uid,
					PushUser:    uids,
					SessionType: stream.Normal,
				}
				result, _ := json.Marshal(msgData)
				logs.Info("push stream dst:%v,%v", dst, string(msgData.Body.Data))
				err := p.client.SendMsg(msgData.Dst, result)
				if err != nil {
					logs.Error("push stream err:%v, stream=%v", err, msgData)
				}
			}

		}
	}
}

func NewPusher(client remote.Client) {
	_pusher = &Pusher{
		client:   client,
		pushChan: make(chan *stream.PushMessage, 1024),
	}
	go _pusher.pushChanRead()
}
