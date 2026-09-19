package testutil

import (
	"core/models/enums"
	"framework/remote"
	"framework/stream"
	"game/component/proto"
	"sync"
)

type Room struct {
	Users      map[string]*proto.RoomUser
	Creator    proto.RoomCreator
	CurBureau  int
	MaxBureau  int
	Broadcasts []any
	Direct     []any
	Dismissals []enums.RoomDismissReason

	mu         sync.RWMutex
	actionMu   sync.Mutex
	dismissing bool
}

func NewPlayingRoom(count int) *Room {
	room := &Room{Users: make(map[string]*proto.RoomUser, count), MaxBureau: 10}
	for chairID := 0; chairID < count; chairID++ {
		uid := string(rune('a' + chairID))
		room.Users[uid] = &proto.RoomUser{
			Uid:        uid,
			UserInfo:   &proto.UserInfo{Uid: uid},
			ChairID:    chairID,
			UserStatus: enums.Playing,
		}
	}
	return room
}

func (r *Room) GetUsers() map[string]*proto.RoomUser           { return r.Users }
func (r *Room) GetId() string                                  { return "test" }
func (r *Room) EndGame(*remote.Session)                        {}
func (r *Room) UserReady(string, *remote.Session)              {}
func (r *Room) SendData(_ *stream.Msg, _ []string, data any)   { r.Direct = append(r.Direct, data) }
func (r *Room) SendDataAll(_ *stream.Msg, data any)            { r.Broadcasts = append(r.Broadcasts, data) }
func (r *Room) GetCreator() *proto.RoomCreator                 { return &r.Creator }
func (r *Room) ConcludeGame([]*proto.EndData, *remote.Session) {}
func (r *Room) DismissRoom(_ *remote.Session, reason enums.RoomDismissReason) {
	r.Dismissals = append(r.Dismissals, reason)
}
func (r *Room) IsDismissing() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.dismissing
}
func (r *Room) SetDismissing(value bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.dismissing = value
}
func (r *Room) SetCurBureau(value int) { r.CurBureau = value }
func (r *Room) GetCurBureau() int      { return r.CurBureau }
func (r *Room) GetMaxBureau() int      { return r.MaxBureau }
func (r *Room) GetHongBaoList() any    { return nil }
func (r *Room) GetGameStarted() bool   { return true }
func (r *Room) RunGameAction(action func()) {
	if action == nil {
		return
	}
	r.actionMu.Lock()
	defer r.actionMu.Unlock()
	action()
}
