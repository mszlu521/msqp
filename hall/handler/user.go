package handler

import (
	"common"
	"common/biz"
	"common/logs"
	"core/dao"
	"core/repo"
	"core/service"
	"encoding/json"
	"framework/remote"
	"hall/models/request"
	"hall/models/response"
)

type UserHandler struct {
	userService *service.UserService
	redisDao    *dao.RedisDao
}

func (h *UserHandler) UpdateUserAddress(session *remote.Session, msg []byte) any {
	logs.Info("UpdateUserAddress stream:%v", string(msg))
	var req request.UpdateUserAddressReq
	if err := json.Unmarshal(msg, &req); err != nil {
		return common.F(biz.RequestDataError)
	}
	err := h.userService.UpdateUserAddressByUid(session.GetUid(), req)
	if err != nil {
		return common.F(biz.SqlError)
	}
	res := response.UpdateUserAddressRes{}
	res.Code = biz.OK
	res.UpdateUserData = req
	return res
}

func (h *UserHandler) BindPhone(session *remote.Session, msg []byte) any {
	uid := session.GetUid()
	var req request.BindPhoneReq
	if err := json.Unmarshal(msg, &req); err != nil {
		return common.F(biz.RequestDataError)
	}
	if !h.redisDao.CheckSmsCode(req.Phone, req.SmsCode) {
		//验证码错误
		return common.F(biz.SmsCodeError)
	}
	if err := h.userService.BindPhone(uid, req.Phone); err != nil {
		return common.F(err)
	}
	res := &response.UpdateUserRes{}
	res.Code = biz.OK
	res.UpdateUserData = response.UpdateUserData{MobilePhone: req.Phone}
	return res
}

func (h *UserHandler) AuthRealName(session *remote.Session, msg []byte) any {
	uid := session.GetUid()
	var req request.AuthRealNameReq
	if err := json.Unmarshal(msg, &req); err != nil {
		return common.F(biz.RequestDataError)
	}
	realNameInfo, _ := json.Marshal(req)
	err := h.userService.UpdateUserRealName(uid, string(realNameInfo))
	if err != nil {
		return common.F(err)
	}
	res := &response.UpdateUserRes{}
	res.Code = biz.OK
	res.UpdateUserData = response.UpdateUserData{RealName: string(realNameInfo)}
	return res
}

func (h *UserHandler) SearchByPhone(session *remote.Session, msg []byte) any {
	var req request.SearchReq
	if err := json.Unmarshal(msg, &req); err != nil {
		return common.F(biz.RequestDataError)
	}
	user, err := h.userService.GetUserData(req.Phone, "")
	if err != nil {
		return common.F(err)
	}
	return common.S(map[string]any{
		"userData": user,
	})
}

func (h *UserHandler) SearchUserData(session *remote.Session, msg []byte) any {
	var req request.SearchReq
	if err := json.Unmarshal(msg, &req); err != nil {
		return common.F(biz.RequestDataError)
	}
	user, err := h.userService.GetUserData("", req.Uid)
	if err != nil {
		return common.F(err)
	}
	return common.S(map[string]any{
		"userData": user,
	})
}

func NewUserHandler(r *repo.Manager) *UserHandler {
	return &UserHandler{
		userService: service.NewUserService(r),
		redisDao:    dao.NewRedisDao(r),
	}
}
