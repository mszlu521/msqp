package room

import (
	"framework/pusher"
	"framework/stream"
)

func (r *Room) ServerMessagePush(msg *stream.Msg, users []stream.PushUser, data any) {
	pusher.GetPusher().Push(msg, users, data, "ServerMessagePush")
}
func (r *Room) SendData(msg *stream.Msg, uids []string, data any) {
	users := make([]stream.PushUser, 0)
	for _, v := range uids {
		user, ok := r.users[v]
		if ok {
			users = append(users, stream.PushUser{
				Uid:         user.UserInfo.Uid,
				ConnectorId: user.UserInfo.FrontendId,
			})
		}
	}
	r.ServerMessagePush(msg, users, data)
}

func (r *Room) SendDataAll(msg *stream.Msg, data any) {
	users := make([]stream.PushUser, 0)
	for _, v := range r.users {
		users = append(users, stream.PushUser{
			Uid:         v.UserInfo.Uid,
			ConnectorId: v.UserInfo.FrontendId,
		})
	}
	r.ServerMessagePush(msg, users, data)
}
