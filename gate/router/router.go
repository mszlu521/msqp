package router

import (
	"common/config"
	"common/rpc"
	"gate/api"
	"gate/auth"
	"github.com/gin-gonic/gin"
)

// RegisterRouter 注册路由
func RegisterRouter() *gin.Engine {
	if config.Conf.Log.Level == "DEBUG" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}
	//初始化grpc的client gate是做为grpc的客户端 去调用user grpc服务
	r := gin.Default()
	r.Use(auth.Cors())
	rpc.Init()
	userHandler := api.NewUserHandler()
	r.POST("/register", userHandler.Register)
	r.POST("/getSMSCode", userHandler.GetSMSCode)
	r.POST("/login", userHandler.Login)
	r.POST("/reconnection", userHandler.Reconnection)
	return r
}
