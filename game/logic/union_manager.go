package logic

import (
	"common/biz"
	"core/models/entity"
	"core/service"
	"fmt"
	"framework/msError"
	"framework/remote"
	"game/component/room"
	"math/rand"
	"sync"
	"time"
)

type UnionManager struct {
	sync.RWMutex
	unionList map[int64]*Union
}

func NewUnionManager() *UnionManager {
	return &UnionManager{
		unionList: make(map[int64]*Union),
	}
}

func (u *UnionManager) GetUnion(unionId int64) *Union {
	u.Lock()
	u.Unlock()
	union, ok := u.unionList[unionId]
	if ok {
		return union
	}
	union = NewUnion(u)
	u.unionList[unionId] = union
	return union
}

func (u *UnionManager) CreateRoomId() string {
	//随机数的方式去创建
	roomId := u.genRoomId()
	for _, v := range u.unionList {
		_, ok := v.RoomList[roomId]
		if ok {
			return u.CreateRoomId()
		}
	}
	return roomId
}

func (u *UnionManager) genRoomId() string {
	rand.New(rand.NewSource(time.Now().UnixNano()))
	//房间号是6位数
	roomIdInt := rand.Int63n(999999)
	if roomIdInt < 100000 {
		roomIdInt += 100000
	}
	return fmt.Sprintf("%d", roomIdInt)
}

func (u *UnionManager) GetRoomById(roomId string) *room.Room {
	for _, v := range u.unionList {
		r, ok := v.RoomList[roomId]
		if ok {
			return r
		}
	}
	return nil
}

func (u *UnionManager) JoinRoom(service *service.RedisService, session *remote.Session, roomId string, data *entity.User) *msError.Error {
	for _, v := range u.unionList {
		r, ok := v.RoomList[roomId]
		if ok {
			return r.JoinRoom(service, session, data)
		}
	}
	return biz.RoomNotExist
}
