package handler

import (
	"common"
	"common/biz"
	"context"
	"core/repo"
	"core/service"
	"encoding/json"
	"fmt"
	"framework/remote"
	"game/logic"
	"game/models/request"
)

type UnionHandler struct {
	um           *logic.UnionManager
	userService  *service.UserService
	redisService *service.RedisService
	unionService *service.UnionService
}

func (h *UnionHandler) CreateRoom(session *remote.Session, msg []byte) any {
	//union 联盟 持有房间
	//unionManager 管理联盟
	//room 房间 又关联 game接口 实现多个不同的游戏
	//1. 接收参数
	uid := session.GetUid()
	if len(uid) <= 0 {
		return common.F(biz.InvalidUsers)
	}
	var req request.CreateRoomReq
	if err := json.Unmarshal(msg, &req); err != nil {
		return common.F(biz.RequestDataError)
	}
	//2. 根据session 用户id 查询用户的信息
	userData, err := h.userService.FindUserByUid(context.TODO(), uid)
	if err != nil {
		return common.F(err)
	}
	if userData == nil {
		return common.F(biz.InvalidUsers)
	}
	//3. 根据游戏规则 游戏类型 用户信息（创建房间的用户） 创建房间了
	roomId, ok := session.Get("roomId")
	if ok {
		//已经在房间中
		isUserInRoom := h.um.IsUserInRoom(fmt.Sprintf("%v", roomId), uid)
		if isUserInRoom {
			return common.F(biz.Fail)
		}
	}

	union := h.um.GetUnion(req.UnionID, h.redisService, h.userService, h.unionService)
	err = union.CreateRoom(h.redisService, h.userService, session, req, userData)
	if err != nil {
		return common.F(err)
	}
	return common.S(nil)
}

func (h *UnionHandler) JoinRoom(session *remote.Session, msg []byte) any {
	uid := session.GetUid()
	if len(uid) <= 0 {
		return common.F(biz.InvalidUsers)
	}
	var req request.JoinRoomReq
	if err := json.Unmarshal(msg, &req); err != nil {
		return common.F(biz.RequestDataError)
	}
	//判断roomId是否在当前服务器，如果不在转发请求
	isCurrent, err := Proxy(h.redisService, session, req.RoomID)
	if err != nil {
		return common.F(err)
	}
	if !isCurrent {
		return nil
	}
	oldRoomId, ok := session.Get("roomId")
	if ok {
		//已经在房间中
		if req.RoomID != oldRoomId {
			roomStr := fmt.Sprintf("%v", oldRoomId)
			isUserInRoom := h.um.IsUserInRoom(roomStr, uid)
			if isUserInRoom {
				req.RoomID = roomStr
			}
		}
	}
	userData, err := h.userService.FindUserByUid(context.TODO(), uid)
	if err != nil {
		return common.F(err)
	}
	if userData == nil {
		return common.F(biz.InvalidUsers)
	}
	bizErr := h.um.JoinRoom(session, req.RoomID, userData)
	if bizErr != nil {
		return common.F(bizErr)
	}
	return nil
}

func (h *UnionHandler) GetUnionInfo(session *remote.Session, msg []byte) any {
	var req request.GetUnionReq
	if err := json.Unmarshal(msg, &req); err != nil {
		return common.F(biz.RequestDataError)
	}
	if req.UnionID <= 0 {
		return common.F(biz.RequestDataError)
	}
	user, err := h.userService.FindUserByUid(context.TODO(), session.GetUid())
	if err != nil {
		return common.F(err)
	}
	if user == nil {
		return common.F(biz.InvalidUsers)
	}
	union := h.um.GetUnion(req.UnionID, h.redisService, h.userService, h.unionService)
	unionInfo := union.GetUnionInfo(session.GetUid())
	roomList := union.GetUnionRoomList()
	res := map[string]any{
		"unionInfo":     unionInfo,
		"roomList":      roomList,
		"unionInfoList": user.UnionInfo,
	}
	return common.S(res)
}

func (h *UnionHandler) GetUnionRoomList(session *remote.Session, msg []byte) any {
	var req request.GetUnionReq
	if err := json.Unmarshal(msg, &req); err != nil {
		return common.F(biz.RequestDataError)
	}
	if req.UnionID <= 0 {
		return common.F(biz.RequestDataError)
	}
	union := h.um.GetUnion(req.UnionID, h.redisService, h.userService, h.unionService)
	roomList := union.GetUnionRoomList()
	res := map[string]any{
		"roomList": roomList,
	}
	return common.S(res)
}

func (h *UnionHandler) QuickJoin(session *remote.Session, msg []byte) any {
	uid := session.GetUid()
	userData, err := h.userService.FindUserByUid(context.TODO(), uid)
	if err != nil {
		return common.F(err)
	}
	if userData == nil {
		return common.F(biz.InvalidUsers)
	}
	roomId, ok := session.Get("roomId")
	if ok {
		//已经在房间中
		isUserInRoom := h.um.IsUserInRoom(fmt.Sprintf("%v", roomId), uid)
		if isUserInRoom {
			return common.F(biz.Fail)
		}
	}
	var req request.QuickJoinReq
	if err := json.Unmarshal(msg, &req); err != nil {
		return common.F(biz.RequestDataError)
	}
	if req.UnionID <= 0 {
		return common.F(biz.RequestDataError)
	}
	union := h.um.GetUnion(req.UnionID, h.redisService, h.userService, h.unionService)
	e := union.QuickJoin(session, req.GameRuleID, userData)
	if e.Code != biz.OK {
		return common.F(e)
	}
	return common.S(nil)
}

func (h *UnionHandler) GetHongBao(session *remote.Session, msg []byte) any {
	var req request.HoneBaoReq
	if err := json.Unmarshal(msg, &req); err != nil {
		return common.F(biz.RequestDataError)
	}
	if req.UnionID <= 0 {
		return common.F(biz.RequestDataError)
	}
	union := h.um.GetUnion(req.UnionID, h.redisService, h.userService, h.unionService)
	res, err := union.GetHongBao(session.GetUid())
	if err != nil {
		return common.F(err)
	}

	return common.S(res)
}

func NewUnionHandler(r *repo.Manager, um *logic.UnionManager) *UnionHandler {
	return &UnionHandler{
		um:           um,
		userService:  service.NewUserService(r),
		redisService: service.NewRedisService(r),
		unionService: service.NewUnionService(r),
	}
}
