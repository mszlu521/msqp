package response

import (
	"common"
	"hall/models/request"
)

type UpdateUserAddressRes struct {
	common.Result
	UpdateUserData request.UpdateUserAddressReq `json:"updateUserData"`
}
type UpdateUserData struct {
	MobilePhone string `json:"mobilePhone,omitempty"`
	RealName    any    `json:"realName,omitempty"`
}
type UpdateUserRes struct {
	common.Result
	UpdateUserData UpdateUserData `json:"updateUserData"`
}
