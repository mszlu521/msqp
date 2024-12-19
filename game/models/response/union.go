package response

import "common"

type HongBaoResp struct {
	common.Result
	UpdateUserData any `json:"updateUserData"`
}
