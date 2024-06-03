package handler

import (
	"common/biz"
	"common/logs"
	"context"
	"core/service"
	"framework/msError"
	"framework/remote"
)

func Dispatch(
	redisService *service.RedisService,
	session *remote.Session,
	roomId string,
	router string) *msError.Error {
	server, err := redisService.Get(context.TODO(), roomId)
	if err != nil {
		return biz.SqlError
	}
	if server == "" {
		return biz.RoomNotExist
	}
	if server == session.GetDst() {
		//不需要分发
		logs.Info("不需要转发请求,dst=%v", session.GetDst())
		return nil
	}
	logs.Info("转发请求,dst=%v", server)
	session.Dispatch(router, server)
	return nil
}
