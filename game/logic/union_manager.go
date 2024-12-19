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

func (u *UnionManager) GetUnion(unionId int64,
	redisService *service.RedisService,
	userService *service.UserService,
	unionService *service.UnionService) *Union {
	u.Lock()
	u.Unlock()
	union, ok := u.unionList[unionId]
	if ok {
		return union
	}
	union = NewUnion(u, unionId, unionService, redisService, userService)
	union.init()
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

func (u *UnionManager) JoinRoom(session *remote.Session, roomId string, data *entity.User) *msError.Error {
	union := u.getUnionByRoomID(roomId)
	if union == nil {
		return biz.RoomNotExist
	}
	return union.JoinRoom(session, roomId, data)
}

func (u *UnionManager) IsUserInRoom(roomId string, uid string) bool {
	rooms := u.GetRoomById(roomId)
	if rooms == nil {
		return false
	}
	return rooms.IsUserInRoom(uid)
}

func (u *UnionManager) getUnionByRoomID(roomId string) *Union {
	for _, v := range u.unionList {
		if v.RoomList[roomId] != nil {
			return v
		}
	}
	return nil
}
