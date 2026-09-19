package logic

import (
	"common/biz"
	"core/models/entity"
	"core/service"
	cryptorand "crypto/rand"
	"fmt"
	"framework/msError"
	"framework/remote"
	"game/component/room"
	"math/big"
	"sync"
)

type UnionManager struct {
	sync.RWMutex
	unionList       map[int64]*Union
	reservedRoomIDs map[string]struct{}
}

func NewUnionManager() *UnionManager {
	return &UnionManager{
		unionList:       make(map[int64]*Union),
		reservedRoomIDs: make(map[string]struct{}),
	}
}

func (u *UnionManager) GetUnion(unionId int64,
	redisService *service.RedisService,
	userService *service.UserService,
	unionService *service.UnionService) *Union {
	u.Lock()
	defer u.Unlock()
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
	u.Lock()
	defer u.Unlock()
	if u.reservedRoomIDs == nil {
		u.reservedRoomIDs = make(map[string]struct{})
	}
	for {
		roomId := u.genRoomId()
		if _, reserved := u.reservedRoomIDs[roomId]; reserved {
			continue
		}
		collision := false
		for _, v := range u.unionList {
			v.RLock()
			if _, ok := v.RoomList[roomId]; ok {
				collision = true
			}
			v.RUnlock()
			if collision {
				break
			}
		}
		if !collision {
			u.reservedRoomIDs[roomId] = struct{}{}
			return roomId
		}
	}
}

func (u *UnionManager) releaseRoomID(roomID string) {
	u.Lock()
	delete(u.reservedRoomIDs, roomID)
	u.Unlock()
}

func (u *UnionManager) genRoomId() string {
	// Room IDs are six digits. Use a fresh cryptographically random value so
	// concurrent room creation does not share a global pseudo-random source.
	const roomIDRange = int64(900000)
	n, err := cryptorand.Int(cryptorand.Reader, big.NewInt(roomIDRange))
	if err != nil {
		// crypto/rand failure is exceptionally rare; preserve the six-digit
		// contract with a time-independent deterministic fallback.
		return "100000"
	}
	return fmt.Sprintf("%06d", n.Int64()+100000)
}

func (u *UnionManager) GetRoomById(roomId string) *room.Room {
	u.RLock()
	defer u.RUnlock()
	for _, v := range u.unionList {
		v.RLock()
		r, ok := v.RoomList[roomId]
		v.RUnlock()
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
	u.RLock()
	defer u.RUnlock()
	for _, v := range u.unionList {
		v.RLock()
		if v.RoomList[roomId] != nil {
			v.RUnlock()
			return v
		}
		v.RUnlock()
	}
	return nil
}
