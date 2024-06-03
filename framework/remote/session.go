package remote

import (
	"common/logs"
	"encoding/json"
	"framework/stream"
	"sync"
)

type Session struct {
	sync.RWMutex
	client Client
	msg    *stream.Msg
	//pushChan        chan *userPushMsg
	data            *stream.SessionData
	pushSessionChan chan *stream.SessionData
	serverId        string
}

func NewSession(client Client, msg *stream.Msg) *Session {
	s := &Session{
		client: client,
		msg:    msg,
		//pushChan:        make(chan *userPushMsg, 1024),
		pushSessionChan: make(chan *stream.SessionData, 1024),
		data: &stream.SessionData{
			AllData:    make(map[string]any),
			SingleData: make(map[string]any),
		},
	}
	//go s.pushChanRead()
	go s.pushSessionChanRead()
	return s
}

func (s *Session) GetUid() string {
	return s.msg.Uid
}

func (s *Session) Put(key string, value any, t stream.DataType) {
	s.Lock()
	defer s.Unlock()
	if t == stream.Single {
		s.data.SingleData[key] = value
	}
	if t == stream.All {
		s.data.AllData[key] = value
	}
	s.pushSessionChan <- s.data
}

func (s *Session) pushSessionChanRead() {
	for {
		select {
		case data := <-s.pushSessionChan:
			msg := stream.Msg{
				Dst:         s.msg.Src,
				Src:         s.msg.Dst,
				Cid:         s.msg.Cid,
				Uid:         s.msg.Uid,
				SessionData: data,
				SessionType: stream.Session,
			}
			res, _ := json.Marshal(msg)
			if err := s.client.SendMsg(msg.Dst, res); err != nil {
				logs.Error("push session data err:%v", err)
			}
		}
	}
}

func (s *Session) SetData(data *stream.SessionData) {
	s.Lock()
	defer s.Unlock()
	if data != nil {
		for k, v := range data.SingleData {
			s.data.SingleData[k] = v
		}
		for k, v := range data.AllData {
			s.data.AllData[k] = v
		}
	}
}

func (s *Session) Get(key string) (any, bool) {
	s.RLock()
	defer s.RUnlock()
	v, ok := s.data.SingleData[key]
	if !ok {
		v, ok = s.data.AllData[key]
	}
	return v, ok
}

func (s *Session) SetServerId(serverId string) {
	s.serverId = serverId
}

func (s *Session) GetServerId() string {
	return s.serverId
}
func (s *Session) GetData() *stream.SessionData {
	return s.data
}

func (s *Session) GetDst() string {
	return s.msg.Dst
}

func (s *Session) SendProxy(newDst string) {
	s.msg.Dst = newDst
	res, _ := json.Marshal(s.msg)
	if err := s.client.SendMsg(newDst, res); err != nil {
		logs.Error("SendProxy session data err:%v", err)
	}
}
func (s *Session) Dispatch(router string, newDst string) {
	s.msg.Router = router
	s.msg.Dst = newDst
	res, _ := json.Marshal(s.msg)
	if err := s.client.SendMsg(newDst, res); err != nil {
		logs.Error("SendProxy session data err:%v", err)
	}
}

func (s *Session) GetMsg() *stream.Msg {
	return s.msg
}
