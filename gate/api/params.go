package api

type RegisterParams struct {
	Account       string `form:"account,omitempty" json:"account,omitempty"`
	Password      string `form:"password,omitempty" json:"password,omitempty"`
	LoginPlatform int32  `form:"loginPlatform,omitempty" json:"loginPlatform,omitempty"`
	SmsCode       string `form:"smsCode,omitempty" json:"smsCode,omitempty"`
}

type SMSCodeParams struct {
	PhoneNumber string `form:"phoneNumber,omitempty" json:"phoneNumber,omitempty"`
}

type ReconnectionParams struct {
	Token string `form:"token,omitempty" json:"token,omitempty"`
}
