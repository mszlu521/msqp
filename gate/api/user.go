package api

import (
	"common"
	"common/biz"
	"common/config"
	"common/jwts"
	"common/logs"
	"common/rpc"
	"context"
	"framework/msError"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jinzhu/copier"
	"time"
	"user/pb"
)

type UserHandler struct {
}

func NewUserHandler() *UserHandler {
	return &UserHandler{}
}

func (u *UserHandler) Register(ctx *gin.Context) {
	//接收参数
	var req RegisterParams
	err2 := ctx.ShouldBind(&req)
	if err2 != nil {
		logs.Error("Register bind err:%v", err2)
		common.Fail(ctx, biz.RequestDataError)
		return
	}
	var userReq pb.RegisterParams
	copier.Copy(&userReq, req)
	response, err := rpc.UserClient.Register(context.TODO(), &userReq)
	if err != nil {
		common.Fail(ctx, msError.ToError(err))
		return
	}
	uid := response.Uid
	if len(uid) == 0 {
		common.Fail(ctx, biz.SqlError)
		return
	}
	logs.Info("uid:%s", uid)
	//gen token by uid jwt  A.B.C A部分头（定义加密算法） B部分 存储数据  C部分签名
	claims := jwts.CustomClaims{
		Uid: uid,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(7 * 24 * time.Hour)),
		},
	}
	token, err := jwts.GenToken(&claims, config.Conf.Jwt.Secret)
	if err != nil {
		logs.Error("Register jwt gen token err:%v", err)
		common.Fail(ctx, biz.Fail)
		return
	}
	result := map[string]any{
		"token": token,
		"serverInfo": map[string]any{
			"host": config.Conf.Services["connector"].ClientHost,
			"port": config.Conf.Services["connector"].ClientPort,
		},
	}
	common.Success(ctx, result)
}

func (u *UserHandler) GetSMSCode(c *gin.Context) {
	phone := c.PostForm("phoneNumber")
	if _, err := rpc.UserClient.GetSMSCode(context.TODO(), &pb.GetSMSCodeParams{PhoneNumber: phone}); err != nil {
		common.Fail(c, msError.ToError(err))
		return
	}
	common.Success(c, gin.H{})
}

func (u *UserHandler) Login(ctx *gin.Context) {
	//接收参数
	var req RegisterParams
	err2 := ctx.ShouldBind(&req)
	if err2 != nil {
		logs.Error("Register bind err:%v", err2)
		common.Fail(ctx, biz.RequestDataError)
		return
	}
	var userReq pb.LoginParams
	copier.Copy(&userReq, req)
	response, err := rpc.UserClient.Login(context.TODO(), &userReq)
	if err != nil {
		common.Fail(ctx, msError.ToError(err))
		return
	}
	uid := response.Uid
	if len(uid) == 0 {
		common.Fail(ctx, biz.SqlError)
		return
	}
	logs.Info("uid:%s", uid)
	//gen token by uid jwt  A.B.C A部分头（定义加密算法） B部分 存储数据  C部分签名
	claims := jwts.CustomClaims{
		Uid: uid,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(7 * 24 * time.Hour)),
		},
	}
	token, err := jwts.GenToken(&claims, config.Conf.Jwt.Secret)
	if err != nil {
		logs.Error("Register jwt gen token err:%v", err)
		common.Fail(ctx, biz.Fail)
		return
	}
	result := map[string]any{
		"token": token,
		"serverInfo": map[string]any{
			"host": config.Conf.Services["connector"].ClientHost,
			"port": config.Conf.Services["connector"].ClientPort,
		},
	}
	common.Success(ctx, result)
}

func (u *UserHandler) Reconnection(ctx *gin.Context) {
	token := ctx.PostForm("token")
	if token == "" {
		common.Fail(ctx, biz.RequestDataError)
		return
	}
	uid, err := jwts.ParseToken(token, config.Conf.Jwt.Secret)
	if err != nil {
		//token过期或者不合法
		common.Fail(ctx, biz.TokenInfoError)
		return
	}
	claims := jwts.CustomClaims{
		Uid: uid,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(7 * 24 * time.Hour)),
		},
	}
	newToken, err := jwts.GenToken(&claims, config.Conf.Jwt.Secret)
	if err != nil {
		logs.Error("Register jwt gen token err:%v", err)
		common.Fail(ctx, biz.Fail)
		return
	}
	result := map[string]any{
		"token": newToken,
		"serverInfo": map[string]any{
			"host": config.Conf.Services["connector"].ClientHost,
			"port": config.Conf.Services["connector"].ClientPort,
		},
	}
	common.Success(ctx, result)
}
