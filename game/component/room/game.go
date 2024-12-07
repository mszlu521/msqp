package room

import (
	"errors"
	"framework/remote"
	"game/component/base"
	"game/component/mj"
	"game/component/proto"
	"game/component/sz"
)

type GameFrame interface {
	GetGameData(session *remote.Session) any
	StartGame(session *remote.Session, user *proto.RoomUser)
	GameMessageHandle(user *proto.RoomUser, session *remote.Session, msg []byte)
	IsUserEnableLeave() bool
	OnEventUserOffLine(user *proto.RoomUser, session *remote.Session)
}

func NewGameFrame(rule proto.GameRule, r base.RoomFrame) (GameFrame, error) {
	if rule.GameType == proto.PinSanZhang {
		return sz.NewGameFrame(rule, r), nil
	}
	if rule.GameType == proto.HongZhong {
		return mj.NewGameFrame(rule, r), nil
	}
	return nil, errors.New("no gameType")
}
