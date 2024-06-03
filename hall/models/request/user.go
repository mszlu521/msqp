package request

type UpdateUserAddressReq struct {
	Address  string `json:"address,omitempty"`
	Location string `json:"location,omitempty"`
}
type BindPhoneReq struct {
	Phone   string `json:"phone,omitempty"`
	SmsCode string `json:"smsCode,omitempty"`
}
type AuthRealNameReq struct {
	Name   string `json:"name,omitempty"`
	IdCard string `json:"idCard,omitempty"`
}
type SearchReq struct {
	Uid   string `json:"uid,omitempty"`
	Phone string `json:"phone,omitempty"`
}
