package logic

import (
	"common/biz"
	"common/logs"
	"core/models/entity"
	"core/service"
	"framework/msError"
	"framework/remote"
	"game/component/room"
	"game/models/request"
	"sync"
)

type Union struct {
	sync.RWMutex
	Id       int64
	m        *UnionManager
	RoomList map[string]*room.Room
}

func (u *Union) CreateRoom(redisService *service.RedisService, userService *service.UserService, session *remote.Session, req request.CreateRoomReq, userData *entity.User) *msError.Error {
	newRoom, err := u.createRoom(req, userData.Uid)
	if err != nil {
		logs.Error("CreateRoom err:%v", err)
		return biz.Fail
	}
	newRoom.UserService = userService
	newRoom.RedisService = redisService
	return newRoom.UserEntryRoom(session, userData)
}

func (u *Union) DismissRoom(roomId string) {
	u.Lock()
	defer u.Unlock()
	delete(u.RoomList, roomId)
}

func (u *Union) createRoom(req request.CreateRoomReq, uid string) (*room.Room, error) {
	u.Lock()
	defer u.Unlock()
	//1. 需要创建一个房间 生成一个房间号
	roomId := u.m.CreateRoomId()
	newRoom, err := room.NewRoom(uid, roomId, req.UnionID, req.GameRule, u)
	if err != nil {
		return nil, err
	}
	u.RoomList[roomId] = newRoom
	return newRoom, nil
}
func NewUnion(m *UnionManager) *Union {
	return &Union{
		RoomList: make(map[string]*room.Room),
		m:        m,
	}
}
