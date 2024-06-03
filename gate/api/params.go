package api

type RegisterParams struct {
	Account       string `form:"account,omitempty"`
	Password      string `form:"password,omitempty"`
	LoginPlatform int32  `form:"loginPlatform,omitempty"`
	SmsCode       string `form:"smsCode,omitempty"`
}
