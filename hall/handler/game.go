package handler

import (
	"common"
	"common/biz"
	"core/dao"
	"core/repo"
	"core/service"
	"encoding/json"
	"framework/remote"
	"hall/models/request"
)

type GameHandler struct {
	redisDao     *dao.RedisDao
	redisService *service.RedisService
}

func (h *GameHandler) JoinRoom(session *remote.Session, msg []byte) any {
	//uid := session.GetUid()
	var req request.JoinRoomReq
	if err := json.Unmarshal(msg, &req); err != nil {
		return common.F(biz.RequestDataError)
	}
	//找到game服务器进行调用
	if err := Dispatch(h.redisService, session, req.RoomId, "unionHandler.joinRoom"); err != nil {
		return common.F(err)
	}
	return nil
}

func NewGameHandler(r *repo.Manager) *GameHandler {
	return &GameHandler{
		redisDao:     dao.NewRedisDao(r),
		redisService: service.NewRedisService(r),
	}
}
