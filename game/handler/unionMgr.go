package handler

import (
	"common"
	"common/biz"
	"core/repo"
	"core/service"
	"encoding/json"
	"framework/remote"
	"game/logic"
	"game/models/request"
)

type UnionMgrHandler struct {
	um           *logic.UnionManager
	userService  *service.UserService
	redisService *service.RedisService
	unionService *service.UnionService
}

func (h *UnionMgrHandler) AddRoomRuleList(session *remote.Session, msg []byte) any {
	var req request.AddRoomRuleReq
	if err := json.Unmarshal(msg, &req); err != nil {
		return common.F(biz.RequestDataError)
	}
	union := h.um.GetUnion(req.UnionID, h.redisService, h.userService, h.unionService)
	if session.GetUid() != union.GetOwnerUid() {
		return common.F(biz.RequestDataError)
	}
	err := union.AddRoomRuleList(req.RoomRule, req.RuleName, req.GameType)
	if err != nil {
		return common.F(biz.SqlError)
	}
	unionInfo := union.GetUnionInfo(session.GetUid())
	roomList := union.GetUnionRoomList()
	res := map[string]any{
		"unionInfo": unionInfo,
		"roomList":  roomList,
	}
	return common.S(res)
}

func (h *UnionMgrHandler) UpdateRoomRuleList(session *remote.Session, msg []byte) any {
	var req request.UpdateRoomRuleReq
	if err := json.Unmarshal(msg, &req); err != nil {
		return common.F(biz.RequestDataError)
	}
	union := h.um.GetUnion(req.UnionID, h.redisService, h.userService, h.unionService)
	if session.GetUid() != union.GetOwnerUid() {
		return common.F(biz.RequestDataError)
	}
	err := union.UpdateRoomRuleList(req.ID, req.RoomRule, req.RuleName, req.GameType)
	if err != nil {
		return common.F(biz.SqlError)
	}
	unionInfo := union.GetUnionInfo(session.GetUid())
	roomList := union.GetUnionRoomList()
	res := map[string]any{
		"unionInfo": unionInfo,
		"roomList":  roomList,
	}
	return common.S(res)
}

func (h *UnionMgrHandler) UpdateOpeningStatus(session *remote.Session, msg []byte) any {
	var req request.UpdateOpeningStatusReq
	if err := json.Unmarshal(msg, &req); err != nil {
		return common.F(biz.RequestDataError)
	}
	union := h.um.GetUnion(req.UnionID, h.redisService, h.userService, h.unionService)
	if session.GetUid() != union.GetOwnerUid() {
		return common.F(biz.RequestDataError)
	}
	union.UpdateOpeningStatus(req.IsOpen)
	unionInfo := union.GetUnionInfo(session.GetUid())
	roomList := union.GetUnionRoomList()
	res := map[string]any{
		"unionInfo": unionInfo,
		"roomList":  roomList,
	}
	return common.S(res)
}

func (h *UnionMgrHandler) RemoveRoomRuleList(session *remote.Session, msg []byte) any {
	var req request.RemoveRoomRuleReq
	if err := json.Unmarshal(msg, &req); err != nil {
		return common.F(biz.RequestDataError)
	}
	union := h.um.GetUnion(req.UnionID, h.redisService, h.userService, h.unionService)
	if session.GetUid() != union.GetOwnerUid() {
		return common.F(biz.RequestDataError)
	}
	union.RemoveRoomRuleList(req.RoomRuleId)
	unionInfo := union.GetUnionInfo(session.GetUid())
	roomList := union.GetUnionRoomList()
	res := map[string]any{
		"unionInfo": unionInfo,
		"roomList":  roomList,
	}
	return common.S(res)
}

func (h *UnionMgrHandler) UpdateUnionNotice(session *remote.Session, msg []byte) any {
	var req request.UpdateUnionNoticeReq
	if err := json.Unmarshal(msg, &req); err != nil {
		return common.F(biz.RequestDataError)
	}
	if req.UnionID <= 0 {
		return common.F(biz.RequestDataError)
	}
	if req.Notice == "" || len(req.Notice) > 150 {
		return common.F(biz.RequestDataError)
	}

	union := h.um.GetUnion(req.UnionID, h.redisService, h.userService, h.unionService)
	if session.GetUid() != union.GetOwnerUid() {
		return common.F(biz.RequestDataError)
	}
	union.UpdateUnionNotice(req.Notice)
	unionInfo := union.GetUnionInfo(session.GetUid())
	roomList := union.GetUnionRoomList()
	res := map[string]any{
		"unionInfo": unionInfo,
		"roomList":  roomList,
	}
	return common.S(res)
}

func (h *UnionMgrHandler) UpdateUnionName(session *remote.Session, msg []byte) any {
	var req request.UpdateUnionNameReq
	if err := json.Unmarshal(msg, &req); err != nil {
		return common.F(biz.RequestDataError)
	}
	if req.UnionID <= 0 {
		return common.F(biz.RequestDataError)
	}
	if req.UnionName == "" || len(req.UnionName) > 60 {
		return common.F(biz.RequestDataError)
	}
	union := h.um.GetUnion(req.UnionID, h.redisService, h.userService, h.unionService)
	if session.GetUid() != union.GetOwnerUid() {
		return common.F(biz.RequestDataError)
	}
	union.UpdateUnionName(req.UnionName)
	unionInfo := union.GetUnionInfo(session.GetUid())
	roomList := union.GetUnionRoomList()
	res := map[string]any{
		"unionInfo": unionInfo,
		"roomList":  roomList,
	}
	return common.S(res)
}

func (h *UnionMgrHandler) UpdatePartnerNoticeSwitch(session *remote.Session, msg []byte) any {
	var req request.UpdatePartnerNoticeSwitchReq
	if err := json.Unmarshal(msg, &req); err != nil {
		return common.F(biz.RequestDataError)
	}
	if req.UnionID <= 0 {
		return common.F(biz.RequestDataError)
	}
	union := h.um.GetUnion(req.UnionID, h.redisService, h.userService, h.unionService)
	if session.GetUid() != union.GetOwnerUid() {
		return common.F(biz.RequestDataError)
	}
	union.UpdatePartnerNoticeSwitch(req.IsOpen)
	return common.S(nil)
}

func (h *UnionMgrHandler) DismissRoom(session *remote.Session, msg []byte) any {
	var req request.DismissRoomReq
	if err := json.Unmarshal(msg, &req); err != nil {
		return common.F(biz.RequestDataError)
	}
	if req.UnionID <= 0 {
		return common.F(biz.RequestDataError)
	}
	union := h.um.GetUnion(req.UnionID, h.redisService, h.userService, h.unionService)
	if session.GetUid() != union.GetOwnerUid() {
		return common.F(biz.RequestDataError)
	}
	union.DismissRoom(req.RoomID, session)
	unionInfo := union.GetUnionInfo(session.GetUid())
	roomList := union.GetUnionRoomList()
	res := map[string]any{
		"unionInfo": unionInfo,
		"roomList":  roomList,
	}
	return common.S(res)
}

func (h *UnionMgrHandler) HongBaoSetting(session *remote.Session, msg []byte) any {
	var req request.HongBaoSettingReq
	if err := json.Unmarshal(msg, &req); err != nil {
		return common.F(biz.RequestDataError)
	}
	if req.UnionID <= 0 {
		return common.F(biz.RequestDataError)
	}
	union := h.um.GetUnion(req.UnionID, h.redisService, h.userService, h.unionService)
	if session.GetUid() != union.GetOwnerUid() {
		return common.F(biz.RequestDataError)
	}
	err := union.UpdateHongBaoSetting(req.Status, req.StartTime, req.EndTime, req.Count, req.TotalScore)
	if err != nil {
		return common.F(err)
	}
	return common.S(nil)
}

func (h *UnionMgrHandler) UpdateLotteryStatus(session *remote.Session, msg []byte) any {
	var req request.UpdateLotteryStatusReq
	if err := json.Unmarshal(msg, &req); err != nil {
		return common.F(biz.RequestDataError)
	}
	if req.UnionID <= 0 {
		return common.F(biz.RequestDataError)
	}
	union := h.um.GetUnion(req.UnionID, h.redisService, h.userService, h.unionService)
	if session.GetUid() != union.GetOwnerUid() {
		return common.F(biz.RequestDataError)
	}
	union.UpdateLotteryStatus(req.IsOpen)
	return common.S(nil)
}

func NewUnionMgrHandler(r *repo.Manager, um *logic.UnionManager) *UnionMgrHandler {
	return &UnionMgrHandler{
		um:           um,
		userService:  service.NewUserService(r),
		redisService: service.NewRedisService(r),
		unionService: service.NewUnionService(r),
	}
}
