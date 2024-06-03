package handler

import (
	"common/biz"
	"common/logs"
	"context"
	"core/service"
	"framework/msError"
	"framework/remote"
)

func Proxy(redisService *service.RedisService, session *remote.Session, roomId string) (bool, *msError.Error) {
	server, err := redisService.Get(context.TODO(), roomId)
	if err != nil {
		return false, biz.SqlError
	}
	if server == "" {
		return false, biz.NotInRoom
	}
	if server == session.GetDst() {
		return true, nil
	}
	logs.Info("转发请求到真正的game服务器,dst=%v", server)
	session.SendProxy(server)
	return false, nil
}
