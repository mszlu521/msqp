package handler

import (
	"common"
	"common/biz"
	"common/logs"
	"context"
	"core/dao"
	"core/repo"
	"core/service"
	"encoding/json"
	"fmt"
	"framework/remote"
	"hall/models/request"
	"hall/models/response"
	"strconv"
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

type emailData map[string]any

func emailIDString(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case float64:
		return strconv.FormatInt(int64(v), 10)
	case json.Number:
		return v.String()
	default:
		return fmt.Sprint(v)
	}
}

func (h *UserHandler) updateEmail(session *remote.Session, msg []byte, deleteEmail bool) any {
	var req request.EmailReq
	if err := json.Unmarshal(msg, &req); err != nil {
		return common.F(biz.RequestDataError)
	}
	target := emailIDString(req.EmailID)
	for attempt := 0; attempt < 3; attempt++ {
		user, findErr := h.userService.FindUserByUid(context.TODO(), session.GetUid())
		if findErr != nil || user == nil {
			return common.F(biz.SqlError)
		}
		updated, updateErr := updateEmailArray(user.EmailArr, target, deleteEmail)
		if updateErr != nil {
			return common.F(biz.SqlError)
		}
		swapped, swapErr := h.userService.UpdateEmailArrIfCurrent(session.GetUid(), user.EmailArr, updated)
		if swapErr != nil {
			return common.F(biz.SqlError)
		}
		if !swapped {
			continue
		}
		return response.UpdateEmailRes{
			Result:         common.Result{Code: biz.OK},
			UpdateUserData: response.UpdateEmailData{EmailArr: updated},
		}
	}
	return common.F(biz.SqlError)
}

func updateEmailArray(current string, target string, deleteEmail bool) (string, error) {
	var raw []json.RawMessage
	if current != "" {
		if err := json.Unmarshal([]byte(current), &raw); err != nil {
			return "", err
		}
	}
	result := make([]json.RawMessage, 0, len(raw))
	for _, item := range raw {
		var data emailData
		if json.Unmarshal(item, &data) != nil || emailIDString(data["id"]) != target {
			result = append(result, item)
			continue
		}
		if !deleteEmail {
			data["isRead"] = true
			updated, err := json.Marshal(data)
			if err != nil {
				return "", err
			}
			result = append(result, updated)
		}
	}
	updated, err := json.Marshal(result)
	return string(updated), err
}

func (h *UserHandler) ReadEmail(session *remote.Session, msg []byte) any {
	return h.updateEmail(session, msg, false)
}

func (h *UserHandler) DeleteEmail(session *remote.Session, msg []byte) any {
	return h.updateEmail(session, msg, true)
}

func (h *UserHandler) SendCustomerServiceMsg(session *remote.Session, msg []byte) any {
	var req request.CustomerServiceMsgReq
	if err := json.Unmarshal(msg, &req); err != nil || len([]rune(req.Content)) == 0 {
		return common.F(biz.RequestDataError)
	}
	logs.Info("customer service message uid=%s content=%s", session.GetUid(), req.Content)
	return common.S(nil)
}

func NewUserHandler(r *repo.Manager) *UserHandler {
	return &UserHandler{
		userService: service.NewUserService(r),
		redisDao:    dao.NewRedisDao(r),
	}
}
