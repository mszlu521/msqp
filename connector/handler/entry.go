package handler

import (
	"common"
	"common/biz"
	"common/config"
	"common/jwts"
	"common/logs"
	"connector/models/request"
	"context"
	"core/repo"
	"core/service"
	"encoding/json"
	"framework/game"
	"framework/net"
)

type EntryHandler struct {
	userService *service.UserService
}

func (h *EntryHandler) Entry(session *net.Session, body []byte) (any, error) {
	logs.Info("==============Entry Start=====================")
	logs.Info("entry request params:%v", string(body))
	logs.Info("==============Entry End=====================")
	var req request.EntryReq
	err := json.Unmarshal(body, &req)
	if err != nil {
		return common.F(biz.RequestDataError), nil
	}
	//校验token
	uid, err := jwts.ParseToken(req.Token, config.Conf.Jwt.Secret)
	if err != nil {
		logs.Error("parse token err:%v", err)
		return common.F(biz.TokenInfoError), nil
	}
	//根据uid 去mongo中查询用户 如果用户不存在 生成一个用户
	user, err := h.userService.FindAndSaveUserByUid(context.TODO(), uid, req.UserInfo)
	if err != nil {
		return common.F(biz.SqlError), nil
	}
	session.Uid = uid
	return common.S(map[string]any{
		"userInfo": user,
		"config":   game.Conf.GetFrontGameConfig(),
	}), nil
}

func NewEntryHandler(r *repo.Manager) *EntryHandler {
	return &EntryHandler{
		userService: service.NewUserService(r),
	}
}
