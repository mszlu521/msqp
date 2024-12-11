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
	GameMessageHandle(user *proto.RoomUser, session *remote.Session, msg []byte)
	IsUserEnableLeave(chairID int) bool
	OnEventUserOffLine(user *proto.RoomUser, session *remote.Session)
	OnEventUserEntry(user *proto.RoomUser, session *remote.Session)
	OnEventGameStart(user *proto.RoomUser, session *remote.Session)
	OnEventRoomDismiss(reason proto.RoomDismissReason, session *remote.Session)
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
