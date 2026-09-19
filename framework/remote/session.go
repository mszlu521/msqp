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
	if client != nil {
		go s.pushSessionChanRead()
	}
	return s
}

func (s *Session) GetUid() string {
	if s == nil {
		return ""
	}
	s.RLock()
	defer s.RUnlock()
	if s.msg == nil {
		return ""
	}
	return s.msg.Uid
}

func (s *Session) Put(key string, value any, t stream.DataType) {
	s.Lock()
	if s.data == nil {
		s.data = &stream.SessionData{AllData: make(map[string]any), SingleData: make(map[string]any)}
	}
	if t == stream.Single {
		s.data.SingleData[key] = value
	}
	if t == stream.All {
		s.data.AllData[key] = value
	}
	data := cloneSessionData(s.data)
	s.Unlock()
	if s.client != nil {
		s.pushSessionChan <- data
	}
}

func (s *Session) pushSessionChanRead() {
	for {
		select {
		case data := <-s.pushSessionChan:
			msgData := s.GetMsg()
			if msgData == nil || s.client == nil {
				continue
			}
			msg := stream.Msg{
				Dst:         msgData.Src,
				Src:         msgData.Dst,
				Cid:         msgData.Cid,
				Uid:         msgData.Uid,
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
	if s == nil {
		return
	}
	s.Lock()
	defer s.Unlock()
	if s.data == nil {
		s.data = &stream.SessionData{AllData: make(map[string]any), SingleData: make(map[string]any)}
	}
	if data != nil {
		for k, v := range data.SingleData {
			s.data.SingleData[k] = v
		}
		for k, v := range data.AllData {
			s.data.AllData[k] = v
		}
	}
}

func cloneSessionData(data *stream.SessionData) *stream.SessionData {
	if data == nil {
		return &stream.SessionData{SingleData: make(map[string]any), AllData: make(map[string]any)}
	}
	clone := &stream.SessionData{
		SingleData: make(map[string]any, len(data.SingleData)),
		AllData:    make(map[string]any, len(data.AllData)),
	}
	for key, value := range data.SingleData {
		clone.SingleData[key] = value
	}
	for key, value := range data.AllData {
		clone.AllData[key] = value
	}
	return clone
}

func (s *Session) Get(key string) (any, bool) {
	if s == nil {
		return nil, false
	}
	s.RLock()
	defer s.RUnlock()
	if s.data == nil {
		return nil, false
	}
	v, ok := s.data.SingleData[key]
	if !ok {
		v, ok = s.data.AllData[key]
	}
	return v, ok
}

func (s *Session) SetServerId(serverId string) {
	s.Lock()
	defer s.Unlock()
	s.serverId = serverId
}

func (s *Session) GetServerId() string {
	s.RLock()
	defer s.RUnlock()
	return s.serverId
}
func (s *Session) GetData() *stream.SessionData {
	s.RLock()
	defer s.RUnlock()
	return cloneSessionData(s.data)
}

func (s *Session) GetDst() string {
	if s == nil {
		return ""
	}
	s.RLock()
	defer s.RUnlock()
	if s.msg == nil {
		return ""
	}
	return s.msg.Dst
}

func (s *Session) SendProxy(newDst string) {
	if s == nil || s.client == nil {
		return
	}
	s.Lock()
	if s.msg == nil {
		s.Unlock()
		return
	}
	s.msg.Dst = newDst
	msg := *s.msg
	client := s.client
	s.Unlock()
	res, _ := json.Marshal(&msg)
	if err := client.SendMsg(newDst, res); err != nil {
		logs.Error("SendProxy session data err:%v", err)
	}
}
func (s *Session) Dispatch(router string, newDst string) {
	if s == nil || s.client == nil {
		return
	}
	s.Lock()
	if s.msg == nil {
		s.Unlock()
		return
	}
	s.msg.Router = router
	s.msg.Dst = newDst
	msg := *s.msg
	client := s.client
	s.Unlock()
	res, _ := json.Marshal(&msg)
	if err := client.SendMsg(newDst, res); err != nil {
		logs.Error("SendProxy session data err:%v", err)
	}
}

func (s *Session) GetMsg() *stream.Msg {
	if s == nil {
		return nil
	}
	s.RLock()
	defer s.RUnlock()
	if s.msg == nil {
		return nil
	}
	msg := *s.msg
	return &msg
}
