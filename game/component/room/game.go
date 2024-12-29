package room

import (
	"core/models/enums"
	"errors"
	"framework/remote"
	"game/component/base"
	"game/component/mj"
	"game/component/proto"
	"game/component/sz"
)

type GameFrame interface {
	GetEnterGameData(session *remote.Session) any
	GameMessageHandle(user *proto.RoomUser, session *remote.Session, msg []byte)
	IsUserEnableLeave(chairID int) bool
	OnEventUserOffLine(user *proto.RoomUser, session *remote.Session)
	OnEventUserEntry(user *proto.RoomUser, session *remote.Session)
	OnEventGameStart(user *proto.RoomUser, session *remote.Session)
	OnEventRoomDismiss(reason enums.RoomDismissReason, session *remote.Session)
	GetGameVideoData() any
	GetGameBureauData() any
}

func NewGameFrame(rule proto.GameRule, r base.RoomFrame, session *remote.Session) (GameFrame, error) {
	if rule.GameType == enums.SZ {
		return sz.NewGameFrame(rule, r, session), nil
	}
	if rule.GameType == enums.ZNMJ {
		return mj.NewGameFrame(rule, r, session), nil
	}
	return nil, errors.New("no gameType")
}
