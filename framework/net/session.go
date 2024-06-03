package net

import "sync"

type Session struct {
	sync.RWMutex
	Cid  string
	Uid  string
	data map[string]any
	all  map[string]any
}

func NewSession(cid string) *Session {
	return &Session{
		Cid:  cid,
		data: make(map[string]any),
		all:  make(map[string]any),
	}
}

func (s *Session) Put(key string, v any) {
	s.Lock()
	defer s.Unlock()
	s.data[key] = v
}
func (s *Session) Get(key string) (any, bool) {
	s.RLock()
	defer s.RUnlock()
	v, ok := s.data[key]
	return v, ok
}
func (s *Session) SetData(uid string, data map[string]any) {
	s.Lock()
	defer s.Unlock()
	if s.Uid == uid {
		for k, v := range data {
			s.data[k] = v
		}
	}
}
func (s *Session) SetAll(data map[string]any) {
	s.Lock()
	defer s.Unlock()
	for k, v := range data {
		s.all[k] = v
	}
}
