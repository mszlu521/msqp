package base

import (
	"framework/remote"
	"framework/stream"
	"game/component/proto"
)

type RoomFrame interface {
	GetUsers() map[string]*proto.RoomUser
	GetId() string
	EndGame(session *remote.Session)
	UserReady(uid string, session *remote.Session)
	SendData(msg *stream.Msg, users []string, data any)
	SendDataAll(msg *stream.Msg, data any)
	GetCreator() *proto.RoomCreator
	ConcludeGame(data []*proto.EndData, session *remote.Session)
	IsDismissing() bool
	SetCurBureau(int)
	GetCurBureau() int
	GetMaxBureau() int
}
