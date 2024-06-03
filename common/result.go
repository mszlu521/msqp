package common

import (
	"common/biz"
	"framework/msError"
	"github.com/gin-gonic/gin"
	"net/http"
)

type Result struct {
	Code int `json:"code"`
	Msg  any `json:"msg"`
}

func F(err *msError.Error) Result {
	return Result{
		Code: err.Code,
	}
}
func S(data any) Result {
	return Result{
		Code: biz.OK,
		Msg:  data,
	}
}

// Fail err 最后自己封装一个
func Fail(ctx *gin.Context, err *msError.Error) {
	ctx.JSON(http.StatusOK, Result{
		Code: err.Code,
		Msg:  err.Err.Error(),
	})
}
func Success(ctx *gin.Context, data any) {
	ctx.JSON(http.StatusOK, Result{
		Code: biz.OK,
		Msg:  data,
	})
}
